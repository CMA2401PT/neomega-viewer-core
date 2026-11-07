package topview

import (
	"cmp"
	"image/color"
	"slices"

	"neomega-blocks/ir"
)

// Section 是区块柱中的一层 16³。
// Blocks 为 nil 或 4096 个方块，下标为 (x<<8)|(z<<4)|y；nil 表示这一层没有方块。
type Section struct {
	Y      int32
	Blocks []ir.IRBlock
}

// Column 是 16×16 的俯视结果，下标为 z*16+x。
// 可见方块的 Alpha 为 255，空格为 0；空格的 Height 不使用。
type Column struct {
	Pixel  [256]color.NRGBA
	Height [256]int32
}

func (t Table) RenderColumn(sections []Section, fallback uint32) Column {
	// 从各层最高处往下找第一格可见方块，并用颜色表取色。
	// 表里没有的状态使用 fallback。调用方传入已经解开的方块，这里不读存档。
	order := make([]int, len(sections))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		return cmp.Compare(sections[b].Y, sections[a].Y)
	})
	var column Column
	for x := range 16 {
		for z := range 16 {
			pixel := z*16 + x
		sections:
			for _, index := range order {
				section := sections[index]
				if section.Blocks == nil {
					continue
				}
				base := int(section.Y) * 16
				for y := 15; y >= 0; y-- {
					block := section.Blocks[(x<<8)|(z<<4)|y]
					if hidden(block) {
						continue
					}
					rgb, ok := t.Color(block)
					if !ok {
						rgb = fallback
					}
					column.Pixel[pixel] = color.NRGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: 255}
					column.Height[pixel] = int32(base + y)
					break sections
				}
			}
		}
	}
	return column
}

func hidden(block ir.IRBlock) bool {
	// 无几何的空气和辅助方块不遮挡下方地形。
	switch block.ShortName() {
	case "", "air", "cave_air", "void_air", "barrier", "light", "structure_void":
		return true
	default:
		return false
	}
}
