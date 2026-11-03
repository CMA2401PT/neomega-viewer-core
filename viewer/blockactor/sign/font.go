package sign

import (
	"image"
	"image/color"
	"math"
)

// placedGlyph 保存字形的行内位置；字距包含末尾间隔，与 Java Font.width 的居中依据一致。
type placedGlyph struct {
	glyph      glyph
	style      style
	x, advance int
}

// firstLine 按 Java 的 90/60 字体单位宽度取首行；优先在空格换行，无空格时在字符间截断。
func (f *font) firstLine(line []letter, maxWidth int) ([]placedGlyph, int) {
	var chars []placedGlyph
	length, lastSpace := 0, -1
	for _, char := range line {
		if char.rune == '\n' || char.rune == '\r' {
			break
		}
		g := f.glyph(char.rune)
		advance := g.advance
		if char.style.bold {
			advance += g.boldOffset
		}
		if char.rune == ' ' {
			lastSpace = len(chars)
		}
		if length+advance > maxWidth {
			if lastSpace >= 0 {
				chars = chars[:lastSpace]
				length = 0
				if len(chars) > 0 {
					last := chars[len(chars)-1]
					length = last.x + last.advance
				}
			}
			break
		}
		chars = append(chars, placedGlyph{glyph: g, style: char.style, x: length, advance: advance})
		length += advance
	}
	return chars, length
}

// rasterize 在半个 Java 字体单位的像素网格上绘制四行文字，牌面变换负责换算世界尺寸。
func (f *font) rasterize(side sideText, hanging bool) (*image.NRGBA, bool) {
	width, height, maxWidth, lineHeight := 192, 96, 180, 20
	if hanging {
		width, maxWidth, lineHeight = 128, 120, 18
	}
	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	var outline *image.NRGBA
	edge := darkColor(side.color)
	if side.glow {
		outline = image.NewNRGBA(out.Rect)
		if side.color == rgba(0) {
			edge = rgba(0xF0EBCC)
		}
	}
	visible := false
	for lineNumber, line := range side.lines {
		chars, length := f.firstLine(line, maxWidth)
		left, top := (width-length)/2, (height-4*lineHeight)/2+lineNumber*lineHeight
		for charIndex, char := range chars {
			g := char.glyph
			// 原版按字形的 shadowOffset 描边：位图为 1 单位，Unifont 为 0.5 单位。
			// 轮廓与正文分层绘制，避免后一个字的轮廓盖住前一个字，也不为装饰线描边。
			if outline != nil {
				borderStyle := char.style
				borderStyle.color = edge
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						g.draw(outline, left+char.x+dx*g.shadowOffset, top+dy*g.shadowOffset, borderStyle)
					}
				}
			}
			visible = g.draw(out, left+char.x, top, char.style) || visible

			// 装饰线按整个字距绘制，空格上的下划线也连续；位置分别为 Java 的 [3.5,4.5] 和 [8,9]。
			for _, decoration := range []struct {
				enabled bool
				y       int
			}{{char.style.strike, 7}, {char.style.underline, 16}} {
				if !decoration.enabled {
					continue
				}
				visible = true
				for y := decoration.y; y < decoration.y+2; y++ {
					start := 0
					if charIndex == 0 {
						start = -2
					}
					for x := start; x < char.advance; x++ {
						putPixel(out, left+char.x+x, top+y, char.style.color)
					}
				}
			}
		}
	}
	if outline != nil && visible {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				putPixel(outline, x, y, out.NRGBAAt(x, y))
			}
		}
		out = outline
	}
	return out, visible
}

// draw 将一个字形按自身基线、斜体与粗体规则画入目标图片，返回是否存在可见墨迹。
func (g glyph) draw(dst *image.NRGBA, left, top int, appearance style) bool {
	visible := false
	for y := 0; y < g.height; y++ {
		shift := 0
		// 原版斜体以行基线为基准倾斜 1/4，重音字符可延伸到行顶之上。
		if appearance.italic {
			shift = int(math.Round(2 - float64(y+g.top)/4))
		}
		for x := 0; x < g.width; x++ {
			pixel := g.pixel(x, y)
			if pixel.A == 0 {
				continue
			}
			pixel.R = uint8(uint16(pixel.R) * uint16(appearance.color.R) / 255)
			pixel.G = uint8(uint16(pixel.G) * uint16(appearance.color.G) / 255)
			pixel.B = uint8(uint16(pixel.B) * uint16(appearance.color.B) / 255)
			visible = true
			putPixel(dst, left+x+shift, top+g.top+y, pixel)
			if appearance.bold {
				putPixel(dst, left+x+shift+g.boldOffset, top+g.top+y, pixel)
			}
		}
	}
	return visible
}

// darkColor 与 Java 告示牌的普通染料文字、发光描边一致，保留 40% 的 RGB。
func darkColor(c color.NRGBA) color.NRGBA {
	return color.NRGBA{uint8(float64(c.R) * 0.4), uint8(float64(c.G) * 0.4), uint8(float64(c.B) * 0.4), c.A}
}

// putPixel 以非预乘 alpha 叠加，斜体、粗体和描边在画布边缘直接裁切。
func putPixel(dst *image.NRGBA, x, y int, src color.NRGBA) {
	if src.A == 0 || !image.Pt(x, y).In(dst.Rect) {
		return
	}
	old := dst.NRGBAAt(x, y)
	sa, da := uint32(src.A), uint32(old.A)
	a := sa*255 + da*(255-sa)
	blend := func(s, d uint8) uint8 { return uint8((uint32(s)*sa*255 + uint32(d)*da*(255-sa) + a/2) / a) }
	dst.SetNRGBA(x, y, color.NRGBA{blend(src.R, old.R), blend(src.G, old.G), blend(src.B, old.B), uint8((a + 127) / 255)})
}
