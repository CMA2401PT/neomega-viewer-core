// SPDX-License-Identifier: MIT
package s03_models

import (
	"neomega-assets/asset"
	"testing"
)

func TestParentInheritanceAndExplicitEmptyElements(t *testing.T) {
	elements := []asset.Element{{Faces: map[asset.Direction]asset.Face{"up": {Texture: "#alias"}}}}
	empty := []asset.Element{}
	resolver := NewResolver(map[asset.ID]asset.Model{
		"minecraft:block/base":  {Elements: &elements, Textures: map[string]asset.TextureBinding{"alias": {Sprite: "#actual"}, "actual": {Sprite: "minecraft:block/old"}}},
		"minecraft:block/child": {Parent: "minecraft:block/base", Textures: map[string]asset.TextureBinding{"actual": {Sprite: "minecraft:block/new"}}},
		"minecraft:block/empty": {Parent: "minecraft:block/child", Elements: &empty},
	})
	child, err := resolver.Resolve("minecraft:block/child")
	if err != nil {
		t.Fatal(err)
	}
	texture, err := child.Texture("#alias")
	if err != nil || texture != "minecraft:block/new" {
		t.Fatalf("child override: %s, %v", texture, err)
	}
	base, err := resolver.Resolve("minecraft:block/base")
	if err != nil {
		t.Fatal(err)
	}
	texture, err = base.Texture("#alias")
	if err != nil || texture != "minecraft:block/old" {
		t.Fatal("child changed its parent")
	}
	resolved, err := resolver.Resolve("minecraft:block/empty")
	if err != nil || len(resolved.Elements) != 0 {
		t.Fatal("explicit empty elements did not clear inherited geometry")
	}
}

func TestReferenceCyclesReturnErrors(t *testing.T) {
	resolver := NewResolver(map[asset.ID]asset.Model{"minecraft:block/a": {Parent: "minecraft:block/b"}, "minecraft:block/b": {Parent: "minecraft:block/a"}})
	if _, err := resolver.Resolve("minecraft:block/a"); err == nil {
		t.Fatal("parent cycle accepted")
	}
	model := ResolvedModel{Textures: map[string]asset.TextureBinding{"a": {Sprite: "#b"}, "b": {Sprite: "#a"}}}
	if _, err := model.Texture("#a"); err == nil {
		t.Fatal("texture cycle accepted")
	}
}

func TestVanillaUnprefixedTextureSlot(t *testing.T) {
	// 26.3's heavy_core faces refer to the local slot as "all", without '#'.
	elements := []asset.Element{{Faces: map[asset.Direction]asset.Face{"up": {Texture: "all"}}}}
	model := asset.Model{Elements: &elements, Textures: map[string]asset.TextureBinding{"all": {Sprite: "minecraft:block/heavy_core"}}}
	models := map[asset.ID]asset.Model{"minecraft:block/heavy_core": model}
	normalized, err := normalizeModel(model, models)
	if err != nil {
		t.Fatal(err)
	}
	models["minecraft:block/heavy_core"] = normalized
	resolved, err := NewResolver(models).Resolve("minecraft:block/heavy_core")
	if err != nil {
		t.Fatal(err)
	}
	texture, err := resolved.Texture(resolved.Elements[0].Faces["up"].Texture)
	if err != nil || texture != "minecraft:block/heavy_core" {
		t.Fatalf("local slot resolved as %q: %v", texture, err)
	}
}
