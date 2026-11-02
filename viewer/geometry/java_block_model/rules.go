// SPDX-License-Identifier: MIT
package java_block_model

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Apply struct {
	Model  ResourceID `json:"model"`
	X      float64    `json:"x,omitempty"`
	Y      float64    `json:"y,omitempty"`
	Z      float64    `json:"z,omitempty"`
	UVLock bool       `json:"uvlock,omitempty"`
	Weight *int       `json:"weight,omitempty"`
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

type ModelRules struct {
	Variants  map[string]Choice `json:"variants,omitempty"`
	Multipart []Part            `json:"multipart,omitempty"`
}

func (state ModelRules) Applications() []Apply {
	var applications []Apply
	for _, choice := range state.Variants {
		applications = append(applications, choice.Options...)
	}
	for _, part := range state.Multipart {
		applications = append(applications, part.Apply.Options...)
	}
	return applications
}

// RuleTable maps canonical block names to model selection rules. A rule is
// not a concrete block state; its input is ir.IRStates with Java properties.
type RuleTable map[ResourceID]ModelRules
