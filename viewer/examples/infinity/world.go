package main

import (
	"context"
	"math"
	"math/rand/v2"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
)

var (
	// 世界的数据来源和构建器共用固定 16³ 布局。
	layout = cube.New(16)
	air    = ir.IRBlock("minecraft:air []")
	stone  = ir.IRBlock("minecraft:stone []")
	dirt   = ir.IRBlock("minecraft:dirt []")
	grass  = ir.IRBlock(`minecraft:grass_block ["snowy"="false"]`)
	pots   = []ir.IRBlock{
		ir.IRBlock(`minecraft:decorated_pot ["cracked"="false","facing"="north","waterlogged"="false"]`),
		ir.IRBlock(`minecraft:decorated_pot ["cracked"="false","facing"="east","waterlogged"="false"]`),
		ir.IRBlock(`minecraft:decorated_pot ["cracked"="false","facing"="south","waterlogged"="false"]`),
		ir.IRBlock(`minecraft:decorated_pot ["cracked"="false","facing"="west","waterlogged"="false"]`),
	}
	sherds = []string{
		"minecraft:angler_pottery_sherd",
		"minecraft:archer_pottery_sherd",
		"minecraft:blade_pottery_sherd",
		"minecraft:arms_up_pottery_sherd",
	}
)

// surfaceHeight 用世界方块坐标生成连续低丘；高度在 1..9，摆设放在其上一格，均落在 Y=0 区域。
func surfaceHeight(x, z int64) int {
	return 5 + int(math.Floor(2*math.Sin(float64(x)/17)+2*math.Cos(float64(z)/23)))
}

// generatedWorld 按坐标重建地形和装饰，不持有已生成的地图。
type generatedWorld struct{}

// Section 提供指定位置的方块与实体数据；地形和装饰都位于 Y=0 子区块。
func (generatedWorld) Section(_ context.Context, pos dispatcher.SectionPos) (*dispatcher.Section, error) {
	if pos.Y != 0 {
		return nil, nil
	}
	// 1. 从空气开始填充完整状态；同一布局决定格子数和 X-Z-Y 索引顺序。
	section := &dispatcher.Section{
		Blocks:        blockdata.Uniform(layout.Len(), air),
		BlockEntities: map[dispatcher.CellPos]nbt.Compound{},
	}
	var side int = layout.Side()
	var originX int64 = int64(pos.X) * int64(side)
	var originZ int64 = int64(pos.Z) * int64(side)
	for x := 0; x < side; x++ {
		for z := 0; z < side; z++ {
			var top int = surfaceHeight(originX+int64(x), originZ+int64(z))
			for y := 0; y <= top; y++ {
				state := stone
				if y == top {
					state = grass
				} else if y == top-1 {
					state = dirt
				}
				section.Blocks.Set(layout.Index(x, y, z), state)
			}
		}
	}
	// 2. 以区域坐标作为随机种子，卸载后再回来仍是同一组摆设。
	// 附魔台放在 X=2..6，陶罐放在 X=10..13，保证两者不重叠且不跨边界。
	random := rand.New(rand.NewPCG(uint64(int64(pos.X)), uint64(int64(pos.Z))))
	var x int = 2 + random.IntN(5)
	var z int = 2 + random.IntN(12)
	var y int = surfaceHeight(originX+int64(x), originZ+int64(z)) + 1
	section.Blocks.Set(layout.Index(x, y, z), ir.IRBlock("minecraft:enchanting_table []"))
	x, z = 10+random.IntN(4), 2+random.IntN(12)
	y = surfaceHeight(originX+int64(x), originZ+int64(z)) + 1
	section.Blocks.Set(layout.Index(x, y, z), pots[random.IntN(len(pots))])
	// 3. 方块实体 NBT 关联局部坐标；四片陶片按 back、left、right、front 排列。
	var items []string = make([]string, 4)
	for side := range items {
		items[side] = sherds[random.IntN(len(sherds))]
	}
	section.BlockEntities[dispatcher.CellPos{uint8(x), uint8(y), uint8(z)}] = nbt.Compound{"sherds": items}
	return section, nil
}
