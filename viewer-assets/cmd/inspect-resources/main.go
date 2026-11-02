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

	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
)

type blockResult struct {
	ID       jmodel.ResourceID    `json:"id"`
	Rules    jmodel.ModelRules    `json:"rules"`
	Renderer jmodel.BlockRenderer `json:"renderer"`
}

type modelResult struct {
	ID         jmodel.ResourceID `json:"id"`
	Definition jmodel.Model      `json:"definition"`
}

type textureResult struct {
	ID     jmodel.ResourceID   `json:"id"`
	UV     jmodel.AtlasTexture `json:"uv"`
	Pixels image.Rectangle     `json:"pixels"`
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
	assets, err := jmodel.Load(*dir)
	if err != nil {
		return err
	}
	result := summary{
		Version:     assets.Info.Version,
		BlockStates: len(assets.Rules),
		Models:      len(assets.Models),
		Textures:    len(assets.Atlas.Textures),
		PNGSize:     [2]int{assets.AtlasImage.Bounds().Dx(), assets.AtlasImage.Bounds().Dy()},
	}
	if *blockName != "" {
		id, err := jmodel.ParseResourceID(*blockName)
		if err != nil {
			return err
		}
		state, ok := assets.Rules[id]
		if !ok {
			return fmt.Errorf("block %s not found", id)
		}
		result.Block = &blockResult{ID: id, Rules: state, Renderer: assets.Renderers.Blocks[id]}
	}
	if *modelName != "" {
		id, err := jmodel.ParseResourceID(*modelName)
		if err != nil {
			return err
		}
		model, ok := assets.Models[id]
		if !ok {
			return fmt.Errorf("model %s not found", id)
		}
		result.Model = &modelResult{ID: id, Definition: model}
	}
	if *textureName != "" {
		id, err := jmodel.ParseResourceID(*textureName)
		if err != nil {
			return err
		}
		texture, ok := assets.Atlas.Textures[id]
		if !ok {
			return fmt.Errorf("texture %s not found", id)
		}
		pixels, _ := assets.Atlas.PixelRect(id)
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
