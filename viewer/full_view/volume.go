// Package full_view 用分层颜色体素支持可旋转的正交视图；不读取存档，也不生成场景网格。
package full_view

import (
	"encoding/binary"
	"encoding/json"
	"io"
)

const BrickSide = 8
const BrickCells = BrickSide * BrickSide * BrickSide
const Mixed uint32 = 0xffffffff

// Brick 用一个颜色表示同色实心块或空气；Cells 非 nil 时提供 512 个颜色编号。
// 编号 0 是空气，其余编号查 Palette。Cells 按 x + 8*(y + 8*z) 排列，可在回调间复用。
type Brick struct {
	Color byte
	Cells []byte
}

// Level 保存同一尺度的空间目录。0 是空气，1…255 是同色实心区域；
// 最细层的 256 以上数值指向砖池，上层的 Mixed 表示需要继续细分。
type Level struct {
	Size  [3]int
	Cells []uint32
}

// Volume 的坐标由目录下标恢复。只有混合小块占用 Bricks，每块固定 512 字节。
type Volume struct {
	Origin  [3]int
	Size    [3]int
	Palette [1024]byte
	Levels  []Level
	Bricks  []byte
}

// Builder 接收任意顺序的小块；未提交的位置保留为空气。
// 同一位置只提交一次，Finish 后由 Volume 接管数组。
type Builder struct{ volume *Volume }

// NewBuilder 分配最细层目录；origin 须按 8 格对齐，size 的每轴须为不小于 8 的二次幂。
func NewBuilder(origin, size [3]int, palette [1024]byte) *Builder {
	d := [3]int{size[0] / 8, size[1] / 8, size[2] / 8}
	return &Builder{volume: &Volume{Origin: origin, Size: size, Palette: palette, Levels: []Level{{Size: d, Cells: make([]uint32, d[0]*d[1]*d[2])}}}}
}

// AddBrick 复制一个范围内、按 8 格对齐的小块，来源可立即复用 Cells。
func (b *Builder) AddBrick(origin [3]int, brick Brick) {
	v := b.volume
	value := uint32(brick.Color)
	if brick.Cells != nil {
		value = uint32(brick.Cells[0])
		for _, c := range brick.Cells {
			if uint32(c) != value {
				value = uint32(len(v.Bricks)/BrickCells) + 256
				v.Bricks = append(v.Bricks, brick.Cells...)
				break
			}
		}
	}
	d := v.Levels[0].Size
	x, y, z := (origin[0]-v.Origin[0])/8, (origin[1]-v.Origin[1])/8, (origin[2]-v.Origin[2])/8
	v.Levels[0].Cells[x+d[0]*(y+d[1]*z)] = value
}

// Build 适用于按坐标生成数据的来源；已有存档可用 Builder 按读入顺序提交小块。
func Build(origin, size [3]int, palette [1024]byte, source func([3]int) Brick) *Volume {
	b := NewBuilder(origin, size, palette)
	for z := 0; z < size[2]; z += 8 {
		for y := 0; y < size[1]; y += 8 {
			for x := 0; x < size[0]; x += 8 {
				p := [3]int{origin[0] + x, origin[1] + y, origin[2] + z}
				b.AddBrick(p, source(p))
			}
		}
	}
	return b.Finish()
}

// Finish 自底向上合并同色区域，返回可直接序列化和上传的只读体积。
func (b *Builder) Finish() *Volume {
	v := b.volume
	d := v.Levels[0].Size
	for d != [3]int{1, 1, 1} {
		previous := v.Levels[len(v.Levels)-1]
		d = [3]int{max(1, d[0]/2), max(1, d[1]/2), max(1, d[2]/2)}
		level := Level{Size: d, Cells: make([]uint32, d[0]*d[1]*d[2])}
		for z := range d[2] {
			for y := range d[1] {
				for x := range d[0] {
					value := previous.Cells[min(x*2, previous.Size[0]-1)+previous.Size[0]*(min(y*2, previous.Size[1]-1)+previous.Size[1]*min(z*2, previous.Size[2]-1))]
					for dz := range min(2, previous.Size[2]) {
						for dy := range min(2, previous.Size[1]) {
							for dx := range min(2, previous.Size[0]) {
								child := previous.Cells[x*2+dx+previous.Size[0]*(y*2+dy+previous.Size[1]*(z*2+dz))]
								if child != value || child >= 256 {
									value = Mixed
								}
							}
						}
					}
					level.Cells[x+d[0]*(y+d[1]*z)] = value
				}
			}
		}
		v.Levels = append(v.Levels, level)
	}
	return v
}

// At 返回世界坐标处的颜色编号，范围外为空气；用于来源验证和参考射线测试。
func (v *Volume) At(p [3]int) byte {
	for i := range 3 {
		p[i] -= v.Origin[i]
		if p[i] < 0 || p[i] >= v.Size[i] {
			return 0
		}
	}
	d := v.Levels[0].Size
	b := v.Levels[0].Cells[p[0]/8+d[0]*(p[1]/8+d[1]*(p[2]/8))]
	if b < 256 {
		return byte(b)
	}
	return v.Bricks[int(b-256)*512+(p[0]&7)+8*((p[1]&7)+8*(p[2]&7))]
}

// Write 将元数据、调色板、逐层目录和砖池流式写出，避免为传输复制整个体积。
func (v *Volume) Write(w io.Writer) error {
	sizes := make([][3]int, len(v.Levels))
	for i, l := range v.Levels {
		sizes[i] = l.Size
	}
	header, err := json.Marshal(struct {
		Format     string   `json:"format"`
		Origin     [3]int   `json:"origin"`
		Size       [3]int   `json:"size"`
		Levels     [][3]int `json:"levels"`
		BrickBytes int      `json:"brickBytes"`
	}{"full-view-1", v.Origin, v.Size, sizes, len(v.Bricks)})
	if err != nil {
		return err
	}
	for len(header)%4 != 0 {
		header = append(header, ' ')
	}
	if err = binary.Write(w, binary.LittleEndian, uint32(len(header))); err != nil {
		return err
	}
	if _, err = w.Write(header); err != nil {
		return err
	}
	if _, err = w.Write(v.Palette[:]); err != nil {
		return err
	}
	// 每层分批编码，避免 binary.Write 为最大目录分配同样大的临时缓冲。
	buffer := make([]byte, 64*1024)
	for _, level := range v.Levels {
		for start := 0; start < len(level.Cells); start += len(buffer) / 4 {
			count := min(len(buffer)/4, len(level.Cells)-start)
			for i, c := range level.Cells[start : start+count] {
				binary.LittleEndian.PutUint32(buffer[i*4:], c)
			}
			if _, err = w.Write(buffer[:count*4]); err != nil {
				return err
			}
		}
	}
	_, err = w.Write(v.Bricks)
	return err
}
