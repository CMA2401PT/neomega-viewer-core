package geometry

import (
	"maps"
	"slices"
	"testing"

	java "neomega-blocks/presets/java/v1"
	blockgeometry "neomega-viewer-core/viewer/geometry"
)

func TestTablesWithJavaResources(t *testing.T) {
	engine, err := Open(OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tables := engine.Tables
	again := engine.Tables
	if again != tables {
		t.Fatal("tables were not shared")
	}
	states := slices.Collect(java.JavaFullStates())
	lookup := func(name string, properties map[string]any, count int) []blockgeometry.ModelID {
		t.Helper()
		for _, block := range states {
			hash := block.Hash()
			if block.NameWithNameSpace() != "minecraft:"+name || !maps.Equal(block.States().GoMap(), properties) {
				continue
			}
			ids, err := tables.ModelIDs(hash)
			if err != nil || len(ids) != count {
				t.Fatalf("%s %v: IDs=%v, want %d models, error=%v", name, properties, ids, count, err)
			}
			for _, id := range ids {
				shape, err := tables.Shape(id)
				if err != nil || len(shape.Quads) == 0 {
					t.Fatalf("%s: empty or missing shape %d: %v", name, id, err)
				}
			}
			return ids
		}
		t.Fatalf("state not found: %s %v", name, properties)
		return nil
	}

	fence := map[string]any{
		"north": "false", "east": "false", "south": "false", "west": "false",
	}
	post := lookup("oak_fence", fence, 1)
	fence["north"], fence["east"] = "true", "true"
	connected := lookup("oak_fence", fence, 3)
	if post[0] != connected[0] {
		t.Fatal("fence states did not share the post")
	}
	if connected[1] == connected[2] {
		t.Fatal("differently rotated fence rails were merged")
	}
	fence["south"] = "true"
	if !slices.Equal(connected, lookup("oak_fence", fence, 4)[:3]) {
		t.Fatal("additional fence connection duplicated existing parts")
	}

	slab := map[string]any{"type": "bottom"}
	bottom := lookup("oak_slab", slab, 1)
	slab["type"] = "top"
	top := lookup("oak_slab", slab, 1)
	slab["type"] = "double"
	double := lookup("oak_slab", slab, 1)
	if bottom[0] == top[0] || bottom[0] == double[0] || top[0] == double[0] {
		t.Fatal("different slab shapes were merged")
	}
	shape, _ := tables.Shape(bottom[0])
	if len(shape.Boundary[3]) != 0 || len(shape.Boundary[2]) != 1 {
		t.Fatal("slab top inside the cell was counted as a boundary")
	}
	if len(shape.Boundary[4]) != 1 || shape.Boundary[4][0].Max != (blockgeometry.Vec2{1, 0.5}) {
		t.Fatalf("slab partial coverage lost: %+v", shape.Boundary[4])
	}

	wire := map[string]any{"north": "none", "east": "none", "south": "none", "west": "none", "power": "0"}
	dot := lookup("redstone_wire", wire, 1)
	wire["north"], wire["east"] = "side", "side"
	corner := lookup("redstone_wire", wire, 3)
	if dot[0] != corner[0] {
		t.Fatal("redstone states did not share their dot")
	}
	wire["power"] = "15"
	powered := lookup("redstone_wire", wire, 3)
	t.Run("RedstonePowerIsBakedIntoQuadColors", func(t *testing.T) {
		for part, id := range corner {
			unpoweredShape, err := tables.Shape(id)
			if err != nil {
				t.Fatal(err)
			}
			poweredShape, err := tables.Shape(powered[part])
			if err != nil {
				t.Fatal(err)
			}
			if id == powered[part] || len(unpoweredShape.Quads) != len(poweredShape.Quads) {
				t.Fatalf("power states merged or changed geometry in part %d", part)
			}
			changed := false
			for i, quad := range unpoweredShape.Quads {
				other := poweredShape.Quads[i]
				// 叠加纹理可以不染色，带色的导线面必须体现 power。
				changed = changed || quad.Color != other.Color
				other.Color = quad.Color
				if quad != other {
					t.Fatalf("power changed more than color in part %d quad %d", part, i)
				}
			}
			if !changed {
				t.Fatalf("power 0 and 15 share all quad colors in part %d", part)
			}
		}
	})

	stone := lookup("stone", nil, 1)
	shape, _ = tables.Shape(stone[0])
	if len(shape.Quads) != 6 {
		t.Fatalf("stone has %d quads", len(shape.Quads))
	}
	for side, rectangles := range shape.Boundary {
		if len(rectangles) != 1 || rectangles[0].Min != (blockgeometry.Vec2{}) || rectangles[0].Max != (blockgeometry.Vec2{1, 1}) {
			t.Fatalf("stone boundary %d: %+v", side, rectangles)
		}
	}
	lookup("air", nil, 0)
	lookup("water", map[string]any{"level": "0"}, 1)
	lookup("lava", map[string]any{"level": "8"}, 1)

	compatible := slices.Collect(java.JavaCompatibleStates())
	if len(tables.Index)+len(engine.Metadata.Skipped) != len(compatible) {
		t.Fatal("some Java states were neither indexed nor reported")
	}
	native := map[uint32]bool{}
	for _, state := range states {
		native[state.Hash()] = true
	}
	nativeSkipped := 0
	for _, skipped := range engine.Metadata.Skipped {
		if native[skipped.Block.Hash()] {
			nativeSkipped++
		}
	}
	if nativeSkipped != 12 {
		t.Fatalf("unexpected skipped states: %+v", engine.Metadata.Skipped)
	}
	for _, skipped := range engine.Metadata.Skipped {
		if (native[skipped.Block.Hash()] && skipped.Block.NameWithNameSpace() != "minecraft:moving_piston") || skipped.Reason == "" {
			t.Fatalf("unexpected skipped state: %+v", skipped)
		}
		if _, err := tables.ModelIDs(skipped.Block.Hash()); err == nil {
			t.Fatal("skipped state has an index entry")
		}
	}
	for mapping := range java.JavaCompatibility() {
		want, ok := tables.Index[mapping.Target.Hash()]
		if !ok {
			continue
		}
		got, ok := tables.Index[mapping.Source.Hash()]
		if !ok || !slices.Equal(got, want) {
			t.Fatalf("compatibility shape mismatch: %+v", mapping)
		}
	}

	references := 0
	used := map[blockgeometry.ModelID]bool{}
	for _, ids := range tables.Index {
		references += len(ids)
		for _, id := range ids {
			used[id] = true
		}
	}
	for _, id := range tables.FullHeight {
		used[id] = true
	}
	if len(used) != len(tables.Shapes) || references <= len(tables.Shapes) {
		t.Fatal("geometry table has unused shapes or did not deduplicate")
	}
	t.Logf("Java states=%d; indexed=%d; shapes=%d; references=%d; skipped=%d (minecraft:moving_piston)",
		len(states), len(tables.Index), len(tables.Shapes), references, len(engine.Metadata.Skipped))
}
