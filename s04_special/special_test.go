// SPDX-License-Identifier: MIT
package s04_special

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"neomega-assets/asset"
	"neomega-assets/internal/jsonfile"
	mcassets "neomega-assets/third_party/mc-assets"
)

func TestKnownNonModelRenderersAreExplicit(t *testing.T) {
	cases := map[asset.ID]asset.RendererKind{
		"minecraft:air":           asset.RendererInvisible,
		"minecraft:water":         asset.RendererFluid,
		"minecraft:bubble_column": asset.RendererFluid,
		"minecraft:moving_piston": asset.RendererDynamic,
	}
	for id, want := range cases {
		got, ok := classifyEmpty(id)
		if !ok || got.Kind != want || got.StaticPreview {
			t.Fatalf("%s: %+v", id, got)
		}
	}
	if _, ok := classifyEmpty("demo:air"); ok {
		t.Fatal("a mod's empty model was silently classified by vanilla name")
	}
}

func TestUnrecognizedEmptyModelDoesNotPassAsComplete(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	models := filepath.Join(root, "models")
	snapshot := asset.NewSnapshot("fixture")
	if err := jsonfile.Write(filepath.Join(input, "snapshot.json"), snapshot); err != nil {
		t.Fatal(err)
	}
	index := asset.ModelIndex{Version: "fixture", GeometrylessBlockStates: []asset.ID{"demo:ghost"}}
	jsonfile.Write(filepath.Join(models, "models.json"), index)
	tables := asset.Tables{BlockStates: map[string]map[string]asset.BlockState{"latest": {"demo:ghost": {Variants: map[string]asset.Choice{"": {Options: []asset.Apply{{Model: "demo:block/ghost"}}}}}}}}
	jsonfile.Write(filepath.Join(models, "blockStatesModels.json"), tables)
	_, err := Run(context.Background(), Options{InputDir: input, ModelDir: models, OutputDir: filepath.Join(root, "completed")})
	if err == nil || !strings.Contains(err.Error(), "demo:ghost") {
		t.Fatalf("unrecognized geometry-less block did not fail clearly: %v", err)
	}
}

func TestColoredShulkerPreviewsUseTheirOwnSkins(t *testing.T) {
	index, err := mcassets.Load()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := asset.NewSnapshot("fixture")
	output := t.TempDir()
	importer := templateImporter{
		index:          index,
		snapshot:       snapshot,
		output:         output,
		importing:      map[asset.ID]bool{},
		neededTextures: map[asset.ID]bool{},
	}
	for _, color := range []string{"red", "blue"} {
		skin := asset.ID("minecraft:entity/shulker/shulker_" + color)
		snapshot.Textures[skin] = asset.File{}
		state, err := importer.state(asset.ID("minecraft:" + color + "_shulker_box"))
		if err != nil {
			t.Fatal(err)
		}
		for _, application := range state.Applications() {
			file := snapshot.Models[application.Model]
			model, err := jsonfile.Read[asset.Model](filepath.Join(output, file.Path))
			if err != nil {
				t.Fatal(err)
			}
			if model.Textures["0"].Sprite != asset.TextureRef(skin) {
				t.Fatalf("%s preview uses another color's skin", color)
			}
			if len(model.Extra["credit"]) == 0 {
				t.Fatal("community model attribution was dropped")
			}
			for _, element := range *model.Elements {
				for _, face := range element.Faces {
					if face.TintIndex != nil {
						t.Fatal("colored skin would be tinted a second time")
					}
				}
			}
		}
	}
}
