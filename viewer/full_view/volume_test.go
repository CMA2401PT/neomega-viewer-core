package full_view

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestVolumeRoundTripAndHierarchy(t *testing.T) {
	origin, size := [3]int{-24, -8, 40}, [3]int{32, 16, 64}
	color := func(p [3]int) byte {
		if (p[0]+p[1]+p[2])%7 == 0 {
			return 0
		}
		return byte((p[0]*p[0]+p[1]*p[1]+p[2])%9 + 1)
	}
	var scratch [512]byte
	v := Build(origin, size, [1024]byte{}, func(o [3]int) Brick {
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
	if header.Format != "full-view-1" || header.BrickBytes != len(v.Bricks) || headerBytes%4 != 0 {
		t.Fatal("元数据错误")
	}
	offset := 4 + headerBytes + 1024
	for _, level := range v.Levels {
		for _, c := range level.Cells {
			if binary.LittleEndian.Uint32(data[offset:]) != c {
				t.Fatal("目录编码错误")
			}
			offset += 4
		}
	}
	if !bytes.Equal(data[offset:], v.Bricks) {
		t.Fatal("砖池编码错误")
	}
}

func TestUniformRectangularVolume(t *testing.T) {
	for _, color := range []byte{0, 1, 255} {
		v := Build([3]int{}, [3]int{64, 8, 32}, [1024]byte{}, func([3]int) Brick { return Brick{Color: color} })
		if len(v.Bricks) != 0 {
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
	cells := bytes.Repeat([]byte{8}, 512)
	v := Build([3]int{}, [3]int{8, 8, 8}, [1024]byte{}, func([3]int) Brick { return Brick{Cells: cells} })
	if len(v.Bricks) != 0 || v.At([3]int{7, 7, 7}) != 8 {
		t.Fatal("展开的同色输入未合并")
	}
}
