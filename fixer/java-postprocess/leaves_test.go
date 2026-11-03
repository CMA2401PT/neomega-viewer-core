package fixer

import (
	"strconv"
	"testing"

	"neomega-viewer-core/viewer/blockdata"
)

func TestLeafDistances(t *testing.T) {
	f := &Fixer{}
	chunk := airChunk(t)
	chunk[cell(8, 1, 8)] = stateBlock(t, "oak_log", nil)
	for y := 2; y <= 8; y++ {
		chunk[cell(8, y, 8)] = stateBlock(t, "oak_leaves", map[string]string{
			"persistent": "false", "distance": "7", "waterlogged": "true",
		})
	}
	// 玩家树叶保持原状态，同时连接上下两段自然树叶。
	chunk[cell(8, 4, 8)] = stateBlock(t, "oak_leaves", map[string]string{
		"persistent": "true", "distance": "7",
	})
	changes := runPass(leafTarget, f.leafDistances, chunk, sectionNeighbours{})
	for y := 2; y <= 7; y++ {
		if y == 4 {
			continue
		}
		assertBlock(t, changes[cell(8, y, 8)], stateBlock(t, "oak_leaves", map[string]string{
			"persistent": "false", "distance": strconv.Itoa(y - 1), "waterlogged": "true",
		}))
	}
	if len(changes) != 5 {
		t.Fatalf("got %d changes, want only five reachable natural leaves", len(changes))
	}
	for index, block := range changes {
		chunk[index] = block
	}
	if len(runPass(leafTarget, f.leafDistances, chunk, sectionNeighbours{})) != 0 {
		t.Fatal("unchanged distances were reported as changes")
	}
}

func TestLeafBoundaryDistances(t *testing.T) {
	f := &Fixer{}
	for _, pos := range []position{
		{x: 0, y: 8, z: 8}, {x: 5, y: 8, z: 8}, {x: 6, y: 8, z: 8},
		{x: 9, y: 8, z: 8}, {x: 10, y: 8, z: 8}, {x: 15, y: 8, z: 8},
		{x: 8, y: 8, z: 0}, {x: 8, y: 8, z: 5}, {x: 8, y: 8, z: 6},
		{x: 8, y: 8, z: 9}, {x: 8, y: 8, z: 10}, {x: 8, y: 8, z: 15},
	} {
		chunk := airChunk(t)
		chunk[cell(pos.x, pos.y, pos.z)] = stateBlock(t, "oak_leaves", map[string]string{
			"persistent": "false", "distance": "1",
		})
		distance := "6"
		if pos.x >= 6 && pos.x <= 9 && pos.z >= 6 && pos.z <= 9 {
			distance = "7"
		}
		changes := runPass(leafTarget, f.leafDistances, chunk, sectionNeighbours{})
		assertBlock(t, changes[cell(pos.x, pos.y, pos.z)], stateBlock(t, "oak_leaves", map[string]string{
			"persistent": "false", "distance": distance,
		}))
	}
}

func TestLeafLogFamilies(t *testing.T) {
	f := &Fixer{}
	for _, test := range []struct{ name, distance string }{
		{name: "oak_log", distance: "1"},
		{name: "stripped_oak_wood", distance: "1"},
		{name: "crimson_stem", distance: "1"},
		{name: "warped_hyphae", distance: "1"},
		{name: "mushroom_stem", distance: "7"},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := airChunk(t)
			chunk[cell(8, 7, 8)] = stateBlock(t, test.name, nil)
			chunk[cell(8, 8, 8)] = stateBlock(t, "oak_leaves", map[string]string{
				"persistent": "false", "distance": "2",
			})
			changes := runPass(leafTarget, f.leafDistances, chunk, sectionNeighbours{})
			assertBlock(t, changes[cell(8, 8, 8)], stateBlock(t, "oak_leaves", map[string]string{
				"persistent": "false", "distance": test.distance,
			}))
		})
	}
}

func TestLeafDistancesAcrossSections(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	lower, upper := palette(airChunk(t)), palette(airChunk(t))
	lower.Set(localIndex(8, -1, 8), stateBlock(t, "oak_log", nil))
	leaves := stateBlock(t, "oak_leaves", map[string]string{"persistent": "false", "distance": "7"})
	upper.Set(localIndex(8, 0, 8), leaves)
	upper.Set(localIndex(15, 0, 8), leaves)
	center := map[int32]*blockdata.Blocks{
		-1: lower, 0: upper,
	}
	east := palette(airChunk(t))
	east.Set(localIndex(0, 0, 8), stateBlock(t, "oak_log", nil))
	fixed := fixColumn(f, center, ChunkNeighbours{E: map[int32]*blockdata.Blocks{0: east}})
	assertBlock(t, fixed[0].Get(localIndex(8, 0, 8)), stateBlock(t, "oak_leaves", map[string]string{
		"persistent": "false", "distance": "1",
	}))
	assertBlock(t, fixed[0].Get(localIndex(15, 0, 8)), stateBlock(t, "oak_leaves", map[string]string{
		"persistent": "false", "distance": "6",
	}))
	assertBlock(t, upper.Get(localIndex(8, 0, 8)), leaves)
}
