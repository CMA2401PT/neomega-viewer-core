// SPDX-License-Identifier: MIT
// Package java_block_model defines and interprets Java resource models.
// Model shapes follow Minecraft resource JSON; format conventions are
// informed by mc-assets (MIT). See ../../third_party/mc-assets-notices/NOTICE.md.
package java_block_model

import (
	"bytes"
	"encoding/json"
)

type Vec3 [3]float64
type UV [4]float64
type Direction string
type TextureRef string

// TextureBinding preserves both old string references and the modern sprite
// object. ForceTranslucent is material information, not part of the resource ID.
type TextureBinding struct {
	Sprite           TextureRef
	ForceTranslucent *bool
	Object           bool
}

func (binding *TextureBinding) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		binding.Object = false
		return json.Unmarshal(data, &binding.Sprite)
	}
	var value struct {
		Sprite           TextureRef `json:"sprite"`
		ForceTranslucent *bool      `json:"force_translucent,omitempty"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*binding = TextureBinding{Sprite: value.Sprite, ForceTranslucent: value.ForceTranslucent, Object: true}
	return nil
}
func (binding TextureBinding) MarshalJSON() ([]byte, error) {
	if !binding.Object && binding.ForceTranslucent == nil {
		return json.Marshal(binding.Sprite)
	}
	return json.Marshal(struct {
		Sprite           TextureRef `json:"sprite"`
		ForceTranslucent *bool      `json:"force_translucent,omitempty"`
	}{Sprite: binding.Sprite, ForceTranslucent: binding.ForceTranslucent})
}

type Face struct {
	Texture   TextureRef `json:"texture"`
	UV        *UV        `json:"uv,omitempty"`
	CullFace  Direction  `json:"cullface,omitempty"`
	Rotation  float64    `json:"rotation,omitempty"`
	TintIndex *int       `json:"tintindex,omitempty"`
}

// ElementRotation retains both the traditional axis/angle form and the newer
// per-axis form so extraction and baking use the same rotation description.
type ElementRotation struct {
	Origin  Vec3    `json:"origin"`
	Axis    string  `json:"axis,omitempty"`
	Angle   float64 `json:"angle,omitempty"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
	Z       float64 `json:"z,omitempty"`
	Rescale bool    `json:"rescale,omitempty"`
}

type Element struct {
	Name                   string             `json:"name,omitempty"`
	Comment                string             `json:"__comment,omitempty"`
	ShadeDirectionOverride Direction          `json:"shade_direction_override,omitempty"`
	From                   Vec3               `json:"from"`
	To                     Vec3               `json:"to"`
	Rotation               *ElementRotation   `json:"rotation,omitempty"`
	Faces                  map[Direction]Face `json:"faces"`
	Shade                  *bool              `json:"shade,omitempty"`
	LightEmission          int                `json:"light_emission,omitempty"`
}

type Transform struct {
	Rotation    *Vec3 `json:"rotation,omitempty"`
	Translation *Vec3 `json:"translation,omitempty"`
	Scale       *Vec3 `json:"scale,omitempty"`
}

type ItemOverride struct {
	Predicate map[string]float64 `json:"predicate"`
	Model     ResourceID         `json:"model"`
}

type Model struct {
	Parent   ResourceID                `json:"parent,omitempty"`
	Textures map[string]TextureBinding `json:"textures,omitempty"`
	// nil inherits elements; a pointer to an empty slice explicitly removes them.
	Elements         *[]Element           `json:"elements,omitempty"`
	AmbientOcclusion *bool                `json:"ambientocclusion,omitempty"`
	Display          map[string]Transform `json:"display,omitempty"`
	GUILight         string               `json:"gui_light,omitempty"`
	Overrides        []ItemOverride       `json:"overrides,omitempty"`
	// Preserve extensions without making the known model structure untyped.
	Extra map[string]json.RawMessage `json:"-"`
}

var modelFields = []string{"parent", "textures", "elements", "ambientocclusion", "display", "gui_light", "overrides"}

func (m *Model) UnmarshalJSON(data []byte) error {
	type plain Model
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	for _, key := range modelFields {
		delete(extra, key)
	}
	*m = Model(value)
	m.Extra = extra
	return nil
}

func (m Model) MarshalJSON() ([]byte, error) {
	type plain Model
	data, err := json.Marshal(plain(m))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for key, value := range m.Extra {
		fields[key] = value
	}
	return json.Marshal(fields)
}

// Models maps canonical resource IDs to definitions, preserving parent references.
type Models map[ResourceID]Model
