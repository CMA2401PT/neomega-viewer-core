package fixer

import (
	"testing"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/blockdata"
)

func TestChunkDoorsAndOwnership(t *testing.T) {
	// 两半跨越负高度 section，必须先收齐整根柱子的修改才能交换互补属性。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	lower, upper := palette(airChunk(t)), palette(airChunk(t))
	var original [2]ir.IRBlock
	for i, half := range []string{"lower", "upper"} {
		original[i] = stateBlock(t, "spruce_door", map[string]string{
			"half": half, "facing": "east", "hinge": "right", "open": "true", "powered": "true",
		})
	}
	lowerPos, upperPos := localIndex(4, 15, 9), localIndex(4, 0, 9)
	lowerBefore, upperBefore := roundtripDoorBlock(t, original[0]), roundtripDoorBlock(t, original[1])
	lower.Set(lowerPos, lowerBefore)
	upper.Set(upperPos, upperBefore)
	untouched := palette(airChunk(t))
	center :=
		map[int32]*blockdata.Blocks{
			-1: lower, 0: upper, 2: untouched,
		}

	fixed := fixColumn(f, center, ChunkNeighbours{})
	assertDoorRestored(t, fixed[-1].Get(lowerPos), original[0], lowerBefore)
	assertDoorRestored(t, fixed[0].Get(upperPos), original[1], upperBefore)
	assertBlock(t, lower.Get(lowerPos), lowerBefore)
	assertBlock(t, upper.Get(upperPos), upperBefore)
	if fixed[-1] == center[-1] || fixed[0] == center[0] || fixed[2] != untouched {
		t.Fatal("changed sections must be copied and unchanged sections shared")
	}

}

func TestChunkNeighbourCoordinates(t *testing.T) {
	// 八个方向各放不同状态，检查统一取格函数的负坐标和邻居下标。
	positions := [8]position{
		NW: {x: -1, y: -17, z: -1}, N: {x: 3, y: -17, z: -1}, NE: {x: 16, y: -17, z: -1},
		W: {x: -1, y: -17, z: 4}, E: {x: 16, y: -17, z: 4},
		SW: {x: -1, y: -17, z: 16}, S: {x: 3, y: -17, z: 16}, SE: {x: 16, y: -17, z: 16},
	}
	var neighbours ChunkNeighbours
	names := [8]string{"stone", "dirt", "cobblestone", "gold_block", "iron_block", "diamond_block", "emerald_block", "sand"}
	for side, pos := range positions {
		blocks := palette(airChunk(t))
		blocks.Set(localIndex(pos.x, pos.y, pos.z), stateBlock(t, names[side], nil))
		neighbours[side] = map[int32]*blockdata.Blocks{-2: blocks}
	}
	center := map[int32]*blockdata.Blocks{-2: palette(airChunk(t))}
	f := &Fixer{passes: []pass{
		{
			cares: func(ir.IRBlock) bool { return true },
			run: func(view snapshot) map[position]ir.IRBlock {
				for side, pos := range positions {
					got, exists := view.get(pos.x, pos.y, pos.z)
					if !exists {
						t.Fatalf("missing neighbour %d", side)
					}
					assertBlock(t, got, stateBlock(t, names[side], nil))
				}
				got, exists := view.get(0, -17, 0)
				if !exists {
					t.Fatal("stored air marked missing")
				}
				assertBlock(t, got, "minecraft:air []")
				got, exists = view.get(0, -1, 0)
				if exists {
					t.Fatal("missing section marked present")
				}
				assertBlock(t, got, "minecraft:air []")
				return nil
			},
		},
	}}
	fixed := fixColumn(f, center, neighbours)
	if fixed[-2] != center[-2] {
		t.Fatal("unchanged chunk must retain its section")
	}
}

func TestChunkRedstoneAcrossTwoAxes(t *testing.T) {
	// 西邻居的上一 section 可以排除坡道；缺这份数据时仍保留输入形状。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	wire := stateBlock(t, "redstone_wire", map[string]string{
		"power": "11", "north": "up", "east": "up", "south": "up", "west": "up",
	})
	blocks := palette(airChunk(t))
	blocks.Set(localIndex(0, 15, 8), wire)
	blocks.Set(localIndex(1, 15, 8), wire)
	center := map[int32]*blockdata.Blocks{
		0: blocks, 1: palette(airChunk(t)),
	}
	west := map[int32]*blockdata.Blocks{
		0: palette(airChunk(t)), 1: palette(airChunk(t)),
	}
	neighbours := ChunkNeighbours{W: west}
	fixed := fixColumn(f, center, neighbours)
	want := stateBlock(t, "redstone_wire", map[string]string{
		"power": "11", "north": "none", "east": "side", "south": "none", "west": "side",
	})
	assertBlock(t, fixed[0].Get(localIndex(0, 15, 8)), want)
	assertBlock(t, blocks.Get(localIndex(0, 15, 8)), wire)
	west[1].Set(localIndex(15, 0, 8), wire)
	fixed = fixColumn(f, center, neighbours)
	assertBlock(t, fixed[0].Get(localIndex(0, 15, 8)), wire)
	assertBlock(t, west[1].Get(localIndex(15, 0, 8)), wire)
	delete(west, 1)
	fixed = fixColumn(f, center, neighbours)
	assertBlock(t, fixed[0].Get(localIndex(0, 15, 8)), wire)
}
