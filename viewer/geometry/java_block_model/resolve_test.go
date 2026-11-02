package java_block_model

import "testing"

func TestReferenceCyclesReturnErrors(t *testing.T) {
	resolver := NewResolver(map[ResourceID]Model{"minecraft:block/a": {Parent: "minecraft:block/b"}, "minecraft:block/b": {Parent: "minecraft:block/a"}})
	if _, err := resolver.Resolve("minecraft:block/a"); err == nil {
		t.Fatal("parent cycle accepted")
	}
	model := Resolved{Textures: map[string]TextureBinding{"a": {Sprite: "#b"}, "b": {Sprite: "#a"}}}
	if _, _, err := model.Texture("#a"); err == nil {
		t.Fatal("texture cycle accepted")
	}
}

func TestBuiltinAndTextureMaterialHints(t *testing.T) {
	translucent := true
	elements := []Element{{From: Vec3{}, To: Vec3{16, 16, 16}}}
	resolver := NewResolver(Models{
		"minecraft:block/special": {Parent: "minecraft:builtin/entity"},
		"minecraft:block/preview": {Parent: "minecraft:block/special", Elements: &elements,
			Textures: map[string]TextureBinding{
				"outer": {Sprite: "#inner", ForceTranslucent: &translucent},
				"inner": {Sprite: "minecraft:block/stone"},
			}},
	})
	special, err := resolver.Resolve("minecraft:block/special")
	if err != nil || special.Builtin != "minecraft:builtin/entity" {
		t.Fatalf("lost builtin: %+v %v", special, err)
	}
	if _, err := Bake(special, Apply{}, nil); err == nil {
		t.Fatal("builtin silently baked as empty geometry")
	}
	preview, err := resolver.Resolve("minecraft:block/preview")
	if err != nil || preview.Builtin != "" || len(preview.Elements) != 1 {
		t.Fatalf("explicit preview did not replace builtin: %+v %v", preview, err)
	}
	texture, blend, err := preview.Texture("#outer")
	if err != nil || texture != "minecraft:block/stone" || !blend {
		t.Fatalf("lost texture hint: %s %v %v", texture, blend, err)
	}
}
