// Package javastates supplies the versioned state set used by preview resources.
// Legacy block conversion presets remain unchanged.
package javastates

import (
	"iter"
	"sync"

	"neomega-blocks/ir"
	legacy "neomega-blocks/presets/java/v1"
	current "neomega-blocks/presets/java/v26_3"
)

const JavaTargetVersion = current.JavaTargetVersion

type StateMapping = legacy.StateMapping

func single(block ir.IRBlock) ir.IRBlock {
	return ir.IRBlock(block.NameWithNameSpace() + " " + string(block.States().Delete("waterlogged"))).Tidy()
}

var load = sync.OnceValue(func() struct {
	states  []ir.IRBlock
	aliases []StateMapping
} {
	catalog, err := current.Open()
	if err != nil {
		panic(err)
	} // Corrupt embedded data is a build error.
	var result struct {
		states  []ir.IRBlock
		aliases []StateMapping
	}
	known := map[ir.IRBlock]bool{}
	for state := range catalog.States() {
		block := single(state.Java)
		if !known[block] {
			result.states = append(result.states, block)
			known[block] = true
		}
	}
	// Older complete states may now omit a newly introduced property. Resolve
	// them to the official default, while retaining the original input hash.
	for old := range legacy.JavaCompatibleStates() {
		if known[old] {
			continue
		}
		input := old
		if old.ShortName() == "chain" {
			input = ir.IRBlock("minecraft:iron_chain " + string(old.States()))
		}
		if old.ShortName() == "creaking_heart" {
			if active, ok := old.States().Get("active"); ok {
				value := "dormant"
				if active == "true" {
					value = "awake"
				}
				input = ir.IRBlock(old.NameWithNameSpace() + " " + string(old.States().Delete("active").Set("creaking_heart_state", value))).Tidy()
			}
			if value, ok := old.States().Get("creaking"); ok {
				state := "dormant"
				if value == "active" {
					state = "awake"
				}
				if value == "disabled" {
					state = "uprooted"
				}
				input = ir.IRBlock(old.NameWithNameSpace() + " " + string(old.States().Delete("creaking").Set("creaking_heart_state", state))).Tidy()
			}
		}
		target, err := catalog.Normalize(input)
		if err != nil {
			if translated, ok := legacy.NormalizeSingleLayer(old); ok {
				target, err = catalog.Normalize(translated)
			}
		}
		if err != nil {
			continue
		}
		result.aliases = append(result.aliases, StateMapping{Source: old, Target: single(target)})
		known[old] = true
	}
	return result
})

func JavaFullStates() iter.Seq[ir.IRBlock] {
	return func(yield func(ir.IRBlock) bool) {
		for _, state := range load().states {
			if !yield(state) {
				return
			}
		}
	}
}

func JavaCompatibility() iter.Seq[StateMapping] {
	return func(yield func(StateMapping) bool) {
		for _, alias := range load().aliases {
			if !yield(alias) {
				return
			}
		}
	}
}

func JavaCompatibleStates() iter.Seq[ir.IRBlock] {
	return func(yield func(ir.IRBlock) bool) {
		for block := range JavaFullStates() {
			if !yield(block) {
				return
			}
		}
		for alias := range JavaCompatibility() {
			if !yield(alias.Source) {
				return
			}
		}
	}
}

func HashToBlock(hash uint32) (ir.IRBlock, bool) {
	catalog, err := current.Open()
	if err != nil {
		return "", false
	}
	if state, ok := catalog.ByHash(hash); ok {
		return state.Java, true
	}
	if state, ok := renderHashes()[hash]; ok {
		return state, true
	}
	return legacy.HashToBlock(hash)
}

var renderHashes = sync.OnceValue(func() map[uint32]ir.IRBlock {
	result := map[uint32]ir.IRBlock{}
	for state := range JavaCompatibleStates() {
		result[state.Hash()] = state
	}
	return result
})

func BlockToHash(block ir.IRBlock) uint32 { return block.Tidy().Hash() }
