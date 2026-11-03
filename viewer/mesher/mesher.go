package mesher

import (
	"neomega-viewer-core/viewer/cube"
	blockgeometry "neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/mesher/geometry"
)

type Mesher[B blockgeometry.BlockID] struct {
	tables *blockgeometry.Tables[B]
	layout cube.Layout
}

// NewMesher 固定区域布局并引用只读几何表，所有层和邻居使用相同尺寸。
func NewMesher[B blockgeometry.BlockID](layout cube.Layout, tables *blockgeometry.Tables[B]) *Mesher[B] {
	return &Mesher[B]{tables: tables, layout: layout}
}

func (m *Mesher[B]) MeshBlock(id B) (geometry.Mesh, error) {
	parts, _, err := queryGeometry(m.tables, id, 0, nil)
	if err != nil {
		return geometry.Mesh{}, err
	}
	mesh := geometry.Mesh{Quads: []geometry.Quad{}}
	for _, part := range parts {
		// 输出独立网格，调用者修改它不会改变共享表。
		mesh.Quads = append(mesh.Quads, part.Quads...)
	}
	return mesh, nil
}
