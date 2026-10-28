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

	"neomega-assets/asset"
	"neomega-assets/utils/jsonfile"
	"neomega-assets/utils/legal"
	"neomega-assets/utils/resourcefiles"
)

type Options struct {
	InputDir  string
	ModelDir  string
	OutputDir string
	TileSize  int
}
type textureImage struct {
	id    asset.ID
	image *image.NRGBA
}

func Run(ctx context.Context, opt Options) (*asset.Atlas, error) {
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
	selected := map[asset.ID]bool{}
	for id := range snapshot.Textures {
		if strings.HasPrefix(id.Path(), "block/") || strings.HasPrefix(id.Path(), "blocks/") {
			selected[id] = true
		}
	}
	for _, id := range report.RequiredTextures {
		selected[id] = true
	}
	ids := make([]asset.ID, 0, len(selected))
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
		width, height := opt.TileSize, opt.TileSize
		if !strings.HasPrefix(id.Path(), "block/") && !strings.HasPrefix(id.Path(), "blocks/") {
			width = frame.Dx()
			height = frame.Dy()
		}
		name := id.TextureName()
		if _, exists := images[name]; exists {
			return nil, fmt.Errorf("atlas key collision at %s", name)
		}
		images[name] = textureImage{id: id, image: renderTexture(img, frame, width, height)}
		paddedWidth := (width + opt.TileSize - 1) / opt.TileSize * opt.TileSize
		paddedHeight := (height + opt.TileSize - 1) / opt.TileSize * opt.TileSize
		rectangles = append(rectangles, rectangle{name: name, width: paddedWidth, height: paddedHeight})
	}
	packed, size, err := pack(rectangles, opt.TileSize)
	if err != nil {
		return nil, err
	}
	atlas := asset.Atlas{Textures: map[string]asset.AtlasTexture{}, Width: size, Height: size, TileSize: opt.TileSize, SUSV: float64(opt.TileSize) / float64(size)}
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	for _, rect := range packed {
		texture := images[rect.name].image
		target := image.Rectangle{Min: rect.position, Max: rect.position.Add(texture.Bounds().Size())}
		draw.Draw(canvas, target, texture, texture.Bounds().Min, draw.Src)
		atlas.Textures[rect.name] = asset.AtlasTexture{U: float64(target.Min.X) / float64(size), V: float64(target.Min.Y) / float64(size), SU: float64(target.Dx()) / float64(size), SV: float64(target.Dy()) / float64(size), TileIndex: target.Min.Y/opt.TileSize*(size/opt.TileSize) + target.Min.X/opt.TileSize}
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
