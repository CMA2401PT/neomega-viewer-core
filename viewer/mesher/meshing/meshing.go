// Package meshing 按固定区域布局摆放共享几何、剔除遮挡面，并合并相容矩形。
package meshing

import (
	"neomega-viewer-core/viewer/cube"
	blockgeometry "neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/geometry/material"
	"neomega-viewer-core/viewer/mesher/geometry"
)

// Builder 使用固定尺寸处理所有输入和邻居，调用之间不共享可变缓存。
type Builder struct{ layout cube.Layout }

// New 固定几何构建的坐标布局，输入格数应为 layout.Len()。
func New(layout cube.Layout) *Builder { return &Builder{layout: layout} }

type LayerShape struct {
	// 层号属于摆放位置，嵌入引用可保持共享几何只读。
	*geometry.Shape
	Layer int
}

type Cell []LayerShape

// Cells 按构建器的 X-Z-Y 顺序排列，几何仍位于各自的格内坐标。
type Cells []Cell

// Boundaries 保存六个邻接面，每个已提供的面有 N² 格；nil 面表示空气。
type Boundaries [6][]Cell

type Options struct {
	Neighbours     *Boundaries
	DisableCulling bool
	DisableGreedy  bool
}

type Stats struct {
	InputQuads   int `json:"inputQuads"`
	RemovedQuads int `json:"removedQuads"`
	ClippedQuads int `json:"clippedQuads"`
	VisibleQuads int `json:"visibleQuads"`
	OutputQuads  int `json:"outputQuads"`
}

// Build 只读输入与共享形状，返回区域网格及剔面、裁切和合并统计。
func (b *Builder) Build(cells Cells, options Options) (geometry.Mesh, Stats) {
	quads, stats := b.cullAndPlace(cells, options.DisableCulling, options.Neighbours)
	stats.VisibleQuads = len(quads)
	if !options.DisableGreedy {
		quads = merge(quads)
	}
	stats.OutputQuads = len(quads)
	return geometry.Mesh{Quads: quads}, stats
}

func (b *Builder) cullAndPlace(cells Cells, disableCulling bool, neighbours *Boundaries) ([]geometry.Quad, Stats) {
	quads := []geometry.Quad{}
	stats := Stats{}
	cull := culler{layout: b.layout, shapes: make(map[*geometry.Shape]*inspectedShape)}
	for index, cell := range cells {
		pos := b.layout.Position(index)
		position := [3]int{int(pos[0]), int(pos[1]), int(pos[2])}
		for _, part := range cell {
			scale := cull.layerScale(cell, part.Layer)
			var info *inspectedShape
			if !disableCulling && scale == 1 {
				info = cull.shape(part.Shape)
			}
			for i, quad := range part.Quads {
				stats.InputQuads++
				// 镂空前景下的缩进层保持原有规则，直接摆放，不参与剔面。
				if info != nil {
					quads = cull.appendCulled(
						quads,
						quad,
						&info.faces[i],
						part,
						position,
						cells,
						neighbours,
						&stats,
					)
				} else {
					quads = append(quads, place(quad, position, scale))
				}
			}
		}
	}
	return quads, stats
}

func unitCube(shape *geometry.Shape) bool {
	// 六面的矩形并集决定是否满格，镂空和透明材质同样提供缩进依据。
	var clip clipBuffers
	for _, covers := range shape.Boundary {
		clip.reset(box{Max: [2]float64{1, 1}})
		for _, cover := range covers {
			clip.subtract(box{Min: [2]float64(cover.Min), Max: [2]float64(cover.Max)})
			if len(clip.remaining) == 0 {
				break
			}
		}
		if len(clip.remaining) != 0 {
			return false
		}
	}
	return true
}

func place(quad geometry.Quad, position [3]int, scale float64) geometry.Quad {
	for vertex := range quad.Positions {
		for axis := range position {
			if scale != 1 {
				quad.Positions[vertex][axis] = .5 + (quad.Positions[vertex][axis]-.5)*scale
			}
			quad.Positions[vertex][axis] += float64(position[axis])
		}
	}
	return quad
}

var sideNames = [6]string{"west", "east", "down", "up", "north", "south"}

func occludes(cover, candidate geometry.Quad, coverGroup, candidateGroup uint32) bool {
	if material.FullyOpaque(cover.Surface) {
		return true
	}
	// A transparent medium may remove its own internal interfaces, but never
	// mask an opaque face even if a caller accidentally reuses its group.
	return candidate.Surface.Mode != material.Opaque &&
		candidateGroup != 0 && candidateGroup == coverGroup
}

const epsilon = blockgeometry.Epsilon

func close(a, b float64) bool { return blockgeometry.Close(a, b) }
