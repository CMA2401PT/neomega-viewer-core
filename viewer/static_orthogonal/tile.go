package static_orthogonal

import (
	"image"
	"image/color"
	"math"
)

// Tile 只保存一个输出瓦片的颜色与逐像素深度，内存与地图尺寸无关。
// 一个 Tile 由一个协程独占；不同 Tile 可以并行绘制。
type Tile struct {
	Image *image.NRGBA
	depth []float64
	// 颜色贴图按固定槽位复用；与形状掩码组合，不随方块种类累计扩容。
	materials [256]struct {
		rgb   uint32
		faces [3][3]byte
	}
}

// NewTile 创建独立瓦片，rect 使用整张画布坐标。
func NewTile(rect image.Rectangle, background color.NRGBA) *Tile {
	t := &Tile{Image: image.NewNRGBA(rect), depth: make([]float64, rect.Dx()*rect.Dy())}
	t.Reset(rect, background)
	return t
}

// Reset 复用同尺寸瓦片的内存，并移动到下一个输出位置。
func (t *Tile) Reset(rect image.Rectangle, background color.NRGBA) {
	t.Image.Rect = rect
	for i := range t.depth {
		t.depth[i] = math.Inf(-1)
		p := t.Image.Pix[i*4:]
		p[0], p[1], p[2], p[3] = background.R, background.G, background.B, background.A
	}
}

// Draw 根据相机尺度绘制一个像素点或查预绘制的立方体贴图，并比较深度。
// 点模式只写一个原色像素；立方体模式共享形状并缓存三种面色。
func (a *Atlas) Draw(t *Tile, block Stamp) {
	if a.point {
		x, y := int(block.U>>4), int(block.V>>4)
		if x < t.Image.Rect.Min.X || x >= t.Image.Rect.Max.X || y < t.Image.Rect.Min.Y || y >= t.Image.Rect.Max.Y {
			return
		}
		i := (y-t.Image.Rect.Min.Y)*t.Image.Rect.Dx() + x - t.Image.Rect.Min.X
		if block.Depth < t.depth[i] {
			return
		}
		pixel := t.Image.Pix[i*4:]
		rgb := block.RGB & 0xffffff
		oldRGB := uint32(pixel[0])<<16 | uint32(pixel[1])<<8 | uint32(pixel[2])
		if block.Depth == t.depth[i] && rgb <= oldRGB {
			return
		}
		t.depth[i] = block.Depth
		pixel[0], pixel[1], pixel[2], pixel[3] = byte(rgb>>16), byte(rgb>>8), byte(rgb), 255
		return
	}
	s := &a.sprites[(block.V&15)*phases+(block.U&15)]
	origin := image.Pt(int(block.U>>4), int(block.V>>4))
	clip := s.rect.Add(origin).Intersect(t.Image.Rect)
	if clip.Empty() {
		return
	}
	rgb := block.RGB & 0xffffff
	material := &t.materials[(rgb*2654435761)>>24]
	if material.rgb != rgb {
		material.rgb = rgb
		for face, factor := range a.shade {
			material.faces[face] = [3]byte{byte(((rgb >> 16) & 255) * factor / 255), byte(((rgb >> 8) & 255) * factor / 255), byte((rgb & 255) * factor / 255)}
		}
	}
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		si := (y-origin.Y-s.rect.Min.Y)*s.rect.Dx() + clip.Min.X - origin.X - s.rect.Min.X
		ti := (y-t.Image.Rect.Min.Y)*t.Image.Rect.Dx() + clip.Min.X - t.Image.Rect.Min.X
		for x := clip.Min.X; x < clip.Max.X; x++ {
			p := s.pixels[si]
			si++
			if p.face >= 0 {
				depth := block.Depth + p.depth
				if depth < t.depth[ti] {
					ti++
					continue
				}
				rgb := material.faces[p.face]
				pixel := t.Image.Pix[ti*4:]
				// 共边射线可能命中两块的同一深度；固定颜色次序让并行输入可复现。
				newRGB := uint32(rgb[0])<<16 | uint32(rgb[1])<<8 | uint32(rgb[2])
				oldRGB := uint32(pixel[0])<<16 | uint32(pixel[1])<<8 | uint32(pixel[2])
				if depth > t.depth[ti] || depth == t.depth[ti] && newRGB > oldRGB {
					t.depth[ti] = depth
					pixel[0], pixel[1], pixel[2], pixel[3] = rgb[0], rgb[1], rgb[2], 255
				}
			}
			ti++
		}
	}
}
