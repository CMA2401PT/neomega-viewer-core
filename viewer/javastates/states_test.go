package javastates

import (
	legacy "neomega-blocks/presets/java/v1"
	"testing"
)

func TestLegacyStatesRemainAddressable(t *testing.T) {
	known := map[uint32]bool{}
	for state := range JavaCompatibleStates() {
		known[state.Hash()] = true
	}
	missing := map[string]int{}
	for state := range legacy.JavaCompatibleStates() {
		if !known[state.Hash()] {
			missing[state.NameWithNameSpace()]++
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing legacy states by block: %v", missing)
	}
}
