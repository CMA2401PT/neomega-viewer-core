package sign

import (
	"encoding/json"
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"unicode"
)

type style struct {
	color                                       color.NRGBA
	bold, italic, underline, strike, obfuscated bool
}

type letter struct {
	rune  rune
	style style
}

// parseLine 接受旧 Java 的 JSON 字符串和新 Java 的直接 NBT 组件，输出带样式的字符。
func parseLine(value any, inherited style) ([]letter, error) {
	if text, ok := value.(string); ok {
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, `"`) {
			if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
				return nil, err
			}
		}
	}
	var out []letter
	if err := appendComponent(&out, value, inherited); err != nil {
		return nil, err
	}
	// 随机字符按连续区段替换为一个省略号，不随时间变化，也不暴露原文字。
	var result []letter
	hidden := false
	for _, char := range out {
		if char.style.obfuscated {
			if hidden {
				continue
			}
			char.rune = '…'
		}
		hidden = char.style.obfuscated
		char.style.obfuscated = false
		result = append(result, char)
	}
	return result, nil
}

// appendComponent 展开父子样式；extra 继承当前组件，数组后续项继承首项样式。
func appendComponent(out *[]letter, value any, inherited style) error {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		appendSectionText(out, v, inherited)
		return nil
	case []string, []map[string]any:
		list, err := componentList(v)
		if err != nil {
			return err
		}
		return appendComponent(out, list, inherited)
	case []any:
		if len(v) == 0 {
			return nil
		}
		base := inherited
		if first, ok := v[0].(map[string]any); ok {
			var err error
			base, err = componentStyle(first, inherited)
			if err != nil {
				return err
			}
		}
		for _, child := range v {
			if err := appendComponent(out, child, base); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		current, err := componentStyle(v, inherited)
		if err != nil {
			return err
		}
		switch {
		case v["text"] != nil:
			text, ok := v["text"].(string)
			if !ok {
				return fmt.Errorf("text 必须是字符串")
			}
			appendSectionText(out, text, current)
		case v["translate"] != nil:
			key, ok := v["translate"].(string)
			if !ok {
				return fmt.Errorf("translate 必须是字符串")
			}
			if fallback, ok := v["fallback"].(string); ok {
				key = fallback
			}
			args, err := componentList(v["with"])
			if err != nil {
				return err
			}
			if err := appendTranslation(out, key, args, current); err != nil {
				return err
			}
		case v["keybind"] != nil:
			appendSectionText(out, fmt.Sprint(v["keybind"]), current)
		case v["score"] != nil:
			score, _ := v["score"].(map[string]any)
			if text, ok := score["value"].(string); ok {
				appendSectionText(out, text, current)
			} else {
				appendSectionText(out, "…", current)
			}
		case v["selector"] != nil || v["nbt"] != nil:
			// 存档渲染没有服务器命令上下文，不执行选择器或读取其他 NBT。
			appendSectionText(out, "…", current)
		}
		if extra := v["extra"]; extra != nil {
			list, err := componentList(extra)
			if err != nil {
				return err
			}
			for _, child := range list {
				if err := appendComponent(out, child, current); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("不支持的文本组件类型 %T", value)
	}
}

func componentStyle(component map[string]any, inherited style) (style, error) {
	current := inherited
	if raw, exists := component["color"]; exists {
		name, ok := raw.(string)
		if !ok {
			return current, fmt.Errorf("文字 color 必须是字符串")
		}
		rgb, found := textColors[name]
		if !found {
			if len(name) != 7 || name[0] != '#' {
				return current, fmt.Errorf("未知文字颜色 %q", name)
			}
			number, err := strconv.ParseUint(name[1:], 16, 24)
			if err != nil {
				return current, err
			}
			rgb = uint32(number)
		}
		current.color = rgba(rgb)
	}
	for name, target := range map[string]*bool{
		"bold": &current.bold, "italic": &current.italic, "underlined": &current.underline,
		"strikethrough": &current.strike, "obfuscated": &current.obfuscated,
	} {
		if value, exists := component[name]; exists {
			var err error
			*target, err = boolean(value)
			if err != nil {
				return current, fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	return current, nil
}

// boolean 兼容 JSON bool 和 NBT 的 Byte/Int；文本组件在 SNBT 中允许数字布尔值。
func boolean(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int8:
		return v != 0, nil
	case uint8:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int:
		return v != 0, nil
	default:
		return false, fmt.Errorf("期望布尔值，实际 %T", value)
	}
}

// appendSectionText 兼容 TellrawR 和 Bedrock 转换结果中的 § 格式码，采用 Java 的 m/n 删除线与下划线含义。
func appendSectionText(out *[]letter, text string, base style) {
	current := base
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '§' && i+1 < len(runes) {
			i++
			code := unicode.ToLower(runes[i])
			if index := strings.IndexRune("0123456789abcdef", code); index >= 0 {
				current = style{color: rgba(sectionColors[index])}
				continue
			}
			switch code {
			case 'r':
				current = base
			case 'k':
				current.obfuscated = true
			case 'l':
				current.bold = true
			case 'o':
				current.italic = true
			case 'n':
				current.underline = true
			case 'm':
				current.strike = true
			case '§':
				*out = append(*out, letter{'§', current})
			}
			continue
		}
		*out = append(*out, letter{ch, current})
	}
}

// appendTranslation 使用存档内的 fallback；没有语言表时保留翻译键，仍展开 %s 与 %1$s 参数。
func appendTranslation(out *[]letter, format string, args []any, current style) error {
	next := 0
	for len(format) > 0 {
		at := strings.IndexByte(format, '%')
		if at < 0 {
			appendSectionText(out, format, current)
			break
		}
		appendSectionText(out, format[:at], current)
		format = format[at+1:]
		if strings.HasPrefix(format, "%") {
			appendSectionText(out, "%", current)
			format = format[1:]
			continue
		}
		end := 0
		for end < len(format) && format[end] >= '0' && format[end] <= '9' {
			end++
		}
		index := next
		if end > 0 && end < len(format) && format[end] == '$' {
			n, _ := strconv.Atoi(format[:end])
			index = n - 1
			end++
		} else {
			end = 0
			next++
		}
		if end >= len(format) || format[end] != 's' {
			appendSectionText(out, "%", current)
			continue
		}
		format = format[end+1:]
		if index < 0 || index >= len(args) {
			appendSectionText(out, "…", current)
			continue
		}
		if err := appendComponent(out, args[index], current); err != nil {
			return err
		}
	}
	return nil
}

func rgba(rgb uint32) color.NRGBA {
	return color.NRGBA{uint8(rgb >> 16), uint8(rgb >> 8), uint8(rgb), 255}
}

var sectionColors = [...]uint32{0x000000, 0x0000AA, 0x00AA00, 0x00AAAA, 0xAA0000, 0xAA00AA, 0xFFAA00, 0xAAAAAA, 0x555555, 0x5555FF, 0x55FF55, 0x55FFFF, 0xFF5555, 0xFF55FF, 0xFFFF55, 0xFFFFFF}
var textColors = map[string]uint32{
	"black": 0x000000, "dark_blue": 0x0000AA, "dark_green": 0x00AA00, "dark_aqua": 0x00AAAA,
	"dark_red": 0xAA0000, "dark_purple": 0xAA00AA, "gold": 0xFFAA00, "gray": 0xAAAAAA,
	"dark_gray": 0x555555, "blue": 0x5555FF, "green": 0x55FF55, "aqua": 0x55FFFF,
	"red": 0xFF5555, "light_purple": 0xFF55FF, "yellow": 0xFFFF55, "white": 0xFFFFFF,
}

// componentList 统一 JSON 的通用列表与 NBT 解码器的带类型列表，供 messages、extra 和 with 共用。
func componentList(value any) ([]any, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []any:
		return v, nil
	case []string:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = item
		}
		return out, nil
	case []map[string]any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = item
		}
		return out, nil
	default:
		return nil, fmt.Errorf("期望文本组件列表，实际 %T", value)
	}
}
