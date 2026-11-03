package banner

import (
	"fmt"
	"image/color"
	nbt "neomega-nbt/presets/v1"
	"strings"
)

type layer struct {
	pattern string
	color   color.NRGBA
}

// Java DyeColor 的纹理染色值，顺序也是旧版 Java NBT 的 Color 编号（不同于 Bedrock）。
var dyes = [...]struct {
	name string
	rgb  uint32
}{
	{"white", 0xF9FFFE}, {"orange", 0xF9801D}, {"magenta", 0xC74EBD}, {"light_blue", 0x3AB3DA},
	{"yellow", 0xFED83D}, {"lime", 0x80C71F}, {"pink", 0xF38BAA}, {"gray", 0x474F52},
	{"light_gray", 0x9D9D97}, {"cyan", 0x169C9C}, {"purple", 0x8932B8}, {"blue", 0x3C44AA},
	{"brown", 0x835432}, {"green", 0x5E7C16}, {"red", 0xB02E26}, {"black", 0x1D1D21},
}

// 1.20.4 及以前的 Java 旗帜使用短图案码；新版使用带命名空间的图案 ID。
var legacyPatterns = map[string]string{
	"b": "base", "bl": "square_bottom_left", "br": "square_bottom_right", "tl": "square_top_left", "tr": "square_top_right",
	"bs": "stripe_bottom", "ts": "stripe_top", "ls": "stripe_left", "rs": "stripe_right", "cs": "stripe_center", "ms": "stripe_middle",
	"drs": "stripe_downright", "dls": "stripe_downleft", "ss": "small_stripes", "cr": "cross", "sc": "straight_cross",
	"bt": "triangle_bottom", "tt": "triangle_top", "bts": "triangles_bottom", "tts": "triangles_top",
	"ld": "diagonal_left", "rd": "diagonal_up_right", "lud": "diagonal_up_left", "rud": "diagonal_right",
	"mc": "circle", "mr": "rhombus", "vh": "half_vertical", "hh": "half_horizontal", "vhr": "half_vertical_right", "hhb": "half_horizontal_bottom",
	"bo": "border", "cbo": "curly_border", "gra": "gradient", "gru": "gradient_up", "bri": "bricks",
	"glb": "globe", "cre": "creeper", "sku": "skull", "flo": "flower", "moj": "mojang", "pig": "piglin",
}

func dyeColor(name string) (color.NRGBA, error) {
	for _, dye := range dyes {
		if dye.name == name {
			return color.NRGBA{uint8(dye.rgb >> 16), uint8(dye.rgb >> 8), uint8(dye.rgb), 255}, nil
		}
	}
	return color.NRGBA{}, fmt.Errorf("未知 Java 染料 %q", name)
}

// patterns 接受现代 patterns 列表与旧版 Patterns 列表；缺少 NBT 表示只有底色。
func patterns(root nbt.Compound, base string) ([]layer, error) {
	c, err := dyeColor(base)
	if err != nil {
		return nil, err
	}
	result := []layer{{"minecraft:base", c}}
	value, modern := root["patterns"]
	if !modern {
		value = root["Patterns"]
	}
	if value == nil {
		return result, nil
	}
	var items []any
	switch v := value.(type) {
	case []any:
		items = v
	case []map[string]any:
		for _, item := range v {
			items = append(items, item)
		}
	default:
		return nil, fmt.Errorf("旗帜图案必须是 compound 列表，实际 %T", value)
	}
	for i, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("旗帜图案 %d 不是 compound", i)
		}
		var pattern, colorName string
		if modern {
			pattern, _ = entry["pattern"].(string)
			colorName, _ = entry["color"].(string)
			if !strings.Contains(pattern, ":") {
				pattern = "minecraft:" + pattern
			}
		} else {
			code, _ := entry["Pattern"].(string)
			mapped, found := legacyPatterns[code]
			if !found {
				return nil, fmt.Errorf("未知旧版 Java 旗帜图案 %q", code)
			}
			pattern = "minecraft:" + mapped
			id, ok := entry["Color"].(int32)
			if !ok || id < 0 || int(id) >= len(dyes) {
				return nil, fmt.Errorf("旧版 Java 旗帜 Color 必须是 0..15 的 Int")
			}
			colorName = dyes[id].name
		}
		c, err := dyeColor(colorName)
		if err != nil {
			return nil, err
		}
		result = append(result, layer{pattern, c})
	}
	return result, nil
}
