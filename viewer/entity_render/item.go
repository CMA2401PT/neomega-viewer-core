package entity_render

import (
	"fmt"
	"strings"
)

// 非外观组件不阻止固定模型绘制；外观组件必须显式支持，不能静默画成默认物品。
// 附魔保留物品本体，当前静态管线不生成附魔光效。
func frameItemModel(id string, value any) (string, error) {
	if value == nil {
		return id, nil
	}
	components, ok := value.(map[string]any)
	if !ok {
		return "", fmt.Errorf("item frame Item.components must be a compound")
	}
	if _, damaged := components["minecraft:damage"]; damaged && id == "minecraft:elytra" {
		return "", fmt.Errorf("unsupported damage-dependent elytra model")
	}
	for name, value := range components {
		switch strings.TrimPrefix(name, "minecraft:") {
		case "item_model":
			model, ok := value.(string)
			if !ok || model == "" {
				return "", fmt.Errorf("item_model must be a resource ID string")
			}
			if !strings.Contains(model, ":") {
				model = "minecraft:" + model
			}
			id = model
		case "custom_name", "item_name", "lore", "damage", "max_damage", "unbreakable",
			"enchantments", "stored_enchantments", "enchantment_glint_override", "repair_cost",
			"attribute_modifiers", "custom_data", "tooltip_display", "tooltip_style",
			"hide_tooltip", "hide_additional_tooltip", "rarity", "max_stack_size":
			// 这些组件不改变物品的固定模型与纹理。
		default:
			return "", fmt.Errorf("unsupported item component %s", name)
		}
	}
	return id, nil
}
