package tablegen

import (
	"errors"
	"fmt"
	"path/filepath"

	"neomega-viewer-core/viewer-assets/asset"
	"neomega-viewer-core/viewer-assets/utils/jsonfile"
	"neomega-viewer-core/viewer/geometry"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
	"neomega-viewer-core/viewer/item_render"
)

func (engine *Builder) BuildItems(assetsDir string) (*item_render.Catalog, error) {
	// 物品和框体都使用当前 Bundle 的图集；不依赖运行时源码目录。
	definitions, err := jsonfile.Read[asset.ItemModels](filepath.Join(assetsDir, "items.json"))
	if err != nil {
		return nil, err
	}
	catalog := &item_render.Catalog{Items: map[string]geometry.Mesh{}, Models: map[string]geometry.Mesh{}, Unsupported: definitions.Unsupported}
	for id, target := range definitions.Models {
		resolved, err := engine.models.Resolve(target)
		if err != nil {
			return nil, err
		}
		mesh, err := item_render.Bake(resolved, engine.Bundle.Atlas, engine.Bundle.AtlasImage)
		if errors.Is(err, item_render.ErrUnsupportedModel) {
			catalog.Unsupported[id] = err.Error()
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("item %s: %w", id, err)
		}
		catalog.Items[id] = mesh
	}
	for _, id := range []model.ResourceID{"minecraft:block/item_frame", "minecraft:block/glow_item_frame"} {
		resolved, err := engine.models.Resolve(id)
		if err != nil {
			return nil, err
		}
		mesh, err := item_render.Bake(resolved, engine.Bundle.Atlas, engine.Bundle.AtlasImage)
		if err != nil {
			return nil, err
		}
		catalog.Models[string(id)] = mesh
	}
	return catalog, nil
}
