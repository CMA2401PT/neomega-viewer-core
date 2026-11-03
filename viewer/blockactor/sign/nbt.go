package sign

import (
	"fmt"
	"image/color"

	nbt "neomega-nbt/presets/v1"
)

// sideText 保留一面的四行、染料和发光设置，排版器无需了解 NBT 字段。
type sideText struct {
	lines [4][]letter
	color color.NRGBA
	glow  bool
}

// signSides 将双面 NBT 或旧版 Text1…Text4 统一为正反两面；缺少文字就是空牌。
func signSides(root nbt.Compound) ([2]sideText, error) {
	var sides [2]sideText
	for i, key := range []string{"front_text", "back_text"} {
		data := map[string]any{}
		if value, exists := root[key]; exists {
			var ok bool
			data, ok = value.(map[string]any)
			if !ok {
				return sides, fmt.Errorf("%s 必须是 compound", key)
			}
		} else if i == 0 {
			data = map[string]any{
				"messages": []any{root["Text1"], root["Text2"], root["Text3"], root["Text4"]},
				"color":    root["Color"], "has_glowing_text": root["GlowingText"],
			}
		}
		dye := "black"
		if value := data["color"]; value != nil {
			var ok bool
			dye, ok = value.(string)
			if !ok {
				return sides, fmt.Errorf("%s.color 必须是字符串", key)
			}
		}
		rgb, found := dyeTextColors[dye]
		if !found {
			return sides, fmt.Errorf("未知告示牌染料 %q", dye)
		}
		side := sideText{color: rgba(rgb)}
		var err error
		if value := data["has_glowing_text"]; value != nil {
			side.glow, err = boolean(value)
			if err != nil {
				return sides, err
			}
		}
		messages, err := componentList(data["messages"])
		if err != nil {
			return sides, fmt.Errorf("%s.messages: %w", key, err)
		}
		baseColor := side.color
		if !side.glow {
			baseColor = darkColor(baseColor)
		}
		for j := 0; j < min(4, len(messages)); j++ {
			side.lines[j], err = parseLine(messages[j], style{color: baseColor})
			if err != nil {
				return sides, fmt.Errorf("%s 第 %d 行: %w", key, j+1, err)
			}
		}
		sides[i] = side
	}
	return sides, nil
}

// Java DyeColor.getTextColor，区别于旗帜使用的纹理染色值。
var dyeTextColors = map[string]uint32{
	"white": 16777215, "orange": 16738335, "magenta": 16711935, "light_blue": 10141901,
	"yellow": 16776960, "lime": 12582656, "pink": 16738740, "gray": 8421504,
	"light_gray": 13882323, "cyan": 65535, "purple": 10494192, "blue": 255,
	"brown": 9127187, "green": 65280, "red": 16711680, "black": 0,
}
