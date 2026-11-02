// Package resources contains explicit static preview choices that are not
// available in the model JSON (biome-independent tints and glass/fluid layers).
package resources

import (
	"strconv"
	"strings"

	"neomega-blocks/ir"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
	"neomega-viewer-core/viewer/geometry/material"
)

type PreviewPolicy struct{}

func (PreviewPolicy) Mode(state ir.IRBlock, sampled material.Mode) material.Mode {
	name := state.ShortName()
	if strings.Contains(name, "glass") || name == "water" || name == "bubble_column" || name == "ice" || name == "honey_block" || name == "slime_block" {
		return material.Blend
	}
	return sampled
}

func (PreviewPolicy) Opacity(state ir.IRBlock) float32 {
	if state.ShortName() == "water" || state.ShortName() == "bubble_column" {
		return 0.7
	}
	return 1
}

// These fixed colors make isolated previews readable. A caller may provide a
// different java_block_model.Policy for another biome or a particular static scene.
func (PreviewPolicy) Tint(state ir.IRBlock, index int) [3]float32 {
	if index < 0 {
		return [3]float32{1, 1, 1}
	}
	name := state.ShortName()
	switch {
	case name == "redstone_wire":
		power, _ := strconv.Atoi(state.States().GetDefault("power", "0").(string))
		p := float64(power) / 15
		r := uint32((0.3 + 0.6*p) * 255)
		g := uint32(max(0, p*p*0.7-0.5) * 255)
		b := uint32(max(0, p*p*0.6-0.7) * 255)
		return jmodel.Linear(r<<16 | g<<8 | b)
	case name == "water" || name == "bubble_column":
		return jmodel.Linear(0x3f76e4)
	case name == "birch_leaves":
		return jmodel.Linear(0x80a755)
	case name == "spruce_leaves":
		return jmodel.Linear(0x619961)
	case strings.HasSuffix(name, "_leaves") || name == "vine":
		return jmodel.Linear(0x77ab2f)
	default:
		return jmodel.Linear(0x91bd59)
	}
}

// PreviewProperties supplies the dry-block convention. Fluid layers can be
// represented by static caller overrides when a scene actually contains water.
func PreviewProperties(state ir.IRBlock) ir.IRBlock {
	if _, exists := state.States().Get("waterlogged"); !exists {
		state = ir.IRBlock(state.NameWithNameSpace() + " " + string(state.States().Set("waterlogged", "false")))
	}
	return state
}

// FluidPreview creates an isolated, still fluid surface, without world queries
// or simulation. A section renderer can supply its own neighbor-based surface.
func FluidPreview(state ir.IRBlock, family string) *jmodel.Resolved {
	fluid := "water"
	if family == "lava" {
		fluid = "lava"
	}
	level, _ := strconv.Atoi(state.States().GetDefault("level", "0").(string))
	height := 8.0 / 9
	if level >= 8 {
		height = 1
	} else {
		height = float64(8-level) / 9
	}
	elements := []jmodel.Element{{From: jmodel.Vec3{0, 0, 0}, To: jmodel.Vec3{16, height * 16, 16}, Faces: map[jmodel.Direction]jmodel.Face{}}}
	for _, side := range []jmodel.Direction{"down", "up", "north", "south", "west", "east"} {
		texture := jmodel.TextureRef("#flow")
		if side == "down" || side == "up" {
			texture = "#still"
		}
		face := jmodel.Face{Texture: texture}
		if fluid == "water" {
			index := 0
			face.TintIndex = &index
		}
		elements[0].Faces[side] = face
	}
	if fluid == "lava" {
		shade := false
		elements[0].Shade = &shade
	}
	return &jmodel.Resolved{
		Elements: elements,
		Textures: map[string]jmodel.TextureBinding{
			"still": {Sprite: jmodel.TextureRef("minecraft:block/" + fluid + "_still")},
			"flow":  {Sprite: jmodel.TextureRef("minecraft:block/" + fluid + "_flow")},
		},
	}
}
