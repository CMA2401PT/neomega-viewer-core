package topview

import (
	"image/color"
	"testing"

	"neomega-blocks/ir"
)

func TestRenderColumnUsesTopBlock(t *testing.T) {
	// 输入顺序不影响顶层选择，缺色状态使用调用方的回退色，空格保留透明。
	stone := ir.IRBlock("minecraft:stone []").Tidy()
	dirt := ir.IRBlock("minecraft:dirt []").Tidy()
	air := ir.IRBlock("minecraft:air []").Tidy()
	table := Table{stone.Hash(): 0x111111, dirt.Hash(): 0x222222}
	lower := make([]ir.IRBlock, 16*16*16)
	upper := make([]ir.IRBlock, 16*16*16)
	for i := range lower {
		lower[i] = air
		upper[i] = air
	}
	lower[(1<<8)|(3<<4)|2] = stone
	lower[(1<<8)|(3<<4)|1] = dirt
	upper[(1<<8)|(3<<4)|0] = ir.IRBlock(`minecraft:stone ["waterlogged"="true"]`).Tidy()
	column := table.RenderColumn([]Section{{Y: 0, Blocks: lower}, {Y: 1, Blocks: upper}}, 0xabcdef)
	pixel := 3*16 + 1
	if column.Pixel[pixel] != (color.NRGBA{R: 0x11, G: 0x11, B: 0x11, A: 255}) || column.Height[pixel] != 16 {
		t.Fatalf("top stone: pixel=%#06x height=%d", column.Pixel[pixel], column.Height[pixel])
	}
	if column.Pixel[0].A != 0 {
		t.Fatalf("empty column got %#06x", column.Pixel[0])
	}
	lower[(4<<8)|(5<<4)|0] = ir.IRBlock("minecraft:oak_sign []").Tidy()
	column = table.RenderColumn([]Section{{Y: -1, Blocks: lower}}, 0xabcdef)
	pixel = 5*16 + 4
	if column.Pixel[pixel] != (color.NRGBA{R: 0xab, G: 0xcd, B: 0xef, A: 255}) || column.Height[pixel] != -16 {
		t.Fatalf("fallback: pixel=%#06x height=%d", column.Pixel[pixel], column.Height[pixel])
	}
}

func TestShadeBrightensTheNorthwestSlope(t *testing.T) {
	// 东南侧抬高的地形应变亮，空白位置仍透明。
	pixels := make([]color.NRGBA, 9)
	for i := 1; i < len(pixels); i++ {
		pixels[i] = color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	}
	heights := []int32{
		0, 64, 64,
		64, 64, 70,
		64, 70, 64,
	}
	Shade(pixels, heights, 3)
	// 中心格的东、南邻居更高，坡向为正。空格子保持 0。
	if pixels[4] != (color.NRGBA{R: 192, G: 192, B: 192, A: 255}) {
		t.Fatalf("center pixel = %#06x, want 0xc0c0c0", pixels[4])
	}
	if pixels[0].A != 0 {
		t.Fatalf("empty pixel became %#06x", pixels[0])
	}
}

func TestShadePreservesBlackAndHandlesNarrowImages(t *testing.T) {
	// 降到纯黑只改变颜色，不改变透明度；单行、单列也没有图外邻居。
	for _, width := range []int{1, 2} {
		pixels := []color.NRGBA{{R: 1, G: 1, B: 1, A: 255}, {R: 1, G: 1, B: 1, A: 255}}
		Shade(pixels, []int32{-64, -64}, width)
		for _, pixel := range pixels {
			if pixel != (color.NRGBA{A: 255}) {
				t.Fatalf("width=%d: 阴影中的黑色应不透明，实际为 %v", width, pixel)
			}
		}
	}
	pixels := []color.NRGBA{{R: 128, G: 128, B: 128, A: 255}}
	Shade(pixels, []int32{64}, 1)
	if pixels[0] != (color.NRGBA{R: 128, G: 128, B: 128, A: 255}) {
		t.Fatalf("单像素图不应产生坡度：%v", pixels[0])
	}
}
