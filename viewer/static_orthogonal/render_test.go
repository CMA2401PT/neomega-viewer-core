package static_orthogonal

import (
	"bytes"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
)

// 用跨负坐标、堆叠与交错方块验证输入顺序和瓦片边界不改变结果。
func TestOrderAndTileIndependence(t *testing.T) {
	for _, camera := range []Camera{{Yaw: 45, Pitch: 35.264, Scale: 4}, {Yaw: 123, Pitch: 23, Scale: 7}, {Yaw: 0, Pitch: 90, Scale: 3}, {Yaw: 45, Pitch: 35.264, Scale: 1}} {
		atlas, err := NewAtlas(camera)
		if err != nil {
			t.Fatal(err)
		}
		var stamps []Stamp
		for x := int32(-6); x < 6; x++ {
			for z := int32(-6); z < 6; z++ {
				for y := int32(-2); y < (x*x+z*z)%5; y++ {
					stamps = append(stamps, atlas.Project(x, y, z, uint32((x+6)*17)<<16|uint32((z+6)*17)<<8|uint32(y+3)*40))
				}
			}
		}
		var bounds image.Rectangle
		for _, s := range stamps {
			bounds = bounds.Union(atlas.Bounds(s))
		}
		whole := NewTile(bounds, color.NRGBA{})
		for _, s := range stamps {
			atlas.Draw(whole, s)
		}
		rand.New(rand.NewPCG(1, 2)).Shuffle(len(stamps), func(i, j int) { stamps[i], stamps[j] = stamps[j], stamps[i] })
		shuffled := NewTile(bounds, color.NRGBA{})
		for _, s := range stamps {
			atlas.Draw(shuffled, s)
		}
		if !bytes.Equal(whole.Image.Pix, shuffled.Image.Pix) {
			t.Fatalf("输入顺序改变遮挡：%+v", camera)
		}
		joined := image.NewNRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y += 17 {
			for x := bounds.Min.X; x < bounds.Max.X; x += 19 {
				rect := image.Rect(x, y, min(x+19, bounds.Max.X), min(y+17, bounds.Max.Y))
				tile := NewTile(rect, color.NRGBA{})
				for _, s := range stamps {
					atlas.Draw(tile, s)
				}
				for row := rect.Min.Y; row < rect.Max.Y; row++ {
					dst := joined.PixOffset(rect.Min.X, row)
					src := tile.Image.PixOffset(rect.Min.X, row)
					copy(joined.Pix[dst:dst+rect.Dx()*4], tile.Image.Pix[src:src+rect.Dx()*4])
				}
			}
		}
		if !bytes.Equal(whole.Image.Pix, joined.Pix) {
			t.Fatalf("瓦片边界改变图片：%+v", camera)
		}
	}
}

// 尺度为 1 时一块只写一个原色像素；更近的点覆盖更远的点。
func TestSinglePixelBlock(t *testing.T) {
	atlas, err := NewAtlas(Camera{Yaw: 0, Pitch: 90, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range atlas.sprites {
		if len(s.pixels) != 0 {
			t.Fatal("点模式仍生成了立方体贴图")
		}
	}
	tile := NewTile(image.Rect(-4, -4, 4, 4), color.NRGBA{})
	front := atlas.Project(-2, 8, -3, 0x123456)
	if atlas.Bounds(front).Dx() != 1 || atlas.Bounds(front).Dy() != 1 {
		t.Fatal("方块不是一个像素")
	}
	atlas.Draw(tile, front)
	atlas.Draw(tile, atlas.Project(-2, -8, -3, 0xff0000))
	count := 0
	for y := -4; y < 4; y++ {
		for x := -4; x < 4; x++ {
			p := tile.Image.NRGBAAt(x, y)
			if p.A != 0 {
				count++
				if p != (color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 255}) {
					t.Fatal(p)
				}
			}
		}
	}
	if count != 1 {
		t.Fatalf("写入了 %d 个像素", count)
	}
}

// 正俯视时一根柱子只能显示最高方块，黑色也必须保持不透明。
func TestTopViewOcclusion(t *testing.T) {
	atlas, err := NewAtlas(Camera{Yaw: 0, Pitch: 90, Scale: 4})
	if err != nil {
		t.Fatal(err)
	}
	tile := NewTile(image.Rect(-4, -4, 8, 8), color.NRGBA{})
	atlas.Draw(tile, atlas.Project(0, 9, 0, 0))
	atlas.Draw(tile, atlas.Project(0, -3, 0, 0xff0000))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if got := tile.Image.NRGBAAt(x, y); got != (color.NRGBA{A: 255}) {
				t.Fatalf("(%d,%d)=%v", x, y, got)
			}
		}
	}
}

// 封闭大立方体的投影内部必须没有背景孔洞。
func TestSolidCubeHasNoCracks(t *testing.T) {
	atlas, err := NewAtlas(Camera{Yaw: 45, Pitch: 35.264, Scale: 8})
	if err != nil {
		t.Fatal(err)
	}
	tile := NewTile(image.Rect(-100, -100, 100, 100), color.NRGBA{})
	for x := int32(-3); x < 3; x++ {
		for y := int32(-3); y < 3; y++ {
			for z := int32(-3); z < 3; z++ {
				atlas.Draw(tile, atlas.Project(x, y, z, 0xffffff))
			}
		}
	}
	for y := -60; y < 60; y++ {
		first, last := 100, -100
		for x := -99; x < 99; x++ {
			if tile.Image.NRGBAAt(x, y).A != 0 {
				first = min(first, x)
				last = max(last, x)
			}
		}
		for x := first; x <= last; x++ {
			if tile.Image.NRGBAAt(x, y).A == 0 {
				t.Fatalf("立方体内部出现接缝 (%d,%d)", x, y)
			}
		}
	}
}

func BenchmarkStamp(b *testing.B) {
	atlas, err := NewAtlas(Camera{Yaw: 45, Pitch: 35.264, Scale: 4})
	if err != nil {
		b.Fatal(err)
	}
	tile := NewTile(image.Rect(-256, -256, 256, 256), color.NRGBA{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		atlas.Draw(tile, atlas.Project(int32(i%80), int32(i/6400%32), int32(i/80%80), 0x529a34))
	}
}
