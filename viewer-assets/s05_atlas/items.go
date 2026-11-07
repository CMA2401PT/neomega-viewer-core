package s05_atlas

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"neomega-viewer-core/viewer-assets/asset"
	"neomega-viewer-core/viewer-assets/utils/jsonfile"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
)

// 与客户端 items/*.json 保持相同结构，只折叠固定展示且无物品组件时可确定的分支。
type staticItemModel struct {
	Type           string            `json:"type"`
	Property       string            `json:"property"`
	Model          json.RawMessage   `json:"model"`
	Tints          []json.RawMessage `json:"tints"`
	Transformation json.RawMessage   `json:"transformation"`
	OnFalse        *staticItemModel  `json:"on_false"`
	Fallback       *staticItemModel  `json:"fallback"`
	Entries        []struct {
		Threshold float64         `json:"threshold"`
		Model     staticItemModel `json:"model"`
	} `json:"entries"`
	Cases []struct {
		When  json.RawMessage `json:"when"`
		Model staticItemModel `json:"model"`
	} `json:"cases"`
}

var errDynamicItem = errors.New("item requires dynamic rendering")

func fixedItemReference(definition *staticItemModel) (model.ResourceID, error) {
	if definition == nil {
		return "", fmt.Errorf("%w: no fixed display fallback", errDynamicItem)
	}
	if len(definition.Transformation) != 0 {
		return "", fmt.Errorf("%w: item transformation", errDynamicItem)
	}
	switch definition.Type {
	case "minecraft:model":
		if len(definition.Tints) > 0 {
			return "", fmt.Errorf("%w: model tints", errDynamicItem)
		}
		var reference string
		if err := json.Unmarshal(definition.Model, &reference); err != nil {
			return "", fmt.Errorf("model reference: %w", err)
		}
		return model.ParseResourceID(reference)
	case "minecraft:condition":
		// 展示框里的物品没有使用者、鱼钩或选中的收纳袋物品；默认鞘翅未损坏。
		switch definition.Property {
		case "minecraft:using_item", "minecraft:fishing_rod/cast", "minecraft:bundle/has_selected_item", "minecraft:broken":
			return fixedItemReference(definition.OnFalse)
		}
	case "minecraft:select":
		switch definition.Property {
		case "minecraft:display_context":
			for _, choice := range definition.Cases {
				var values []string
				if err := json.Unmarshal(choice.When, &values); err != nil {
					var value string
					if err := json.Unmarshal(choice.When, &value); err != nil {
						return "", err
					}
					values = []string{value}
				}
				for _, value := range values {
					if value == "fixed" {
						return fixedItemReference(&choice.Model)
					}
				}
			}
			return fixedItemReference(definition.Fallback)
		case "minecraft:block_state", "minecraft:charge_type", "minecraft:trim_material":
			// 缺省物品没有 block_state、装填物或盔甲纹饰组件。
			return fixedItemReference(definition.Fallback)
		}
	case "minecraft:range_dispatch":
		switch definition.Property {
		case "minecraft:use_cycle", "minecraft:use_duration", "minecraft:crossbow/pull":
			// 固定展示中没有使用者，使用时间和拉弓进度为零。
			selected, threshold := definition.Fallback, -1.0e300
			for i := range definition.Entries {
				entry := &definition.Entries[i]
				if entry.Threshold <= 0 && entry.Threshold >= threshold {
					selected, threshold = &entry.Model, entry.Threshold
				}
			}
			return fixedItemReference(selected)
		}
	}
	return "", fmt.Errorf("%w: %s %s", errDynamicItem, definition.Type, definition.Property)
}

func staticItems(snapshot asset.Snapshot, input string, models model.Models, selected map[model.ResourceID]bool) (asset.ItemModels, error) {
	// 读取普通物品的显式模型引用，并将物品与框体依赖的纹理加入 selected；复杂定义记录为不支持。
	items := asset.ItemModels{Models: map[string]model.ResourceID{}, Unsupported: map[string]string{}}
	resolver := model.NewResolver(models)
	roots := []model.ResourceID{"minecraft:block/item_frame", "minecraft:block/glow_item_frame"}
	for id, file := range snapshot.Items {
		definition, err := jsonfile.Read[struct {
			Model staticItemModel `json:"model"`
		}](filepath.Join(input, filepath.FromSlash(file.Path)))
		if err != nil {
			return items, err
		}
		target, err := fixedItemReference(&definition.Model)
		if errors.Is(err, errDynamicItem) {
			items.Unsupported[string(id)] = err.Error()
			continue
		}
		if err != nil {
			return items, fmt.Errorf("item %s: %w", id, err)
		}
		items.Models[string(id)] = target
		roots = append(roots, target)
	}
	for _, id := range roots {
		resolved, err := resolver.Resolve(id)
		if err != nil {
			return items, err
		}
		for _, element := range resolved.Elements {
			for _, face := range element.Faces {
				texture, _, err := resolved.Texture(face.Texture)
				if err != nil {
					return items, err
				}
				selected[texture] = true
			}
		}
		if resolved.Builtin == "minecraft:builtin/generated" {
			for layer := 0; layer < 5; layer++ {
				slot := fmt.Sprintf("layer%d", layer)
				if _, ok := resolved.Textures[slot]; !ok {
					break
				}
				texture, _, err := resolved.Texture(model.TextureRef("#" + slot))
				if err != nil {
					return items, err
				}
				selected[texture] = true
			}
		}
	}
	return items, nil
}
