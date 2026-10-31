// SPDX-License-Identifier: MIT
package s03_models

import (
	"testing"

	jmodel "neomega-blocks/geometry/java_block_model"
)

func TestVanillaUnprefixedTextureSlot(t *testing.T) {
	// 26.3's heavy_core faces refer to the local slot as "all", without '#'.
	elements := []jmodel.Element{{Faces: map[jmodel.Direction]jmodel.Face{"up": {Texture: "all"}}}}
	model := jmodel.Model{Elements: &elements, Textures: map[string]jmodel.TextureBinding{"all": {Sprite: "minecraft:block/heavy_core"}}}
	models := map[jmodel.ResourceID]jmodel.Model{"minecraft:block/heavy_core": model}
	normalized, err := normalizeModel(model, models)
	if err != nil {
		t.Fatal(err)
	}
	models["minecraft:block/heavy_core"] = normalized
	resolved, err := jmodel.NewResolver(models).Resolve("minecraft:block/heavy_core")
	if err != nil {
		t.Fatal(err)
	}
	texture, _, err := resolved.Texture(resolved.Elements[0].Faces["up"].Texture)
	if err != nil || texture != "minecraft:block/heavy_core" {
		t.Fatalf("local slot resolved as %q: %v", texture, err)
	}
}
