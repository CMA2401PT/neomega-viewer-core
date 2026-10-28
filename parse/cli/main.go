// SPDX-License-Identifier: MIT
// This example loads only final outputs, then queries their typed contents.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"io"
	"os"

	"neomega-assets/asset"
	"neomega-assets/parse"
)

type blockResult struct {
	ID       asset.ID            `json:"id"`
	State    asset.BlockState    `json:"state"`
	Renderer asset.BlockRenderer `json:"renderer"`
}

type modelResult struct {
	ID         asset.ID    `json:"id"`
	Definition asset.Model `json:"definition"`
}

type textureResult struct {
	ID     asset.ID           `json:"id"`
	UV     asset.AtlasTexture `json:"uv"`
	Pixels image.Rectangle    `json:"pixels"`
}

type summary struct {
	Version     string         `json:"version"`
	BlockStates int            `json:"blockstates"`
	Models      int            `json:"models"`
	Textures    int            `json:"textures"`
	PNGSize     [2]int         `json:"pngSize"`
	Block       *blockResult   `json:"block,omitempty"`
	Model       *modelResult   `json:"model,omitempty"`
	Texture     *textureResult `json:"texture,omitempty"`
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("parse", flag.ContinueOnError)
	dir := flags.String("dir", "dist/final", "final output directory")
	blockName := flags.String("block", "", "optional block ID, e.g. minecraft:chest")
	modelName := flags.String("model", "", "optional model ID, e.g. minecraft:block/stone")
	textureName := flags.String("texture", "", "optional texture ID, e.g. minecraft:block/stone")
	if err := flags.Parse(args); err != nil {
		return err
	}
	assets, err := parse.Load(*dir)
	if err != nil {
		return err
	}
	result := summary{
		Version:     assets.BuildInfo.Version,
		BlockStates: len(assets.BlockStates),
		Models:      len(assets.Models),
		Textures:    len(assets.Atlas.Textures),
		PNGSize:     [2]int{assets.AtlasImage.Bounds().Dx(), assets.AtlasImage.Bounds().Dy()},
	}
	if *blockName != "" {
		id, err := asset.ParseID(*blockName)
		if err != nil {
			return err
		}
		state, ok := assets.BlockState(id)
		if !ok {
			return fmt.Errorf("block %s not found", id)
		}
		result.Block = &blockResult{ID: id, State: state, Renderer: assets.Renderers.Blocks[id]}
	}
	if *modelName != "" {
		id, err := asset.ParseID(*modelName)
		if err != nil {
			return err
		}
		model, ok := assets.Model(id)
		if !ok {
			return fmt.Errorf("model %s not found", id)
		}
		result.Model = &modelResult{ID: id, Definition: model}
	}
	if *textureName != "" {
		id, err := asset.ParseID(*textureName)
		if err != nil {
			return err
		}
		texture, ok := assets.Texture(id)
		if !ok {
			return fmt.Errorf("texture %s not found", id)
		}
		pixels, _ := assets.TextureRect(id)
		result.Texture = &textureResult{ID: id, UV: texture, Pixels: pixels}
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		if err == flag.ErrHelp {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
