package java_block_model

import "strings"

// UsesProperty reports whether model selection refers to this Java property.
func (rules ModelRules) UsesProperty(key string) bool {
	for selector := range rules.Variants {
		for _, pair := range strings.Split(selector, ",") {
			name, _, _ := strings.Cut(pair, "=")
			if name == key {
				return true
			}
		}
	}
	var uses func(*Condition) bool
	uses = func(condition *Condition) bool {
		if condition == nil {
			return false
		}
		if _, ok := condition.Properties[key]; ok {
			return true
		}
		for i := range condition.OR {
			if uses(&condition.OR[i]) {
				return true
			}
		}
		for i := range condition.AND {
			if uses(&condition.AND[i]) {
				return true
			}
		}
		return false
	}
	for _, part := range rules.Multipart {
		if uses(part.When) {
			return true
		}
	}
	return false
}
