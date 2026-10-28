// SPDX-License-Identifier: MIT
package s04_special

import (
	"neomega-assets/asset"
	"slices"
	"strings"
)

func classifyEmpty(id asset.ID) (asset.BlockRenderer, bool) {
	if id.Namespace() != "minecraft" {
		return asset.BlockRenderer{}, false
	}
	switch id.Path() {
	case "air", "cave_air", "void_air", "barrier", "light", "structure_void":
		return asset.BlockRenderer{Kind: asset.RendererInvisible}, true
	case "water", "lava", "bubble_column":
		return asset.BlockRenderer{Kind: asset.RendererFluid, Family: strings.TrimSuffix(id.Path(), "_column"), DynamicFields: []string{"fluidLevel", "neighborFluidStates"}}, true
	case "moving_piston":
		return asset.BlockRenderer{Kind: asset.RendererDynamic, Family: "moving_piston", DynamicFields: []string{"movedBlockState", "progress"}}, true
	}
	return asset.BlockRenderer{}, false
}

func specialProfile(id asset.ID) asset.BlockRenderer {
	name := id.Path()
	profile := asset.BlockRenderer{Kind: asset.RendererSpecial, StaticPreview: true}
	switch {
	case strings.HasSuffix(name, "_chest") || name == "chest":
		profile.Family = "chest"
		profile.DynamicFields = []string{"lidAngle", "seasonalSkin"}
	case strings.HasSuffix(name, "shulker_box"):
		profile.Family = "shulker_box"
		profile.DynamicFields = []string{"lidProgress"}
	case strings.HasSuffix(name, "_banner"):
		profile.Family = "banner"
		profile.DynamicFields = []string{"patterns", "clothMotion"}
	case strings.HasSuffix(name, "_head") || strings.HasSuffix(name, "_skull"):
		profile.Family = "head"
		if strings.Contains(name, "player") {
			profile.DynamicFields = []string{"profileSkin"}
		}
		if strings.Contains(name, "dragon") {
			profile.DynamicFields = []string{"jawAngle"}
		}
		if strings.Contains(name, "piglin") {
			profile.DynamicFields = []string{"earMotion"}
		}
	case name == "decorated_pot":
		profile.Family = "decorated_pot"
		profile.DynamicFields = []string{"sherds", "wobble"}
	case strings.Contains(name, "copper_golem_statue"):
		profile.Family = "copper_golem_statue"
	case name == "conduit":
		profile.Family = "conduit"
		profile.DynamicFields = []string{"active", "rotation", "eyeState"}
	case name == "end_portal" || name == "end_gateway":
		profile.Family = "portal"
		profile.DynamicFields = []string{"time", "portalShader"}
	default:
		profile.Family = "community_template"
	}
	return profile
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

// Families also declare textures used by dynamic effects, not just those that
// appear in the static template's faces (banner patterns, eyes, lid skins, etc.).
var familyPrefixes = map[string][]string{
	"chest":               {"entity/chest/"},
	"shulker_box":         {"entity/shulker/"},
	"banner":              {"entity/banner/"},
	"decorated_pot":       {"entity/decorated_pot/"},
	"copper_golem_statue": {"entity/copper_golem/"},
	"conduit":             {"entity/conduit/"},
	"portal":              {"entity/end_portal/"},
	"head":                {"entity/skeleton/", "entity/creeper/creeper", "entity/zombie/zombie", "entity/enderdragon/dragon", "entity/piglin/piglin", "entity/player/slim/steve"},
	"water":               {"block/water_"},
	"lava":                {"block/lava_"},
	"bubble":              {"block/water_"},
}
