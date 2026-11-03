// Package cube 定义一次构建所使用的 N³ 坐标约定，数据本身不携带尺寸。
package cube

// Layout 创建后只读，可由分发器和网格构建器共享。
// 局部坐标沿用渲染协议的 uint8，因此边长范围是 1 到 256。
type Layout struct{ side, area int }

// New 确定构建器的固定边长；不支持的边长会立即 panic。
func New(side int) Layout {
	if side < 1 || side > 256 {
		panic("cube: side must be in [1, 256]")
	}
	return Layout{side: side, area: side * side}
}

func (l Layout) Side() int { return l.side }
func (l Layout) Len() int  { return l.area * l.side }

// Index 使用 X-Z-Y 顺序，Y 连续；坐标必须处于本区域内。
func (l Layout) Index(x, y, z int) int { return x*l.area + z*l.side + y }

// Position 将连续索引还原为区域局部坐标。
func (l Layout) Position(index int) [3]uint8 {
	return [3]uint8{uint8(index / l.area), uint8(index % l.side), uint8(index / l.side % l.side)}
}

// BoundaryIndex 寻址六个邻接面，顺序为西、东、下、上、北、南。
func (l Layout) BoundaryIndex(side int, p [3]int) int {
	switch side / 2 {
	case 0:
		return p[2]*l.side + p[1]
	case 1:
		return p[0]*l.side + p[2]
	default:
		return p[0]*l.side + p[1]
	}
}
