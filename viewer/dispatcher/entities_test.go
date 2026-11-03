package dispatcher_test

import (
	"testing"

	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/entity_render"
	"neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/javapreset"
	preset "neomega-viewer-core/viewer/resources"
)

func TestEntityListOwnsOutputWithoutBlocks(t *testing.T) {
	// 无方块区域也输出框体；宽松查询和邻居列表都不能增加本区域的实体输出。
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := javapreset.NewPipeline(cube.New(16), resources.Tables, resources.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	entity := entity_render.Entity{ID: "minecraft:item_frame", Pos: geometry.Vec3{1.5, 2.5, 3.5}, NBT: nbt.Compound{"Facing": int8(2)}}
	input := &entity_render.Input{List: []entity_render.Entity{entity}, Query: func(geometry.Bounds) []entity_render.Entity {
		t.Fatal("standalone frames must not query neighbors")
		return []entity_render.Entity{entity, entity}
	}}
	center := dispatcher.Section{Entities: input}
	result, err := pipeline.Build(center, dispatcher.Neighbours{&center}, dispatcher.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(result.Frame.Mesh.Quads), len(resources.Items.Models["minecraft:block/item_frame"].Quads); got != want || got == 0 {
		t.Fatalf("entity output quads: %d, want %d", got, want)
	}
	if len(result.Frame.Actors) != 0 || len(result.EntityDiagnostics) != 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected output: %+v", result)
	}
}
