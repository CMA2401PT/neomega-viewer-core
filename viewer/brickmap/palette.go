package brickmap

import (
	"slices"

	"neomega-blocks/ir"
)

type Palette struct {
	// 把 Java 状态压成共享材质编号；不同状态的六面颜色和介质组相同时共用一条记录。
	source      *MaterialSource
	stateIDs    map[ir.IRBlock]uint16
	materials   []Material
	materialIDs map[Material]uint16
	unknown     map[string]bool
}

func NewPalette(source *MaterialSource) *Palette {
	// 每份体素数据独立编号，预留第 0 条记录表示空气。
	return &Palette{
		source:      source,
		stateIDs:    map[ir.IRBlock]uint16{},
		materials:   []Material{{}},
		materialIDs: map[Material]uint16{},
		unknown:     map[string]bool{},
	}
}

func (p *Palette) ID(block ir.IRBlock) uint16 {
	// 缓存已提取的状态并按材质值去重；超过 65535 种非空气材质时 panic。
	if id, ok := p.stateIDs[block]; ok {
		return id
	}
	switch block.ShortName() {
	case "", "air", "cave_air", "void_air", "barrier", "light", "structure_void":
		p.stateIDs[block] = 0
		return 0
	}
	m, known := p.source.Material(block)
	if !known {
		p.unknown[block.ShortName()] = true
	}
	id, ok := p.materialIDs[m]
	if !ok {
		if len(p.materials) == 65536 {
			panic("材质数量超过 65535")
		}
		id = uint16(len(p.materials))
		p.materials = append(p.materials, m)
		p.materialIDs[m] = id
	}
	p.stateIDs[block] = id
	return id
}

func (p *Palette) Materials() []Material {
	// 返回按编号排列的共享材质表；交给 Builder 前应先为全部方块分配编号。
	return p.materials
}

func (p *Palette) Unknown() []string {
	// 按短名去重并排序，供调用方报告使用回退色的方块。
	var names []string
	for name := range p.unknown {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
