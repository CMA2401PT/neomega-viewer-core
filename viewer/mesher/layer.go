package mesher

import (
	"fmt"

	blockgeometry "neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/mesher/geometry"
	"neomega-viewer-core/viewer/mesher/meshing"
)

type Options struct {
	Meshing meshing.Options
	// 缺失 ID 可直接引用调用方的形状，避免向共享表注册临时 ID。
	MissingShape *blockgeometry.Shape
}

type Stats struct {
	meshing.Stats
	// Blocks counts cells with local geometry before neighbor culling.
	Blocks int `json:"blocks"`
	// 统计实际引用的共享形状，不再表示运行时烘焙次数。
	Shapes int `json:"shapes"`
}

type MissingCell struct {
	// 仅标记中心 section，邻居占位只用于覆盖查询。
	Pos   [3]uint8
	Layer int
}

type Result struct {
	Mesh    geometry.Mesh
	Missing []MissingCell
	Stats   Stats
}

// Neighbours 按相对中心的区域偏移（X、Y、Z）提供多层方块；缺失位置按空气处理。
type Neighbours[B blockgeometry.BlockID] map[[3]int][][]B

// 六个接触面仍按西、东、下、上、北、南排列，与 meshing.Boundaries 对应。
var adjacentOffsets = [6][3]int{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}}

// NeighbourOffsets 返回构建中心网格所需的区域偏移，包含判定邻居流体高度的上方区域。
func (m *Mesher[B]) NeighbourOffsets() [][3]int {
	offsets := append([][3]int(nil), adjacentOffsets[:]...)
	for _, offset := range adjacentOffsets {
		if offset[1] == 0 {
			offset[1] = 1
			offsets = append(offsets, offset)
		}
	}
	if m.layout.Side() == 1 {
		// 一格区域的上邻居也位于自身顶层，其形状还取决于再上一格。
		offsets = append(offsets, [3]int{0, 2, 0})
	}
	return offsets
}

// MeshLayer 构建一层 N³ 个状态 ID，外部邻居按空气处理。
func (m *Mesher[B]) MeshLayer(blocks []B, options Options) (*Result, error) {
	return m.MeshLayers([][]B{blocks}, options)
}

// MeshLayers 将同格的各层一起剔面与合并，外部邻居按空气处理。
func (m *Mesher[B]) MeshLayers(layerBlocks [][]B, options Options) (*Result, error) {
	return m.MeshWithNeighbours(layerBlocks, nil, options)
}

// MeshWithNeighbours 根据相对坐标查询邻居并剔面，所需位置见 NeighbourOffsets；输入保持只读。
func (m *Mesher[B]) MeshWithNeighbours(layerBlocks [][]B, neighbours Neighbours[B], options Options) (*Result, error) {
	// 每层遵守构建器的 N³ 约定，层号决定同格共面的覆盖先后。
	blockCount := m.layout.Len()
	for layer, blocks := range layerBlocks {
		if len(blocks) != blockCount {
			return nil, fmt.Errorf("layer %d: region requires %d block IDs, got %d", layer, blockCount, len(blocks))
		}
	}
	for offset, layers := range neighbours {
		for layer, blocks := range layers {
			if len(blocks) != blockCount {
				return nil, fmt.Errorf("neighbour %v layer %d: expected %d blocks, got %d", offset, layer, blockCount, len(blocks))
			}
		}
	}
	boundaries, err := m.boundaries(layerBlocks, neighbours, options.MissingShape)
	if err != nil {
		return nil, err
	}
	cells, missing, err := m.prepareNeighborhood(layerBlocks, boundaries, options.MissingShape)
	if err != nil {
		return nil, err
	}
	options.Meshing.Neighbours = boundaries
	mesh, stats := meshing.New(m.layout).Build(cells, options.Meshing)
	visibleBlocks, shapes := countGeometry(cells)
	return &Result{
		Mesh:    mesh,
		Missing: missing,
		Stats: Stats{
			Stats:  stats,
			Blocks: visibleBlocks,
			Shapes: shapes,
		},
	}, nil
}

type localGeometry struct {
	meshing.LayerShape
	// 满高度引用兼作流体标记，普通形状不需要邻居选择。
	fullHeight *geometry.Shape
}

func queryGeometry[B blockgeometry.BlockID](
	tables *blockgeometry.Tables[B],
	id B,
	layer int,
	missing *blockgeometry.Shape,
) ([]localGeometry, bool, error) {
	// 空模型列表是有意无几何；只有键不存在才使用占位。
	ids, found := tables.Index[id]
	if !found {
		if missing != nil {
			return []localGeometry{{LayerShape: meshing.LayerShape{Shape: missing, Layer: layer}}}, true, nil
		}
		_, err := tables.ModelIDs(id)
		return nil, true, err
	}
	parts := make([]localGeometry, 0, len(ids))
	for _, id := range ids {
		shape, err := tables.Shape(id)
		if err != nil {
			return nil, false, err
		}
		local := localGeometry{LayerShape: meshing.LayerShape{Shape: shape, Layer: layer}}
		if full, fluid := tables.FullHeight[id]; fluid {
			local.fullHeight, err = tables.Shape(full)
			if err != nil {
				return nil, false, err
			}
		}
		parts = append(parts, local)
	}
	return parts, false, nil
}

func (m *Mesher[B]) prepareNeighborhood(
	layerBlocks [][]B,
	boundaries *meshing.Boundaries,
	missingShape *blockgeometry.Shape,
) (meshing.Cells, []MissingCell, error) {
	// 先取得原始几何，再读邻居，避免遍历顺序影响流体选择。
	locals := make([][]localGeometry, m.layout.Len())
	var missing []MissingCell
	for layer, blocks := range layerBlocks {
		for i, id := range blocks {
			parts, absent, err := queryGeometry(m.tables, id, layer, missingShape)
			if err != nil {
				p := m.layout.Position(i)
				return nil, nil, fmt.Errorf("layer %d, cell (%d,%d,%d), ID %d: %w", layer, p[0], p[1], p[2], id, err)
			}
			if absent {
				p := m.layout.Position(i)
				missing = append(missing, MissingCell{Pos: [3]uint8{p[0], p[1], p[2]}, Layer: layer})
			}
			locals[i] = append(locals[i], parts...)
		}
	}
	cells := make(meshing.Cells, m.layout.Len())
	for i, parts := range locals {
		for _, local := range parts {
			part := local.LayerShape
			if local.fullHeight != nil && int(m.layout.Position(i)[1]) < m.layout.Side()-1 {
				for _, above := range locals[i+1] {
					if above.fullHeight != nil && above.fullHeight.CullGroup == local.fullHeight.CullGroup {
						// 同格还可能有实体或植物，只替换这一份流体引用。
						part.Shape = local.fullHeight
						break
					}
				}
			}
			if local.fullHeight != nil && int(m.layout.Position(i)[1]) == m.layout.Side()-1 && boundaries != nil && boundaries[3] != nil {
				p := m.layout.Position(i)
				for _, above := range boundaries[3][m.layout.BoundaryIndex(3, [3]int{int(p[0]), int(p[1]), int(p[2])})] {
					if above.CullGroup == local.fullHeight.CullGroup {
						part.Shape = local.fullHeight
						break
					}
				}
			}
			cells[i] = append(cells[i], part)
		}
	}
	return cells, missing, nil
}

func countGeometry(cells meshing.Cells) (int, int) {
	blocks := 0
	used := map[*geometry.Shape]struct{}{}
	for _, cell := range cells {
		visible := false
		for _, part := range cell {
			used[part.Shape] = struct{}{}
			visible = visible || len(part.Quads) != 0
		}
		if visible {
			blocks++
		}
	}
	return blocks, len(used)
}

func (m *Mesher[B]) boundaries(
	center [][]B,
	neighbours Neighbours[B],
	missing *blockgeometry.Shape,
) (*meshing.Boundaries, error) {
	result := &meshing.Boundaries{}
	for side, offset := range adjacentOffsets {
		layers := neighbours[offset]
		if len(layers) > 0 {
			result[side] = make([]meshing.Cell, m.layout.Side()*m.layout.Side())
		}
		for layer, values := range layers {
			for a := 0; a < m.layout.Side(); a++ {
				for b := 0; b < m.layout.Side(); b++ {
					edge := m.layout.Side() - 1
					if side%2 == 1 {
						edge = 0
					}
					p := [3]int{edge, b, a}
					if side/2 == 1 {
						p = [3]int{a, edge, b}
					} else if side/2 == 2 {
						p = [3]int{a, b, edge}
					}
					index := m.layout.Index(p[0], p[1], p[2])
					parts, _, err := queryGeometry(m.tables, values[index], layer, missing)
					if err != nil {
						return nil, err
					}
					for _, part := range parts {
						shape := part.LayerShape
						aboveLayers, aboveIndex := layers, index+1
						if p[1] == m.layout.Side()-1 {
							aboveOffset := offset
							aboveOffset[1]++
							aboveLayers = neighbours[aboveOffset]
							if aboveOffset == ([3]int{}) {
								aboveLayers = center
							}
							aboveIndex = index - (m.layout.Side() - 1)
						}
						if part.fullHeight != nil {
							for aboveLayer, aboveValues := range aboveLayers {
								above, _, err := queryGeometry(m.tables, aboveValues[aboveIndex], aboveLayer, missing)
								if err != nil {
									return nil, err
								}
								for _, candidate := range above {
									if candidate.fullHeight != nil && candidate.fullHeight.CullGroup == part.fullHeight.CullGroup {
										shape.Shape = part.fullHeight
									}
								}
							}
						}
						slot := m.layout.BoundaryIndex(side, p)
						result[side][slot] = append(result[side][slot], shape)
					}
				}
			}
		}
	}
	return result, nil
}
