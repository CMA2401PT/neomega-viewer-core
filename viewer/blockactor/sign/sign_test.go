package sign

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockactor"
	resources "neomega-viewer-core/viewer/resources"
	"neomega-viewer-core/viewer/splitlayer"
)

func TestComponentsAndLegacyFormatting(t *testing.T) {
	base := style{color: rgba(0x123456)}
	line, err := parseLine(`{"text":"A","bold":true,"color":"red","extra":[{"text":"B","bold":false},{"text":"§aC§rD"},{"text":"secret","obfuscated":true},{"text":"E"}]}`, base)
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, char := range line {
		text.WriteRune(char.rune)
	}
	if text.String() != "ABCD…E" {
		t.Fatal(text.String())
	}
	if !line[0].style.bold || line[1].style.bold || line[2].style.bold || !line[3].style.bold {
		t.Fatal("显式 false、颜色重置或 §r 继承错误", line)
	}
	if line[2].style.color != rgba(0x55FF55) || line[3].style.color != rgba(0xFF5555) {
		t.Fatal("格式码颜色错误")
	}
	component := map[string]any{"text": "你好", "italic": int8(1), "extra": []any{map[string]any{"text": "界", "italic": int8(0)}}}
	line, err = parseLine(component, base)
	if err != nil || !line[0].style.italic || line[2].style.italic {
		t.Fatal("直接 NBT 组件没有继承/覆盖样式", err)
	}
	line, err = parseLine(`{"translate":"unused","fallback":"%2$s / %1$s %%","with":["甲","乙"]}`, base)
	if err != nil {
		t.Fatal(err)
	}
	text.Reset()
	for _, char := range line {
		text.WriteRune(char.rune)
	}
	if text.String() != "乙 / 甲 %" {
		t.Fatal(text.String())
	}
}

func TestSignNBTForms(t *testing.T) {
	// 真实 NBT 解码会产生 []string、[]Compound，不能只用手写 []any 验证组件递归。
	stored := nbt.Compound{"front_text": nbt.Compound{"messages": []nbt.Compound{
		{"text": "甲", "extra": []nbt.Compound{{"text": "乙", "bold": int8(1)}}},
		{"translate": "%s", "with": []string{"丙"}},
	}}}
	data, err := nbt.EncodeToNBTBytes(stored, nbt.OptEncodeToNBT{Encoding: nbt.EncodingBigEndian})
	if err != nil {
		t.Fatal(err)
	}
	var decoded nbt.Compound
	if err := nbt.DecodeFromNBTBytes(data, &decoded, nbt.OptDecodeFromNBT{Encoding: nbt.EncodingBigEndian}); err != nil {
		t.Fatal(err)
	}
	parsed, err := signSides(decoded)
	if err != nil || len(parsed[0].lines[0]) != 2 || !parsed[0].lines[0][1].style.bold || parsed[0].lines[1][0].rune != '丙' {
		t.Fatal("真实 NBT 组件列表未展开", err)
	}
	modern, err := signSides(nbt.Compound{"front_text": map[string]any{"messages": []string{`{"text":"Hello"}`, "§l你好"}, "color": "orange", "has_glowing_text": int8(1)}})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := signSides(nbt.Compound{"Text1": `{"text":"Hello"}`, "Text2": "§l你好", "Color": "orange", "GlowingText": int8(1)})
	if err != nil || !reflect.DeepEqual(modern, legacy) {
		t.Fatal("旧版和双面 NBT 不一致", err)
	}
	if modern[0].color != rgba(0xFF681F) || modern[0].lines[0][0].style.color != modern[0].color {
		t.Fatal("没有使用染料的文字颜色")
	}
	plain, err := signSides(nbt.Compound{"front_text": map[string]any{"messages": []any{"A", map[string]any{"text": "B", "color": "#123456"}}, "color": "white"}})
	if err != nil || plain[0].lines[0][0].style.color != rgba(0x666666) || plain[0].lines[1][0].style.color != rgba(0x123456) {
		t.Fatal("普通染料应压暗，显式组件颜色应保留", err)
	}
	for _, bad := range []nbt.Compound{{"front_text": 12}, {"front_text": map[string]any{"color": "bad"}}, {"Text1": `{"text":`}, {"Text1": `{"text":"x","bold":"true"}`}} {
		if _, err := signSides(bad); err == nil {
			t.Fatal("无效 NBT 未报告错误", bad)
		}
	}
}

func TestJavaFontAndGlow(t *testing.T) {
	f, err := loadFont()
	if err != nil {
		t.Fatal(err)
	}
	// Java 位图中 A 为 5 列墨迹加 1 单位间距；Unifont 中文为 16 像素加 1 单位间距。
	for r, want := range map[rune]int{'A': 12, 'i': 4, ' ': 8, '中': 18, '\u200c': 0} {
		if got := f.glyph(r).advance; got != want {
			t.Fatalf("%q 字距 %d，期望 %d", r, got, want)
		}
	}
	if _, ok := f.unicode['𠀀']; !ok {
		t.Fatal("缺少非 BMP 的 Unifont 字形")
	}
	if f.glyph('é').top >= 0 || f.glyph('中').boldOffset != 1 || f.glyph('A').boldOffset != 2 {
		t.Fatal("重音基线或不同字体的粗体偏移丢失")
	}
	line, _ := parseLine("hello world", style{color: rgba(0xFFFFFF)})
	short, width := f.firstLine(line, 70)
	if len(short) != 5 || width > 70 {
		t.Fatal("没有在空格处选取首行", len(short), width)
	}
	line, _ = parseLine(`{"text":"A\n","extra":[{"text":"B"}]}`, style{color: rgba(0xFFFFFF)})
	short, _ = f.firstLine(line, 180)
	if len(short) != 1 {
		t.Fatal("换行后的 extra 泄漏到首行")
	}
	side := sideText{color: rgba(0)}
	side.lines[0], _ = parseLine("A", style{color: rgba(0)})
	plain, visible := f.rasterize(side, false)
	if !visible {
		t.Fatal("黑字被当成透明")
	}
	side.glow = true
	glow, _ := f.rasterize(side, false)
	black, edge, expanded := 0, 0, 0
	for y := 0; y < glow.Rect.Dy(); y++ {
		for x := 0; x < glow.Rect.Dx(); x++ {
			p := glow.NRGBAAt(x, y)
			if p == rgba(0) {
				black++
			}
			if p == rgba(0xF0EBCC) {
				edge++
			}
			if p.A != 0 && plain.NRGBAAt(x, y).A == 0 {
				expanded++
			}
		}
	}
	if black == 0 || edge == 0 || expanded == 0 {
		t.Fatal("黑色发光必须同时保留黑字和浅色轮廓", black, edge, expanded)
	}
	if img, visible := f.rasterize(sideText{}, true); visible || img.Rect.Dx() != 128 || img.Rect.Dy() != 96 {
		t.Fatal("空牌或悬挂牌尺寸错误")
	}
	// 空格上的下划线也需要像素，不能依赖字形本身有墨迹。
	side = sideText{}
	side.lines[0] = []letter{{' ', style{color: color.NRGBA{255, 0, 0, 255}, underline: true}}}
	if _, visible := f.rasterize(side, false); !visible {
		t.Fatal("空格下划线没有绘制")
	}
}

func TestGlowUsesPerGlyphOffsetAndExcludesDecorations(t *testing.T) {
	// 单像素字形让两种偏移的范围可以直接检查：中文必须连成 3×3，不能留下跳格的碎线。
	f := &font{bitmap: map[rune]glyph{
		'A': {width: 1, height: 1, advance: 2, boldOffset: 2, shadowOffset: 2, rows: [16]uint32{1}},
		'中': {width: 1, height: 1, advance: 2, boldOffset: 1, shadowOffset: 1, rows: [16]uint32{1}},
		' ': {advance: 8},
	}}
	for _, tc := range []struct {
		r      rune
		offset int
	}{{'A', 2}, {'中', 1}} {
		side := sideText{color: rgba(0), glow: true}
		side.lines[0] = []letter{{tc.r, style{color: rgba(0)}}}
		img, _ := f.rasterize(side, false)
		for y := 5; y <= 11; y++ {
			for x := 92; x <= 98; x++ {
				want := color.NRGBA{}
				if (x == 95-tc.offset || x == 95 || x == 95+tc.offset) && (y == 8-tc.offset || y == 8 || y == 8+tc.offset) {
					want = rgba(0xF0EBCC)
				}
				if x == 95 && y == 8 {
					want = rgba(0)
				}
				if got := img.NRGBAAt(x, y); got != want {
					t.Fatalf("%q (%d,%d): %v，期望 %v", tc.r, x, y, got, want)
				}
			}
		}
	}
	side := sideText{color: rgba(0)}
	side.lines[0] = []letter{{' ', style{color: rgba(0), underline: true, strike: true}}}
	plain, _ := f.rasterize(side, false)
	side.glow = true
	glow, _ := f.rasterize(side, false)
	if !bytes.Equal(plain.Pix, glow.Pix) {
		t.Fatal("下划线或删除线被错误描边")
	}
}

func TestAllSignModelsAndOrientations(t *testing.T) {
	preset, err := resources.Open(resources.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(preset.Tables)
	if err != nil {
		t.Fatal(err)
	}
	root := nbt.Compound{
		"front_text": map[string]any{"messages": []string{"FRONT 中文"}},
		"back_text":  map[string]any{"messages": []string{"BACK 背面"}, "has_glowing_text": int8(1)},
	}
	for _, name := range h.BlockNames() {
		var states []ir.IRBlock
		if strings.Contains(name, "_wall_") {
			for _, facing := range []string{"north", "east", "south", "west"} {
				states = append(states, ir.IRBlock(fmt.Sprintf(`%s ["facing"="%s","waterlogged"="false"]`, name, facing)).Tidy())
			}
		} else {
			for rotation := 0; rotation < 16; rotation++ {
				if strings.HasSuffix(name, "_hanging_sign") {
					for _, attached := range []string{"false", "true"} {
						states = append(states, ir.IRBlock(fmt.Sprintf(`%s ["attached"="%s","rotation"="%d","waterlogged"="false"]`, name, attached, rotation)).Tidy())
					}
				} else {
					states = append(states, ir.IRBlock(fmt.Sprintf(`%s ["rotation"="%d","waterlogged"="false"]`, name, rotation)).Tidy())
				}
			}
		}
		for _, state := range states {
			out, err := h.Build(blockactor.Input{State: state, NBT: root})
			if err != nil {
				t.Fatal(state, err)
			}
			body, err := h.models.MeshBlock(splitlayer.SplitBlock(state)[0])
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Mesh.Quads) != len(body.Quads)+2 || len(out.Textures) != 3 {
				t.Fatal("需要牌体和两面独立文字", state, len(out.Mesh.Quads), len(out.Textures))
			}
			for i, q := range body.Quads {
				if q.Positions != out.Mesh.Quads[i].Positions || q.UV != out.Mesh.Quads[i].UV {
					t.Fatal("牌体形状或木纹发生变化", state)
				}
			}
			front, _ := frontNormal(state)
			for i, q := range out.Mesh.Quads[len(body.Quads):] {
				direction := 1.0
				if i == 1 {
					direction = -1
				}
				if q.Surface.Unlit != (i == 1) || math.Abs(q.Normal[0]-front[0]*direction) > 0.001 || math.Abs(q.Normal[2]-front[2]*direction) > 0.001 {
					t.Fatal("文字朝向或发光标记错误", state)
				}
				for a, p := range q.Positions {
					for b, other := range q.Positions {
						if q.UV[a][0] < q.UV[b][0] && (other[0]-p[0])*q.Normal[2]-(other[2]-p[2])*q.Normal[0] <= 0 {
							t.Fatal("背面文字左右镜像", state)
						}
						if q.UV[a][1] < q.UV[b][1] && p[1] <= other[1] {
							t.Fatal("文字上下颠倒", state)
						}
					}
				}
				if strings.HasSuffix(name, "_hanging_sign") {
					top, bottom := -math.MaxFloat64, math.MaxFloat64
					for _, p := range q.Positions {
						top = max(top, p[1])
						bottom = min(bottom, p[1])
					}
					if math.Abs(top-bottom-0.675) > 1e-6 || math.Abs((top+bottom)/2-0.305) > 1e-6 {
						t.Fatal("悬挂牌文字比例或中心错误", state, top, bottom)
					}
				}
			}
			if bytes.Equal(out.Textures[1].Pixels, out.Textures[2].Pixels) {
				t.Fatal("正反面文字被共用")
			}
		}
	}
}
