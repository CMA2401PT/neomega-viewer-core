// Package blockdata 提供不含尺寸的渲染输入，索引顺序由构建器约定。
package blockdata

import (
	"slices"

	"neomega-blocks/ir"
)

// Blocks 以调色板保存完整 Java 状态，Indices 每个 uint32 对应一格，不打包位。
// 展开形式供渲染时连续扫描；压缩存储的解包由应用适配完成。
// 构建期间两者均保持只读；Indices 的长度必须等于构建器的 N³。
type Blocks struct {
	Palette []ir.IRBlock
	Indices []uint32
}

// Uniform 创建指定格数的同状态输入，不推断它的空间尺寸。
func Uniform(count int, state ir.IRBlock) *Blocks {
	return &Blocks{Palette: []ir.IRBlock{state}, Indices: make([]uint32, count)}
}

// Get 按构建器约定的连续索引读取完整状态。
func (b *Blocks) Get(index int) ir.IRBlock { return b.Palette[b.Indices[index]] }

// Set 用于构造输入，复用已有状态或追加新状态；不会改写其他格。
func (b *Blocks) Set(index int, state ir.IRBlock) {
	slot := slices.Index(b.Palette, state)
	if slot < 0 {
		slot = len(b.Palette)
		b.Palette = append(b.Palette, state)
	}
	b.Indices[index] = uint32(slot)
}
