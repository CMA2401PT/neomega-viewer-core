// SPDX-License-Identifier: MIT
package s04_special

import (
	"slices"
	"strings"

	"neomega-assets/resources"
	jmodel "neomega-blocks/geometry/java_block_model"
)

func classifyEmpty(id jmodel.ResourceID) (jmodel.BlockRenderer, bool) {
	if id == "minecraft:air" {
		return jmodel.BlockRenderer{Kind: jmodel.RendererInvisible}, true
	}
	return resources.NonModelRenderer(id)
}

func stateProperties(state jmodel.ModelRules) []string {
	keys := map[string]bool{}
	for variant := range state.Variants {
		for _, pair := range strings.Split(variant, ",") {
			if key, _, ok := strings.Cut(pair, "="); ok {
				keys[key] = true
			}
		}
	}
	var visit func(*jmodel.Condition)
	visit = func(condition *jmodel.Condition) {
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
