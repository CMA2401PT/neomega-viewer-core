package asset

import jmodel "neomega-blocks/geometry/java_block_model"

type Renderers struct {
	Version string `json:"version"`
	// Shared rendering descriptors are accompanied by producer-only provenance and counts.
	Blocks         map[jmodel.ResourceID]jmodel.BlockRenderer `json:"blocks"`
	FamilyTextures map[string][]jmodel.ResourceID             `json:"familyTextures"`
	Counts         map[jmodel.RendererKind]int                `json:"counts"`
	Sources        []Source                                   `json:"sources"`
}
