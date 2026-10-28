// SPDX-License-Identifier: MIT
// Package resources holds hand-maintained renderer rules and bundled assets.
// These rules describe Minecraft resources; s04_special applies them.
package resources

import (
	"strings"

	"neomega-assets/asset"
)

// JavaTargetVersion is aligned with neomega-blocks' existing conversion preset.
const JavaTargetVersion = "1.21.4"

// NonModelRenderer identifies supported vanilla blocks without static geometry.
// The pipeline itself handles minecraft:air.
func NonModelRenderer(id asset.ID) (asset.BlockRenderer, bool) {
	if id.Namespace() != "minecraft" {
		return asset.BlockRenderer{}, false
	}
	switch id.Path() {
	case "cave_air", "void_air", "barrier", "light", "structure_void":
		return asset.BlockRenderer{Kind: asset.RendererInvisible}, true
	case "water", "lava", "bubble_column":
		return asset.BlockRenderer{Kind: asset.RendererFluid, Family: strings.TrimSuffix(id.Path(), "_column"), DynamicFields: []string{"fluidLevel", "neighborFluidStates"}}, true
	case "moving_piston":
		return asset.BlockRenderer{Kind: asset.RendererDynamic, Family: "moving_piston", DynamicFields: []string{"movedBlockState", "progress"}}, true
	}
	return asset.BlockRenderer{}, false
}

// SpecialRenderer describes the runtime inputs beyond a community preview.
func SpecialRenderer(id asset.ID) asset.BlockRenderer {
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
	case strings.HasSuffix(name, "_sign"):
		profile.Family = "sign"
		profile.DynamicFields = []string{"frontText", "backText"}
	case strings.HasSuffix(name, "_bed"):
		profile.Family = "bed"
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

// ModelRenderer adds runtime requirements to an existing static model.
func ModelRenderer(id asset.ID) asset.BlockRenderer {
	profile := asset.BlockRenderer{Kind: asset.RendererModel, StaticPreview: true}
	if strings.HasSuffix(id.Path(), "_sign") {
		profile.Family = "sign"
		profile.DynamicFields = []string{"frontText", "backText"}
	}
	return profile
}

// FamilyUsesTexture includes material used beyond the static preview.
func FamilyUsesTexture(family string, id asset.ID) bool {
	if id.Namespace() != "minecraft" {
		return false
	}
	for _, prefix := range familyPrefixes[family] {
		if strings.HasPrefix(id.Path(), prefix) {
			return true
		}
	}
	return false
}

// Families also declare textures used by dynamic effects, not just those that
// appear in the static template's faces (banner patterns, eyes, lid skins, etc.).
var familyPrefixes = map[string][]string{
	"bed":                 {"entity/bed/"},
	"sign":                {"entity/signs/"},
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

// NormalizeModelReference adapts the short names in the bundled mc-assets models.
func NormalizeModelReference(value asset.ID) (asset.ID, error) {
	id, err := asset.ParseID(string(value))
	if err != nil {
		return "", err
	}
	path := id.Path()
	if !strings.HasPrefix(path, "block/") && !strings.HasPrefix(path, "item/") && !strings.HasPrefix(path, "builtin/") {
		path = "block/" + path
	}
	return asset.ID(id.Namespace() + ":" + path), nil
}

// NormalizeTextureReference adapts legacy texture paths to current resources.
func NormalizeTextureReference(value asset.TextureRef, available map[asset.ID]asset.File) (asset.TextureRef, error) {
	if strings.HasPrefix(string(value), "#") {
		return value, nil
	}
	id, err := asset.ParseID(string(value))
	if err != nil {
		return "", err
	}
	// Keep a version's real path, including explicit overrides, when it exists.
	if _, ok := available[id]; ok {
		return asset.TextureRef(id), nil
	}
	// This entity texture moved in modern client resources.
	if id == "minecraft:entity/banner_base" {
		id = "minecraft:entity/banner/banner_base"
	}
	return asset.TextureRef(id), nil
}

// ImportedModelID keeps bundled models separate from native client models.
func ImportedModelID(id asset.ID) asset.ID {
	return asset.ID("neomega:" + id.Path())
}

// TemplateVariant describes a model derived from an imported template.
// The importing stage handles file I/O and applies these declared replacements.
type TemplateVariant struct {
	Source     asset.ID
	Target     asset.ID
	Textures   map[string]asset.ID
	ClearTint  bool
	OriginNote string
}

// TemplateVariantFor chooses a colored shulker's actual skin. Clearing tint
// prevents the already colored texture from receiving a second color multiply.
func TemplateVariantFor(block asset.ID) (TemplateVariant, bool) {
	if !strings.HasSuffix(block.Path(), "_shulker_box") {
		return TemplateVariant{}, false
	}
	color := strings.TrimSuffix(block.Path(), "_shulker_box")
	return TemplateVariant{
		Source: ImportedModelID("minecraft:block/entity/dyed_shulker_box"),
		Target: asset.ID("neomega:block/preview/" + block.Path()),
		Textures: map[string]asset.ID{
			"0": asset.ID("minecraft:entity/shulker/shulker_" + color),
		},
		ClearTint:  true,
		OriginNote: " (selected " + color + " skin)",
	}, true
}
