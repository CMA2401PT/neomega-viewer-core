package s05_atlas

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"neomega-viewer-core/viewer-assets/asset"
	"neomega-viewer-core/viewer-assets/utils/jsonfile"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
)

func staticItems(snapshot asset.Snapshot, input string, models model.Models, selected map[model.ResourceID]bool) (asset.ItemModels, error) {
	// 读取普通物品的显式模型引用，并将物品与框体依赖的纹理加入 selected；复杂定义记录为不支持。
	items := asset.ItemModels{Models: map[string]model.ResourceID{}, Unsupported: map[string]string{}}
	resolver := model.NewResolver(models)
	roots := []model.ResourceID{"minecraft:block/item_frame", "minecraft:block/glow_item_frame"}
	for id, file := range snapshot.Items {
		definition, err := jsonfile.Read[struct {
			Model struct {
				Type  string            `json:"type"`
				Model json.RawMessage   `json:"model"`
				Tints []json.RawMessage `json:"tints"`
			} `json:"model"`
		}](filepath.Join(input, filepath.FromSlash(file.Path)))
		if err != nil {
			return items, err
		}
		if definition.Model.Type != "minecraft:model" || len(definition.Model.Tints) > 0 {
			items.Unsupported[string(id)] = "only plain minecraft:model without tints is supported"
			continue
		}
		var reference string
		if err := json.Unmarshal(definition.Model.Model, &reference); err != nil {
			return items, fmt.Errorf("item %s model reference: %w", id, err)
		}
		target, err := model.ParseResourceID(reference)
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
