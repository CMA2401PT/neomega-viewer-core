package brickmap

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestVolumeRoundTripAndHierarchy(t *testing.T) {
	origin, size := [3]int{-24, -8, 40}, [3]int{32, 16, 64}
	color := func(p [3]int) uint16 {
		if (p[0]+p[1]+p[2])%7 == 0 {
			return 0
		}
		return uint16((p[0]*p[0]+p[1]*p[1]+p[2])%9 + 1)
	}
	var scratch [512]uint16
	v := Build(origin, size, make([]Material, 256), func(o [3]int) Brick {
		for z := range 8 {
			for y := range 8 {
				for x := range 8 {
					scratch[x+8*(y+8*z)] = color([3]int{o[0] + x, o[1] + y, o[2] + z})
				}
			}
		}
		return Brick{Cells: scratch[:]}
	})
	for z := range size[2] {
		for y := range size[1] {
			for x := range size[0] {
				p := [3]int{x + origin[0], y + origin[1], z + origin[2]}
				if v.At(p) != color(p) {
					t.Fatalf("%v 的颜色错误", p)
				}
			}
		}
	}
	if v.At([3]int{-25, 0, 40}) != 0 {
		t.Fatal("范围外不是空气")
	}
	if v.Levels[len(v.Levels)-1].Cells[0] != Mixed {
		t.Fatal("混合区域被错误合并")
	}
	var output bytes.Buffer
	if err := v.Write(&output); err != nil {
		t.Fatal(err)
	}
	data := output.Bytes()
	headerBytes := int(binary.LittleEndian.Uint32(data))
	var header struct {
		Format     string
		BrickBytes int
		Levels     [][3]int
	}
	if err := json.Unmarshal(data[4:4+headerBytes], &header); err != nil {
		t.Fatal(err)
	}
	if header.Format != "brickmap-2" || header.BrickBytes != len(v.Bricks)*2 || headerBytes%4 != 0 {
		t.Fatal("元数据错误")
	}
	offset := 4 + headerBytes + 256*28
	for _, level := range v.Levels {
		for _, c := range level.Cells {
			if binary.LittleEndian.Uint32(data[offset:]) != c {
				t.Fatal("目录编码错误")
			}
			offset += 4
		}
	}
	if len(data[offset:]) != len(v.Bricks)*2 {
		t.Fatal("砖池编码错误")
	}
}

func TestUniformRectangularVolume(t *testing.T) {
	for _, color := range []uint16{0, 1, 65535} {
		v := Build([3]int{}, [3]int{64, 8, 32}, make([]Material, 256), func([3]int) Brick { return Brick{MaterialID: color} })
		if len(v.Bricks)*2 != 0 {
			t.Fatal("同色区域不应展开")
		}
		for _, level := range v.Levels {
			for _, c := range level.Cells {
				if c != uint32(color) {
					t.Fatal("短轴目录合并错误")
				}
			}
		}
	}
}

func TestUniformCellsAreCollapsed(t *testing.T) {
	cells := make([]uint16, 512)
	for i := range cells {
		cells[i] = 8
	}
	v := Build([3]int{}, [3]int{8, 8, 8}, make([]Material, 256), func([3]int) Brick { return Brick{Cells: cells} })
	if len(v.Bricks)*2 != 0 || v.At([3]int{7, 7, 7}) != 8 {
		t.Fatal("展开的同色输入未合并")
	}
}

func TestAddSectionAxisOrder(t *testing.T) {
	builder := NewBuilder([3]int{-16, -16, -16}, [3]int{16, 16, 16}, nil)
	cells := make([]uint16, 4096)
	for x := range 16 {
		for z := range 16 {
			for y := range 16 {
				cells[x<<8|z<<4|y] = uint16(1 + x*256 + z*16 + y)
			}
		}
	}
	builder.AddSection([3]int{-16, -16, -16}, cells)
	volume := builder.Finish()
	clear(cells)
	for x := range 16 {
		for z := range 16 {
			for y := range 16 {
				if got, want := volume.At([3]int{x - 16, y - 16, z - 16}), uint16(1+x*256+z*16+y); got != want {
					t.Fatalf("%d,%d,%d: %d != %d", x, y, z, got, want)
				}
			}
		}
	}
}
