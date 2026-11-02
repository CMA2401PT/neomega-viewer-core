// SPDX-License-Identifier: MIT
// Package s05_atlas creates the PNG and the coordinate table as one pair.
// mc-assets UV names are MIT-derived compatibility conventions;
// packing and PNG processing are implemented here with Go's standard library.
package s05_atlas

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"neomega-viewer-core/viewer-assets/asset"
	"neomega-viewer-core/viewer-assets/utils/jsonfile"
	"neomega-viewer-core/viewer-assets/utils/legal"
	"neomega-viewer-core/viewer-assets/utils/resourcefiles"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
)

type Options struct {
	InputDir  string
	ModelDir  string
	OutputDir string
	TileSize  int
}
type textureImage struct {
	id        jmodel.ResourceID
	image     *image.NRGBA
	frames    int
	frameTime int
}

func Run(ctx context.Context, opt Options) (*jmodel.Atlas, error) {
	if opt.TileSize == 0 {
		opt.TileSize = 16
	}
	if opt.TileSize < 1 || opt.TileSize > 1024 || opt.TileSize&(opt.TileSize-1) != 0 {
		return nil, fmt.Errorf("tile size must be a power of two between 1 and 1024")
	}
	snapshot, err := jsonfile.Read[asset.Snapshot](filepath.Join(opt.InputDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	report, err := jsonfile.Read[asset.ModelIndex](filepath.Join(opt.ModelDir, "model-index.json"))
	if err != nil {
		return nil, err
	}
	renderers, err := jsonfile.Read[asset.Renderers](filepath.Join(opt.ModelDir, "blockRenderers.json"))
	if err != nil {
		return nil, fmt.Errorf("s04 renderer table required: %w", err)
	}
	if renderers.Version != snapshot.Version {
		return nil, fmt.Errorf("renderer and snapshot versions differ")
	}
	if snapshot.Version != report.Version {
		return nil, fmt.Errorf("model table version %s differs from texture version %s", report.Version, snapshot.Version)
	}
	selected := map[jmodel.ResourceID]bool{}
	for id := range snapshot.Textures {
		if strings.HasPrefix(id.Path(), "block/") || strings.HasPrefix(id.Path(), "blocks/") {
			selected[id] = true
		}
	}
	for _, id := range report.RequiredTextures {
		selected[id] = true
	}
	ids := make([]jmodel.ResourceID, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	images := map[string]textureImage{}
	var rectangles []rectangle
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, ok := snapshot.Textures[id]
		if !ok {
			return nil, fmt.Errorf("model references missing texture %s", id)
		}
		img, err := loadPNG(filepath.Join(opt.InputDir, filepath.FromSlash(file.Path)))
		if err != nil {
			return nil, fmt.Errorf("texture %s: %w", id, err)
		}
		var metadata asset.TextureMetadata
		if file, ok := snapshot.TextureMetadata[id]; ok {
			metadata, err = jsonfile.Read[asset.TextureMetadata](filepath.Join(opt.InputDir, filepath.FromSlash(file.Path)))
			if err != nil {
				return nil, err
			}
		}
		frame, err := FirstFrame(img, metadata)
		if err != nil {
			return nil, fmt.Errorf("texture %s: %w", id, err)
		}
		isBlock := strings.HasPrefix(id.Path(), "block/") || strings.HasPrefix(id.Path(), "blocks/")
		width, height := opt.TileSize, opt.TileSize
		if !isBlock {
			width = frame.Dx()
			height = frame.Dy()
		}
		texture := textureImage{id: id, frames: 1}
		if isBlock && metadata.Animation != nil {
			texture.image, texture.frames, err = renderAnimation(img, metadata.Animation, opt.TileSize)
			if err != nil {
				return nil, fmt.Errorf("texture %s: %w", id, err)
			}
			// 逐帧时长未支持；统一使用默认帧时，省略时为 1 tick。
			texture.frameTime = metadata.Animation.FrameTime
			if texture.frameTime == 0 {
				texture.frameTime = 1
			}
			height = texture.image.Bounds().Dy()
		} else {
			texture.image = renderTexture(img, frame, width, height)
		}
		name := packingName(id)
		if _, exists := images[name]; exists {
			return nil, fmt.Errorf("atlas key collision at %s", name)
		}
		images[name] = texture
		paddedWidth := (width + opt.TileSize - 1) / opt.TileSize * opt.TileSize
		paddedHeight := (height + opt.TileSize - 1) / opt.TileSize * opt.TileSize
		rectangles = append(rectangles, rectangle{name: name, width: paddedWidth, height: paddedHeight})
	}
	packed, size, err := pack(rectangles, opt.TileSize)
	if err != nil {
		return nil, err
	}
	atlas := jmodel.Atlas{Textures: map[jmodel.ResourceID]jmodel.AtlasTexture{}, Width: size.X, Height: size.Y, TileSize: opt.TileSize}
	canvas := image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))
	for _, rect := range packed {
		texture := images[rect.name]
		target := image.Rectangle{Min: rect.position, Max: rect.position.Add(texture.image.Bounds().Size())}
		draw.Draw(canvas, target, texture.image, texture.image.Bounds().Min, draw.Src)
		// 坐标覆盖整条动画，下游按帧数纵向切分。
		atlas.Textures[texture.id] = jmodel.AtlasTexture{
			U:         float64(target.Min.X) / float64(size.X),
			V:         float64(target.Min.Y) / float64(size.Y),
			SU:        float64(target.Dx()) / float64(size.X),
			SV:        float64(target.Dy()) / float64(size.Y),
			TileIndex: target.Min.Y/opt.TileSize*(size.X/opt.TileSize) + target.Min.X/opt.TileSize,
			Frames:    texture.frames,
			FrameTime: texture.frameTime,
		}
	}
	if err := os.MkdirAll(opt.OutputDir, 0755); err != nil {
		return nil, err
	}
	output, err := os.Create(filepath.Join(opt.OutputDir, "blocksAtlasLatest.png"))
	if err != nil {
		return nil, err
	}
	encodeErr := png.Encode(output, canvas)
	closeErr := output.Close()
	if encodeErr != nil {
		return nil, encodeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "blocksAtlases.json"), atlas); err != nil {
		return nil, err
	}
	for _, name := range []string{"blockstates.json", "models.json", "blockRenderers.json"} {
		if err := resourcefiles.CopyFile(filepath.Join(opt.ModelDir, name), filepath.Join(opt.OutputDir, name)); err != nil {
			return nil, err
		}
	}
	build := asset.BuildInfo{GeometrylessBlockStates: report.GeometrylessBlockStates, RendererCounts: renderers.Counts, Version: snapshot.Version, Sources: snapshot.Sources, BlockStateCount: report.BlockStateCount, ModelCount: report.ModelCount, TextureCount: len(images), BuiltinModels: report.BuiltinModels}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "build-info.json"), build); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(opt.OutputDir, "THIRD_PARTY_NOTICES.txt"), []byte(legal.Text()), 0644); err != nil {
		return nil, err
	}
	return &atlas, nil
}

// packingName orders tiles by their atlas-local path. Resource lookup and the
// exported coordinate table use the full ID, independent of the packing order.
func packingName(id jmodel.ResourceID) string {
	name := strings.TrimPrefix(strings.TrimPrefix(id.Path(), "block/"), "blocks/")
	if id.Namespace() != "minecraft" {
		name = id.Namespace() + ":" + name
	}
	return name
}
