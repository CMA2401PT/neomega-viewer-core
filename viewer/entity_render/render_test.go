package entity_render_test

import (
	"math"
	"reflect"
	"testing"

	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/entity_render"
	"neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/item_render"
)

func TestFrameItemComponents(t *testing.T) {
	body := geometry.Mesh{Quads: []geometry.Quad{{}}}
	item := geometry.Mesh{Quads: []geometry.Quad{{Color: [3]float32{.2, .4, .6}}}}
	catalog := &item_render.Catalog{Items: map[string]geometry.Mesh{"minecraft:diamond_sword": item}, Models: map[string]geometry.Mesh{"minecraft:block/item_frame": body}}
	renderer := entity_render.New(catalog, item_render.New(catalog))
	for _, components := range []any{nil, nbt.Compound{}, nbt.Compound{
		"minecraft:custom_name": nbt.Compound{"text": "展示用"},
		"minecraft:lore":        []string{"说明"}, "minecraft:damage": int32(12),
		"minecraft:enchantments": nbt.Compound{"minecraft:sharpness": int32(5)},
	}, nbt.Compound{"minecraft:item_model": "diamond_sword"}} {
		id := "minecraft:diamond_sword"
		values, _ := components.(nbt.Compound)
		if _, override := values["minecraft:item_model"]; override {
			id = "minecraft:stick"
		}
		stack := nbt.Compound{"id": id, "count": int32(1)}
		if components != nil {
			stack["components"] = components
		}
		entity := entity_render.Entity{ID: "minecraft:item_frame", NBT: nbt.Compound{"Facing": int8(2), "Item": stack}}
		data, err := nbt.EncodeToNBTBytes(entity.NBT, nbt.OptEncodeToNBT{Encoding: nbt.EncodingBigEndian})
		if err != nil {
			t.Fatal(err)
		}
		if err := nbt.DecodeFromNBTBytes(data, &entity.NBT, nbt.OptDecodeFromNBT{Encoding: nbt.EncodingBigEndian}); err != nil {
			t.Fatal(err)
		}
		mesh, issues := renderer.Build(entity_render.Input{List: []entity_render.Entity{entity}})
		if len(issues) != 0 || len(mesh.Quads) != 2 || mesh.Quads[1].Color != item.Quads[0].Color {
			t.Fatalf("components %v: mesh=%v, issues=%v", components, mesh, issues)
		}
		var beforeValue nbt.Compound
		if err := nbt.DecodeFromNBTBytes(data, &beforeValue, nbt.OptDecodeFromNBT{Encoding: nbt.EncodingBigEndian}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(beforeValue, entity.NBT) {
			t.Fatal("source NBT changed")
		}
	}
	for _, components := range []any{int32(1), nbt.Compound{"minecraft:dyed_color": int32(123)}, nbt.Compound{"minecraft:item_model": int32(1)}} {
		entity := entity_render.Entity{ID: "minecraft:item_frame", NBT: nbt.Compound{"Facing": int8(2), "Item": nbt.Compound{"id": "minecraft:diamond_sword", "components": components}}}
		mesh, issues := renderer.Build(entity_render.Input{List: []entity_render.Entity{entity}})
		if len(mesh.Quads) != 1 || len(issues) != 1 {
			t.Fatalf("unsupported appearance must keep frame: %v, %v", mesh, issues)
		}
	}
}

func TestFrameFacingAndItemRotation(t *testing.T) {
	// 固定物品坐标已由生成器居中；展示框只添加半尺寸、八向旋转和原版法线偏移。
	point := geometry.Vec3{.5, 0, 0}
	item := geometry.Mesh{Quads: []geometry.Quad{{Positions: [4]geometry.Vec3{point, point, point, point}}}}
	catalog := &item_render.Catalog{Items: map[string]geometry.Mesh{"example:item": item}}
	renderer := entity_render.New(catalog, item_render.New(catalog))
	for facing, want := range []geometry.Vec3{{0, -1.0 / 32, .25}, {0, 1.0 / 32, -.25}, {0, .25, -1.0 / 32}, {0, .25, 1.0 / 32}, {-1.0 / 32, .25, 0}, {1.0 / 32, .25, 0}} {
		input := entity_render.Entity{ID: "minecraft:item_frame", NBT: nbt.Compound{"Facing": int8(facing), "ItemRotation": int8(2), "Item": nbt.Compound{"id": "example:item"}}}
		// 可见框的物品基底深度为 7/16；隐藏框的基底深度为 1/2。
		catalog.Models = map[string]geometry.Mesh{"minecraft:block/item_frame": {}}
		mesh, issues := renderer.Build(entity_render.Input{List: []entity_render.Entity{input}})
		if len(issues) != 0 || len(mesh.Quads) != 1 {
			t.Fatalf("facing %d: mesh=%+v issues=%+v", facing, mesh, issues)
		}
		for axis, value := range mesh.Quads[0].Positions[0] {
			if math.Abs(value-want[axis]) > 1e-10 {
				t.Fatalf("facing %d: position %v, want %v", facing, mesh.Quads[0].Positions[0], want)
			}
		}
	}
}

func TestItemFailureKeepsFrameAndUnlitIsPreserved(t *testing.T) {
	// 物品错误必须留下框体与独立实体诊断，普通框不能覆盖物品已有的自发光材质。
	body := geometry.Mesh{Quads: []geometry.Quad{{}}}
	item := geometry.Mesh{Quads: []geometry.Quad{{}}}
	item.Quads[0].Surface.Unlit = true
	catalog := &item_render.Catalog{Items: map[string]geometry.Mesh{"example:item": item}, Models: map[string]geometry.Mesh{"minecraft:block/item_frame": body, "minecraft:block/glow_item_frame": body}}
	renderer := entity_render.New(catalog, item_render.New(catalog))
	frame := entity_render.Entity{ID: "minecraft:item_frame", Pos: geometry.Vec3{1, 2, 3}, NBT: nbt.Compound{"Facing": int8(2), "Item": nbt.Compound{"id": "example:missing"}}}
	mesh, issues := renderer.Build(entity_render.Input{List: []entity_render.Entity{frame}})
	if len(mesh.Quads) != 1 || len(issues) != 1 || issues[0].ID != frame.ID || issues[0].Pos != frame.Pos {
		t.Fatalf("partial failure: mesh=%+v, issues=%+v", mesh, issues)
	}
	frame.NBT["Item"] = nbt.Compound{"id": "example:item"}
	mesh, issues = renderer.Build(entity_render.Input{List: []entity_render.Entity{frame}})
	if len(issues) != 0 || !mesh.Quads[1].Surface.Unlit {
		t.Fatal("ordinary frame removed item's unlit material")
	}
	catalog.Items["example:item"] = body
	frame.ID = "minecraft:glow_item_frame"
	mesh, issues = renderer.Build(entity_render.Input{List: []entity_render.Entity{frame}})
	if len(issues) != 0 || mesh.Quads[0].Surface.Unlit || !mesh.Quads[1].Surface.Unlit {
		t.Fatal("glow frame must only force its item unlit")
	}
}
