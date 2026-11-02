package java_block_model

import (
	"reflect"
	"testing"

	"neomega-blocks/ir"
)

func TestVariantsAndMultipartKeepTheSelectionTree(t *testing.T) {
	state := ModelRules{Multipart: []Part{
		{Apply: Choice{Options: []Apply{{Model: "post"}}}},
		{When: &Condition{Properties: map[string]string{"north": "true"}}, Apply: Choice{Options: []Apply{{Model: "north"}}}},
		{When: &Condition{OR: []Condition{{Properties: map[string]string{"east": "true"}}, {AND: []Condition{{Properties: map[string]string{"south": "true"}}, {Properties: map[string]string{"west": "false"}}}}}}, Apply: Choice{Options: []Apply{{Model: "rail"}}}},
	}}
	selected, err := Select(state, ir.IRStates(`["north"="true","east"="false","south"="true","west"="false"]`))
	if err != nil || len(selected) != 3 {
		t.Fatalf("multipart selection: %v %v", selected, err)
	}
	selected, err = Select(state, ir.IRStates(`["north"="false","east"="false","south"="true","west"="true"]`))
	if err != nil || len(selected) != 1 || selected[0].Options[0].Model != "post" {
		t.Fatalf("unmatched parts were included: %v %v", selected, err)
	}
	variants := ModelRules{Variants: map[string]Choice{
		"type=bottom": {Options: []Apply{{Model: "slab"}}},
		"type=top":    {Options: []Apply{{Model: "slab", X: 180, UVLock: true}}},
	}}
	selected, err = Select(variants, ir.IRStates(`["type"="top"]`))
	if err != nil || len(selected) != 1 || selected[0].Options[0].X != 180 || !selected[0].Options[0].UVLock {
		t.Fatalf("variant transform lost: %v %v", selected, err)
	}
	if _, err := Select(variants, ir.IRStates(`["type"="invalid"]`)); err == nil {
		t.Fatal("invalid state silently selected a model")
	}
}

func TestSelectPreservesMatchingPartsAndWeights(t *testing.T) {
	heavy := 100
	choice := Choice{Options: []Apply{
		{Model: "first", Y: 90, UVLock: true},
		{Model: "other", Weight: &heavy},
	}}
	state := ModelRules{
		Variants: map[string]Choice{"": choice, "north=true": choice},
		Multipart: []Part{
			{Apply: choice},
			{When: &Condition{Properties: map[string]string{"north": "true"}}, Apply: choice},
			{When: &Condition{Properties: map[string]string{"north": "false"}}, Apply: choice},
		},
	}
	selected, err := Select(state, ir.IRStates(`["north"="true"]`))
	if err != nil || len(selected) != 4 {
		t.Fatalf("matching positions were lost: %v %v", selected, err)
	}
	for _, application := range selected {
		if !reflect.DeepEqual(application, choice) {
			t.Fatalf("candidate weights or transforms changed: %+v", application)
		}
	}
}

func TestInheritanceAndTextureSlotsDoNotMutateParents(t *testing.T) {
	elements := []Element{{Faces: map[Direction]Face{"up": {Texture: "#alias"}}}}
	empty := []Element{}
	resolver := NewResolver(Models{
		"minecraft:block/base":  {Elements: &elements, Textures: map[string]TextureBinding{"alias": {Sprite: "#skin"}, "skin": {Sprite: "block/old"}}},
		"minecraft:block/child": {Parent: "minecraft:block/base", Textures: map[string]TextureBinding{"skin": {Sprite: "block/new"}}},
		"minecraft:block/empty": {Parent: "minecraft:block/child", Elements: &empty},
	})
	child, err := resolver.Resolve("minecraft:block/child")
	if err != nil {
		t.Fatal(err)
	}
	texture, _, err := child.Texture("#alias")
	if err != nil || texture != "minecraft:block/new" || len(child.Elements) != 1 {
		t.Fatalf("child inheritance: %s %v", texture, err)
	}
	parent, _ := resolver.Resolve("minecraft:block/base")
	texture, _, _ = parent.Texture("#alias")
	if texture != "minecraft:block/old" {
		t.Fatal("child changed the parent")
	}
	cleared, _ := resolver.Resolve("minecraft:block/empty")
	if len(cleared.Elements) != 0 {
		t.Fatal("explicit empty elements inherited geometry")
	}
}
