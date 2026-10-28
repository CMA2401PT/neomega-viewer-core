// SPDX-License-Identifier: MIT
// Package parse reads the single-version files produced in dist/final.
// It depends only on asset types and JSON I/O, not on the generation stages.
package parse

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"neomega-assets/asset"
	"neomega-assets/utils/jsonfile"
)

// Assets contains the typed tables and the decoded atlas from one final directory.
// Models retain their parent references; loading does not choose state variants.
type Assets struct {
	BlockStates asset.BlockStates
	Models      asset.Models
	Atlas       asset.Atlas
	Renderers   asset.Renderers
	BuildInfo   asset.BuildInfo
	AtlasImage  image.Image
}

func ReadBlockStates(path string) (asset.BlockStates, error) {
	return jsonfile.Read[asset.BlockStates](path)
}

func ReadModels(path string) (asset.Models, error) {
	return jsonfile.Read[asset.Models](path)
}

func ReadAtlas(path string) (asset.Atlas, error) {
	return jsonfile.Read[asset.Atlas](path)
}

func ReadRenderers(path string) (asset.Renderers, error) {
	return jsonfile.Read[asset.Renderers](path)
}

func ReadBuildInfo(path string) (asset.BuildInfo, error) {
	return jsonfile.Read[asset.BuildInfo](path)
}

func ReadAtlasImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}

// Load reads all data files from one final output directory. For consumers that
// need only one table, the Read functions avoid loading the rest of the bundle.
func Load(dir string) (*Assets, error) {
	var result Assets
	var err error
	result.BlockStates, err = ReadBlockStates(filepath.Join(dir, "blockstates.json"))
	if err != nil {
		return nil, err
	}
	result.Models, err = ReadModels(filepath.Join(dir, "models.json"))
	if err != nil {
		return nil, err
	}
	result.Atlas, err = ReadAtlas(filepath.Join(dir, "blocksAtlases.json"))
	if err != nil {
		return nil, err
	}
	result.Renderers, err = ReadRenderers(filepath.Join(dir, "blockRenderers.json"))
	if err != nil {
		return nil, err
	}
	result.BuildInfo, err = ReadBuildInfo(filepath.Join(dir, "build-info.json"))
	if err != nil {
		return nil, err
	}
	result.AtlasImage, err = ReadAtlasImage(filepath.Join(dir, "blocksAtlasLatest.png"))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// BlockState and Model accept canonical IDs and translate to the export keys.
func (assets *Assets) BlockState(id asset.ID) (asset.BlockState, bool) {
	state, ok := assets.BlockStates[id.ExportName()]
	return state, ok
}

func (assets *Assets) Model(id asset.ID) (asset.Model, bool) {
	model, ok := assets.Models[id.ExportName()]
	return model, ok
}

func (assets *Assets) Texture(id asset.ID) (asset.AtlasTexture, bool) {
	texture, ok := assets.Atlas.Textures[id.TextureName()]
	return texture, ok
}

// TextureRect converts normalized UV coordinates into pixels in AtlasImage.
func (assets *Assets) TextureRect(id asset.ID) (image.Rectangle, bool) {
	texture, ok := assets.Texture(id)
	if !ok {
		return image.Rectangle{}, false
	}
	x := int(math.Round(texture.U * float64(assets.Atlas.Width)))
	y := int(math.Round(texture.V * float64(assets.Atlas.Height)))
	width := int(math.Round(texture.SU * float64(assets.Atlas.Width)))
	height := int(math.Round(texture.SV * float64(assets.Atlas.Height)))
	return image.Rect(x, y, x+width, y+height), true
}
