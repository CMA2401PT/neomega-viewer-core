package javapreset

import (
	"testing"

	current "neomega-blocks/presets/java/v26_3"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/entity_render"
	preset "neomega-viewer-core/viewer/resources"
	"neomega-viewer-core/viewer/splitlayer"
)

func TestPreview26FramedItems(t *testing.T) {
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := NewPipeline(cube.New(1), resources.Tables, resources.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"diamond_sword", "bow", "crossbow", "brush", "spyglass", "trident", "fishing_rod", "iron_chestplate", "poplar_hanging_sign", "sulfur_spike"} {
		for facing := range 6 {
			for rotation := range 8 {
				entity := entity_render.Entity{ID: "minecraft:glow_item_frame", NBT: nbt.Compound{
					"Facing": int8(facing), "ItemRotation": int8(rotation),
					"Item": nbt.Compound{"id": "minecraft:" + id, "count": int32(1), "components": nbt.Compound{"minecraft:custom_name": "展示", "minecraft:enchantments": nbt.Compound{}}},
				}}
				result, err := pipeline.Build(dispatcher.Section{Entities: &entity_render.Input{List: []entity_render.Entity{entity}}}, nil, dispatcher.Options{})
				if err != nil {
					t.Fatal(err)
				}
				if len(result.EntityDiagnostics) != 0 || len(result.Frame.Mesh.Quads) <= len(resources.Items.Models["minecraft:block/glow_item_frame"].Quads) {
					t.Fatalf("%s facing=%d rotation=%d: %+v", id, facing, rotation, result.EntityDiagnostics)
				}
			}
		}
	}
}

// Every legal 26.3 state must have the geometry used after water splitting.
// Moving pistons require runtime piston data and are explicitly unsupported.
func TestPreview26AllStates(t *testing.T) {
	catalog, err := current.Open()
	if err != nil {
		t.Fatal(err)
	}
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resources.Metadata.Version != "26.3" {
		t.Fatal(resources.Metadata.Version)
	}
	for state := range catalog.States() {
		if state.Java.ShortName() == "moving_piston" {
			continue
		}
		for _, hash := range splitlayer.SplitBlock(state.Java) {
			if _, err := resources.Tables.ModelIDs(hash); err != nil {
				t.Fatalf("%s: %v", state.Java, err)
			}
		}
	}
	pipeline, err := NewPipeline(cube.New(1), resources.Tables, resources.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	for definition := range catalog.Blocks() {
		state, _ := catalog.ByID(definition.DefaultStateID)
		if state.Java.ShortName() == "moving_piston" {
			continue
		}
		result, err := pipeline.Build(dispatcher.Section{Blocks: blockdata.Uniform(1, state.Java)}, dispatcher.Neighbours{}, dispatcher.Options{})
		if err != nil {
			t.Fatalf("%s: %v", state.Java, err)
		}
		if len(result.Diagnostics) != 0 {
			t.Errorf("%s: %+v", state.Java, result.Diagnostics)
		}
	}
}
