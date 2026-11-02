// Model selection evaluates Java IR properties while retaining weighted choices.
package java_block_model

import (
	"fmt"
	"slices"
	"strings"

	"neomega-blocks/ir"
)

func valueMatches(actual, expected string) bool {
	negative := strings.HasPrefix(expected, "!")
	expected = strings.TrimPrefix(expected, "!")
	found := slices.Contains(strings.Split(expected, "|"), actual)
	return found != negative
}

func variantMatches(selector string, properties map[string]string) bool {
	if selector == "" {
		return true
	}
	for _, pair := range strings.Split(selector, ",") {
		key, expected, ok := strings.Cut(pair, "=")
		actual, present := properties[key]
		if !ok || !present || !valueMatches(actual, expected) {
			return false
		}
	}
	return true
}

func matches(condition *Condition, properties map[string]string) bool {
	if condition == nil {
		return true
	}
	for key, expected := range condition.Properties {
		actual, present := properties[key]
		if !present || !valueMatches(actual, expected) {
			return false
		}
	}
	if condition.OR != nil {
		found := false
		for i := range condition.OR {
			found = found || matches(&condition.OR[i], properties)
		}
		if !found {
			return false
		}
	}
	for i := range condition.AND {
		if !matches(&condition.AND[i], properties) {
			return false
		}
	}
	return true
}

// Select returns all matching model choices. The caller chooses an alternative
// from each Choice; weights and transforms remain intact. States must use Java
// string-valued properties; parsing the IR here does not convert editions.
func Select(state ModelRules, states ir.IRStates) ([]Choice, error) {
	properties := make(map[string]string)
	for key, value := range states.KeyValues {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("Java property %s must be a string", key)
		}
		properties[key] = text
	}
	// 每个命中的 multipart 部件都保留自己的候选组。
	choices := []Choice{}
	keys := make([]string, 0, len(state.Variants))
	for key := range state.Variants {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if variantMatches(key, properties) {
			choices = append(choices, state.Variants[key])
		}
	}
	if len(state.Variants) != 0 && len(choices) == 0 {
		return nil, fmt.Errorf("no variant matches properties %v", properties)
	}
	for _, part := range state.Multipart {
		if matches(part.When, properties) {
			choices = append(choices, part.Apply)
		}
	}

	for _, choice := range choices {
		if len(choice.Options) == 0 {
			return nil, fmt.Errorf("empty model choice")
		}
	}
	return choices, nil
}
