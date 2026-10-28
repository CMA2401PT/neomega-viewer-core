// SPDX-License-Identifier: MIT
// Model shapes follow Minecraft resource JSON; compatibility conventions are
// informed by mc-assets (MIT). See NOTICE.md and resources/third_party/mc-assets/LICENSE.
package asset

import (
	"bytes"
	"encoding/json"
	"fmt"
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
// per-axis form. This pipeline carries geometry; it does not bake rotations.
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
	Model     ID                 `json:"model"`
}

type Model struct {
	Parent   ID                        `json:"parent,omitempty"`
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

type Apply struct {
	Model  ID      `json:"model"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Z      float64 `json:"z,omitempty"`
	UVLock bool    `json:"uvlock,omitempty"`
	Weight *int    `json:"weight,omitempty"`
}

// Choice models the JSON union "one application or an array of weighted ones".
// Alternatives stay alternatives; generation does not randomly select one.
type Choice struct {
	Options []Apply
	Array   bool
}

func (choice *Choice) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("empty model choice")
	}
	if data[0] == '[' {
		choice.Array = true
		return json.Unmarshal(data, &choice.Options)
	}
	var value Apply
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	choice.Array = false
	choice.Options = []Apply{value}
	return nil
}
func (choice Choice) MarshalJSON() ([]byte, error) {
	if choice.Array {
		return json.Marshal(choice.Options)
	}
	if len(choice.Options) != 1 {
		return nil, fmt.Errorf("single model choice needs one application")
	}
	return json.Marshal(choice.Options[0])
}

// Condition separates property comparisons from logical OR/AND groups.
type Condition struct {
	Properties map[string]string
	OR         []Condition
	AND        []Condition
}

func (condition *Condition) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*condition = Condition{Properties: map[string]string{}}
	for key, value := range fields {
		switch key {
		case "OR":
			if err := json.Unmarshal(value, &condition.OR); err != nil {
				return err
			}
		case "AND":
			if err := json.Unmarshal(value, &condition.AND); err != nil {
				return err
			}
		default:
			var text string
			raw := bytes.TrimSpace(value)
			switch {
			case len(raw) > 0 && raw[0] == '"':
				if err := json.Unmarshal(raw, &text); err != nil {
					return err
				}
			case bytes.Equal(raw, []byte("true")) || bytes.Equal(raw, []byte("false")):
				text = string(raw)
			case len(raw) > 0 && (raw[0] == '-' || raw[0] >= '0' && raw[0] <= '9'):
				var number json.Number
				if err := json.Unmarshal(raw, &number); err != nil {
					return err
				}
				text = number.String()
			default:
				return fmt.Errorf("condition %s must be a scalar property value", key)
			}
			// Resource packs and older mc-assets data may use bools/numbers;
			// property comparisons have textual semantics in the renderer.
			condition.Properties[key] = text
		}
	}
	return nil
}
func (condition Condition) MarshalJSON() ([]byte, error) {
	fields := map[string]json.RawMessage{}
	for key, value := range condition.Properties {
		fields[key], _ = json.Marshal(value)
	}
	if condition.OR != nil {
		fields["OR"], _ = json.Marshal(condition.OR)
	}
	if condition.AND != nil {
		fields["AND"], _ = json.Marshal(condition.AND)
	}
	return json.Marshal(fields)
}

type Part struct {
	When  *Condition `json:"when,omitempty"`
	Apply Choice     `json:"apply"`
}

type BlockState struct {
	Variants  map[string]Choice `json:"variants,omitempty"`
	Multipart []Part            `json:"multipart,omitempty"`
}

func (state BlockState) Applications() []Apply {
	var applications []Apply
	for _, choice := range state.Variants {
		applications = append(applications, choice.Options...)
	}
	for _, part := range state.Multipart {
		applications = append(applications, part.Apply.Options...)
	}
	return applications
}

// BlockStates maps export names (the default minecraft: namespace is omitted)
// to model choices. A build contains one version, so no version envelope is used.
type BlockStates map[string]BlockState

// Models maps export names to model definitions, preserving parent references.
type Models map[string]Model

type ModelIndex struct {
	// Empty geometry can be intentional (air/fluids) or require a special renderer.
	GeometrylessBlockStates []ID   `json:"geometrylessBlockStates"`
	Version                 string `json:"version"`
	RequiredTextures        []ID   `json:"requiredTextures"`
	BuiltinModels           []ID   `json:"builtinModels"`
	ModelCount              int    `json:"modelCount"`
	BlockStateCount         int    `json:"blockStateCount"`
}
