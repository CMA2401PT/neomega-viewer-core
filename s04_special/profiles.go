// SPDX-License-Identifier: MIT
package s04_special

import (
	"slices"
	"strings"

	"neomega-assets/asset"
	"neomega-assets/resources"
)

func classifyEmpty(id asset.ID) (asset.BlockRenderer, bool) {
	if id == "minecraft:air" {
		return asset.BlockRenderer{Kind: asset.RendererInvisible}, true
	}
	return resources.NonModelRenderer(id)
}

func stateProperties(state asset.BlockState) []string {
	keys := map[string]bool{}
	for variant := range state.Variants {
		for _, pair := range strings.Split(variant, ",") {
			if key, _, ok := strings.Cut(pair, "="); ok {
				keys[key] = true
			}
		}
	}
	var visit func(*asset.Condition)
	visit = func(condition *asset.Condition) {
		if condition == nil {
			return
		}
		for key := range condition.Properties {
			keys[key] = true
		}
		for i := range condition.OR {
			visit(&condition.OR[i])
		}
		for i := range condition.AND {
			visit(&condition.AND[i])
		}
	}
	for _, part := range state.Multipart {
		visit(part.When)
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}
