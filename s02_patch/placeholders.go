// SPDX-License-Identifier: MIT
package s02_patch

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"neomega-assets/asset"
	"neomega-assets/utils/jsonfile"
)

// These two small diagnostic textures are generated here, not copied from a JAR.
func addPlaceholders(snapshot *asset.Snapshot, output string) error {
	for _, name := range []string{"unknown", "errored"} {
		block := asset.ID("minecraft:" + name)
		model := asset.ID("minecraft:block/" + name)
		texture := model
		statePath := "assets/minecraft/blockstates/" + name + ".json"
		modelPath := "assets/minecraft/models/block/" + name + ".json"
		texturePath := "assets/minecraft/textures/block/" + name + ".png"
		state := asset.BlockState{Variants: map[string]asset.Choice{"": {Options: []asset.Apply{{Model: model}}}}}
		data := asset.Model{Parent: "minecraft:block/cube_all", Textures: map[string]asset.TextureBinding{"all": {Sprite: asset.TextureRef(texture)}}}
		if err := jsonfile.Write(filepath.Join(output, statePath), state); err != nil {
			return err
		}
		if err := jsonfile.Write(filepath.Join(output, modelPath), data); err != nil {
			return err
		}
		img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
		bright := color.NRGBA{R: 255, B: 255, A: 255}
		if name == "errored" {
			bright = color.NRGBA{R: 255, A: 255}
		}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				c := color.NRGBA{A: 255}
				if (x/8+y/8)%2 == 0 {
					c = bright
				}
				img.SetNRGBA(x, y, c)
			}
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(output, texturePath)), 0755); err != nil {
			return err
		}
		file, err := os.Create(filepath.Join(output, texturePath))
		if err != nil {
			return err
		}
		err = png.Encode(file, img)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		snapshot.BlockStates[block] = asset.File{Path: statePath, Origin: "neomega-assets generated placeholder (MIT)"}
		snapshot.Models[model] = asset.File{Path: modelPath, Origin: "neomega-assets generated placeholder (MIT)"}
		snapshot.Textures[texture] = asset.File{Path: texturePath, Origin: "neomega-assets generated placeholder (MIT)"}
	}
	return nil
}
