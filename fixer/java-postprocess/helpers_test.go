package fixer

import (
	"iter"
	"slices"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/blockdata"
)

// 测试夹具用普通索引数组表达 16³ 布局，生产规则不依赖这种分块方式。
type Column map[int32]*blockdata.Blocks
type ChunkNeighbours [8]Column

const (
	NW = iota
	N
	NE
	W
	E
	SW
	S
	SE
)

var chunkOffsets = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

type sectionNeighbours [6][]ir.IRBlock

func localIndex(x, y, z int) int { return (x&15)<<8 | (z&15)<<4 | y&15 }

func palette(blocks []ir.IRBlock) *blockdata.Blocks {
	result := blockdata.Uniform(len(blocks), ir.IRBlock("minecraft:air []"))
	for i, state := range blocks {
		result.Set(i, state)
	}
	return result
}

func fixtureInput(center Column, grid map[[3]int]*blockdata.Blocks) Input {
	return Input{
		Bounds: HorizontalBounds{Max: [2]int{16, 16}},
		Blocks: func(accept func(ir.IRBlock) bool) iter.Seq2[[3]int, ir.IRBlock] {
			return func(yield func([3]int, ir.IRBlock) bool) {
				for y, blocks := range center {
					palette := blocks.Palette
					selected := make([]bool, len(palette))
					active := false
					for i, state := range palette {
						selected[i] = accept(state)
						active = active || selected[i]
					}
					if !active {
						continue
					}
					for i := range 4096 {

						index := int(blocks.Indices[i])
						if selected[index] && !yield([3]int{i >> 8, int(y)*16 + (i & 15), i >> 4 & 15}, palette[index]) {
							return
						}
					}
				}
			}
		},
		Get: func(x, y, z int) (ir.IRBlock, bool) {
			blocks := grid[[3]int{x >> 4, y >> 4, z >> 4}]
			if blocks == nil {
				return "minecraft:air []", false
			}
			return blocks.Get(localIndex(x, y, z)), true
		},
	}
}

func applyFixture(f *Fixer, center Column, grid map[[3]int]*blockdata.Blocks) Column {
	result := make(Column, len(center))
	for y, blocks := range center {
		result[y] = blocks
	}
	for pos, state := range f.Fix(fixtureInput(center, grid)) {
		y := int32(pos[1] >> 4)
		if result[y] == center[y] {
			result[y] = &blockdata.Blocks{Palette: slices.Clone(center[y].Palette), Indices: slices.Clone(center[y].Indices)}
		}
		result[y].Set(localIndex(pos[0], pos[1], pos[2]), state)
	}
	return result
}

func fixColumn(f *Fixer, center Column, neighbours ChunkNeighbours) Column {
	grid := make(map[[3]int]*blockdata.Blocks)
	for y, blocks := range center {
		grid[[3]int{0, int(y), 0}] = blocks
	}
	for side, column := range neighbours {
		for y, blocks := range column {
			grid[[3]int{chunkOffsets[side][0], int(y), chunkOffsets[side][1]}] = blocks
		}
	}
	return applyFixture(f, center, grid)
}

func fixPalette(f *Fixer, blocks *blockdata.Blocks, neighbours [6]*blockdata.Blocks) *blockdata.Blocks {
	grid := map[[3]int]*blockdata.Blocks{{}: blocks}
	for side, adjacent := range neighbours {
		if adjacent != nil {
			delta := sideOffset(side)
			grid[[3]int{delta.x, delta.y, delta.z}] = adjacent
		}
	}
	return applyFixture(f, Column{0: blocks}, grid)[0]
}

func fixSection(f *Fixer, blocks []ir.IRBlock, neighbours sectionNeighbours) []ir.IRBlock {
	var adjacent [6]*blockdata.Blocks
	for side, values := range neighbours {
		if values != nil {
			adjacent[side] = palette(values)
		}
	}
	fixed := fixPalette(f, palette(blocks), adjacent)
	result := make([]ir.IRBlock, len(blocks))
	for i := range result {
		result[i] = fixed.Get(i)
	}
	return result
}

func runPass(cares func(ir.IRBlock) bool, run func(snapshot) map[position]ir.IRBlock, blocks []ir.IRBlock, neighbours sectionNeighbours) map[int]ir.IRBlock {
	center := palette(blocks)
	grid := map[[3]int]*blockdata.Blocks{{}: center}
	for side, values := range neighbours {
		if values != nil {
			delta := sideOffset(side)
			grid[[3]int{delta.x, delta.y, delta.z}] = palette(values)
		}
	}
	input := fixtureInput(Column{0: center}, grid)
	view := snapshot{get: input.Get, solid: make(map[ir.IRBlock]uint8), bounds: input.Bounds}
	for pos, state := range input.Blocks(cares) {
		if cares(state) {
			view.targets = append(view.targets, target{position{pos[0], pos[1], pos[2]}, state})
		}
	}
	result := make(map[int]ir.IRBlock)
	for pos, state := range run(view) {
		result[cell(pos.x, pos.y, pos.z)] = state
	}
	return result
}
