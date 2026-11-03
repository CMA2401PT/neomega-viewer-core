package banner

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"reflect"
	"testing"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/mesher"
	resources "neomega-viewer-core/viewer/resources"
)

func TestPatternOrderAndAlpha(t *testing.T) {
	// 用一像素的真实 alpha 混合验证后层覆盖，不依赖旗帜模型或实际图片内容。
	imageOf := func(c color.NRGBA) *image.NRGBA {
		p := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		p.SetNRGBA(0, 0, c)
		return p
	}
	h := Handler{textures: map[string]*image.NRGBA{
		"minecraft:base": imageOf(color.NRGBA{255, 255, 255, 255}),
		"half":           imageOf(color.NRGBA{255, 255, 255, 128}),
	}}
	red, blue := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}
	out, err := h.compose([]layer{{"minecraft:base", red}, {"half", blue}})
	if err != nil || out.NRGBAAt(0, 0) != (color.NRGBA{127, 0, 128, 255}) {
		t.Fatalf("半透明叠色错误: %v %v", out, err)
	}
	reversed, err := h.compose([]layer{{"minecraft:base", blue}, {"half", red}})
	if err != nil || reversed.NRGBAAt(0, 0) != (color.NRGBA{128, 0, 127, 255}) {
		t.Fatal("没有保留图案顺序", err)
	}
}

func TestModernAndLegacyNBT(t *testing.T) {
	modern, err := patterns(nbt.Compound{"patterns": []any{map[string]any{"pattern": "minecraft:stripe_bottom", "color": "red"}}}, "white")
	if err != nil {
		t.Fatal(err)
	}
	old, err := patterns(nbt.Compound{"Patterns": []any{map[string]any{"Pattern": "bs", "Color": int32(14)}}}, "white")
	if err != nil || !reflect.DeepEqual(modern, old) {
		t.Fatal("新旧 Java NBT 不一致", err)
	}
	for _, root := range []nbt.Compound{
		{"patterns": []any{map[string]any{"pattern": "minecraft:base", "color": "unknown"}}},
		{"Patterns": []any{map[string]any{"Pattern": "?", "Color": int32(0)}}},
		{"Patterns": []any{map[string]any{"Pattern": "bs", "Color": int32(16)}}},
	} {
		if _, err := patterns(root, "white"); err == nil {
			t.Fatal("无效图案没有返回诊断错误")
		}
	}
	if p, err := patterns(nil, "blue"); err != nil || len(p) != 1 {
		t.Fatal("无 NBT 应保留底色", err)
	}
}

func TestRealModelsAllColorsAndOrientations(t *testing.T) {
	preset, err := resources.Open(resources.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := New(preset.Tables)
	if err != nil {
		t.Fatal(err)
	}
	models := mesher.NewMesher(cube.New(1), preset.Tables)
	dispatcher := blockactor.New(handler)
	var previous []byte
	for _, dye := range dyes {
		for rotation := 0; rotation < 16; rotation++ {
			state := ir.IRBlock(fmt.Sprintf(`minecraft:%s_banner ["rotation"="%d"]`, dye.name, rotation))
			input := blockactor.Input{State: state, Pos: [3]uint8{3, 2, 7}, NBT: nbt.Compound{"patterns": []any{
				map[string]any{"pattern": "minecraft:stripe_bottom", "color": "blue"},
			}}}
			output, err := handler.Build(input)
			if err != nil {
				t.Fatal(state, err)
			}
			static, err := models.MeshBlock(state.Hash())
			if err != nil {
				t.Fatal(err)
			}
			changed := 0
			for i, q := range output.Mesh.Quads {
				if q.Positions != static.Quads[i].Positions || q.UV != static.Quads[i].UV {
					t.Fatal("状态旋转或 UV 被改动", state)
				}
				if q.Surface.Texture == "banner:face" {
					changed++
				}
			}
			if changed != 6 || len(output.Textures) != 2 {
				t.Fatal("应只替换旗面的六个面", state, changed)
			}
			tex := output.Textures[0]
			if tex.Width != 64 || tex.Height != 64 || len(tex.Pixels) != 64*64*4 {
				t.Fatal("未保留原尺寸图案")
			}
			if rotation == 0 {
				if bytes.Equal(previous, tex.Pixels) {
					t.Fatal("不同底色产生同一贴图")
				}
				previous = tex.Pixels
			}
			placed, err := dispatcher.BuildOne(input)
			if err != nil {
				t.Fatal(err)
			}
			for i, q := range placed.Mesh.Quads {
				for j, p := range q.Positions {
					for axis, offset := range input.Pos {
						if p[axis] != output.Mesh.Quads[i].Positions[j][axis]+float64(offset) {
							t.Fatal("actor 平移不是恰好一次")
						}
					}
				}
			}
		}
		for _, facing := range []string{"north", "east", "south", "west"} {
			state := ir.IRBlock(fmt.Sprintf(`minecraft:%s_wall_banner ["facing"="%s"]`, dye.name, facing))
			output, err := handler.Build(blockactor.Input{State: state})
			if err != nil {
				t.Fatal(state, err)
			}
			static, err := models.MeshBlock(state.Hash())
			if err != nil {
				t.Fatal(err)
			}
			for i, q := range output.Mesh.Quads {
				if q.Positions != static.Quads[i].Positions || q.UV != static.Quads[i].UV {
					t.Fatal("挂墙朝向被改动", state)
				}
			}
		}
	}
	for name := range handler.textures {
		if name == "minecraft:banner_base" {
			continue
		}
		if name == "minecraft:banner_base" {
			continue
		}
		_, err := handler.Build(blockactor.Input{State: ir.IRBlock(`minecraft:white_banner ["rotation"="0"]`), NBT: nbt.Compound{"patterns": []any{map[string]any{"pattern": name, "color": "red"}}}})
		if err != nil {
			t.Fatal("内置图案不能合成", name, err)
		}
	}
}
