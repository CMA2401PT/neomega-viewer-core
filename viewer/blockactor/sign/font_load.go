package sign

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"image/color"
	"image/png"
	"math/bits"
	"path"
	"strconv"
	"strings"
)

// glyph 的尺寸、字距和偏移统一为半个 Java 字体单位，使位图字形与 Unifont 共用排版器。
type glyph struct {
	width, height, top, advance int
	boldOffset, shadowOffset    int
	pixels                      []color.NRGBA
	rows                        [16]uint32
}

// hexGlyph 保留 16 行位掩码，避免为十余万个 Unicode 字符各分配一张 RGBA 图片。
type hexGlyph struct {
	rows  [16]uint32
	width int
}

type font struct {
	bitmap  map[rune]glyph
	unicode map[rune]hexGlyph
}

// loadFont 使用 Java 默认字体的提供器顺序：空格、位图、Unifont；构造后全部只读。
func loadFont() (*font, error) {
	f := &font{bitmap: map[rune]glyph{' ': {advance: 8, boldOffset: 2, shadowOffset: 2}, '\u200c': {}}, unicode: make(map[rune]hexGlyph)}
	if err := f.loadBitmaps(); err != nil {
		return nil, err
	}
	if err := f.loadUnicode(); err != nil {
		return nil, err
	}
	return f, nil
}

// loadBitmaps 根据原版字符表切字，保留左侧空白；字距只去除右侧透明列。
func (f *font) loadBitmaps() error {
	data, err := files.ReadFile("font/bitmap.json")
	if err != nil {
		return err
	}
	var definition struct {
		Providers []struct {
			File           string
			Height, Ascent int
			Chars          []string
		}
	}
	if err := json.Unmarshal(data, &definition); err != nil {
		return err
	}
	for _, provider := range definition.Providers {
		data, err := files.ReadFile("font/" + path.Base(provider.File))
		if err != nil {
			return err
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return err
		}
		if provider.Height == 0 {
			provider.Height = 8 // bitmap 提供器未指定 height 时使用 8。
		}
		cw, ch := img.Bounds().Dx()/len([]rune(provider.Chars[0])), img.Bounds().Dy()/len(provider.Chars)
		scale := 2 * provider.Height / ch
		for row, chars := range provider.Chars {
			for column, r := range []rune(chars) {
				if _, exists := f.bitmap[r]; r == 0 || exists {
					continue
				}
				g := glyph{width: cw * scale, height: ch * scale, top: 2 * (7 - provider.Ascent), boldOffset: 2, shadowOffset: 2}
				g.pixels = make([]color.NRGBA, g.width*g.height)
				inkWidth := 0
				for y := 0; y < g.height; y++ {
					for x := 0; x < g.width; x++ {
						p := color.NRGBAModel.Convert(img.At(column*cw+x/scale, row*ch+y/scale)).(color.NRGBA)
						g.pixels[y*g.width+x] = p
						if p.A != 0 {
							inkWidth = max(inkWidth, x+1)
						}
					}
				}
				g.advance = ((inkWidth+1)/2 + 1) * 2
				f.bitmap[r] = g
			}
		}
	}
	return nil
}

// loadUnicode 读取原版携带的 Unifont，按同一份宽度覆盖表保留中日韩文字的字格。
func (f *font) loadUnicode() error {
	data, err := files.ReadFile("font/unifont.json")
	if err != nil {
		return err
	}
	var definition struct {
		Ranges []struct {
			From, To    string
			Left, Right int
		} `json:"size_overrides"`
	}
	if err := json.Unmarshal(data, &definition); err != nil {
		return err
	}
	data, err = files.ReadFile("font/unifont.zip")
	if err != nil {
		return err
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	file, err := archive.Open("unifont_all_no_pua-16.0.01.hex")
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		code, encoded, _ := strings.Cut(scanner.Text(), ":")
		number, err := strconv.ParseInt(code, 16, 32)
		if err != nil {
			return err
		}
		data, err := hex.DecodeString(encoded)
		if err != nil {
			return err
		}
		r := rune(number)
		var g hexGlyph
		columns := uint32(0)
		stride := len(data) / 16
		for y := range g.rows {
			for _, b := range data[y*stride : (y+1)*stride] {
				g.rows[y] = g.rows[y]<<8 | uint32(b)
			}
			columns |= g.rows[y]
		}
		left, right := bits.LeadingZeros32(columns)-(32-stride*8), stride*8-bits.TrailingZeros32(columns)-1
		if columns == 0 {
			left, right = 0, stride*8-1
		}
		for _, override := range definition.Ranges {
			if r >= []rune(override.From)[0] && r <= []rune(override.To)[0] {
				left, right = override.Left, override.Right
				break
			}
		}
		g.width = right - left + 1
		for y := range g.rows {
			// 覆盖表可把原本 8 列的字形扩为 16 列，右侧补白而不是负移位。
			g.rows[y] = (g.rows[y] << (32 - stride*8)) >> (31 - right)
		}
		f.unicode[r] = g
	}
	return scanner.Err()
}

func (f *font) glyph(r rune) glyph {
	if g, exists := f.bitmap[r]; exists {
		return g
	}
	if g, exists := f.unicode[r]; exists {
		return glyph{width: g.width, height: 16, advance: (g.width/2 + 1) * 2, boldOffset: 1, shadowOffset: 1, rows: g.rows}
	}
	// 未收录的码点画空心方框，不能把非 BMP 字符截成两个 UTF-16 字符。
	g := glyph{width: 10, height: 16, advance: 12, boldOffset: 2, shadowOffset: 2}
	for y := range g.rows {
		g.rows[y] = 0b1000000001
		if y == 0 || y == 15 {
			g.rows[y] = 0b1111111111
		}
	}
	return g
}

// pixel 返回字形原像素；位图保留彩色符号，Unifont 从一位掩码展开为白色。
func (g glyph) pixel(x, y int) color.NRGBA {
	if g.pixels != nil {
		return g.pixels[y*g.width+x]
	}
	if g.rows[y]&(1<<uint(g.width-1-x)) != 0 {
		return color.NRGBA{255, 255, 255, 255}
	}
	return color.NRGBA{}
}
