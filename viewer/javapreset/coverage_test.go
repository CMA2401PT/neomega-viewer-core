package javapreset

import (
	"strings"
	"testing"

	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/entity_render"
	preset "neomega-viewer-core/viewer/resources"
	"neomega-viewer-core/viewer/splitlayer"
)

func TestFramedItemsInEveryOrientation(t *testing.T) {
	// 展示框的朝向与物品旋转独立组合，所有组合都应保留框体和物品。
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := NewPipeline(
		cube.New(1),
		resources.Tables,
		resources.Items,
		nil,
	)
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
				result, err := pipeline.Build(
					dispatcher.Section{Entities: &entity_render.Input{List: []entity_render.Entity{entity}}},
					nil,
					dispatcher.Options{},
				)
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

// Every legal target state must have the geometry used after water splitting.
// Moving pistons require runtime piston data and are explicitly unsupported.
func TestEveryTargetStateHasGeometry(t *testing.T) {
	// 枚举目标状态检查查表覆盖，再用每种方块的默认状态检查完整渲染组合。
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resources.Metadata.Version != java.JavaTargetVersion {
		t.Fatal(resources.Metadata.Version)
	}
	for state := range java.JavaFullStates() {
		if state.ShortName() == "moving_piston" {
			continue
		}
		for _, hash := range splitlayer.SplitBlock(state) {
			if _, err := resources.Tables.ModelIDs(hash); err != nil {
				t.Fatalf("%s: %v", state, err)
			}
		}
	}
	pipeline, err := NewPipeline(
		cube.New(1),
		resources.Tables,
		resources.Items,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for target := range java.JavaFullStates() {
		name := target.NameWithNameSpace()
		if seen[name] || target.ShortName() == "moving_piston" {
			continue
		}
		seen[name] = true
		properties, _ := java.BlockProperties(name)
		var defaults []string
		for key, property := range properties {
			defaults = append(defaults, `"`+key+`"=`+property.Default)
		}
		state := ir.IRBlock(name + " [" + strings.Join(defaults, ",") + "]").Tidy()
		result, err := pipeline.Build(
			dispatcher.Section{Blocks: blockdata.Uniform(1, state)},
			dispatcher.Neighbours{},
			dispatcher.Options{},
		)
		if err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		if len(result.Diagnostics) != 0 {
			t.Errorf("%s: %+v", state, result.Diagnostics)
		}
	}
}
