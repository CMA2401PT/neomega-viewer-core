package asset

import model "neomega-blocks/geometry/java_block_model"

type ModelIndex struct {
	// Empty geometry can be intentional (air/fluids) or require a special renderer.
	GeometrylessBlockStates []model.ResourceID `json:"geometrylessBlockStates"`
	Version                 string             `json:"version"`
	RequiredTextures        []model.ResourceID `json:"requiredTextures"`
	BuiltinModels           []model.ResourceID `json:"builtinModels"`
	ModelCount              int                `json:"modelCount"`
	BlockStateCount         int                `json:"blockStateCount"`
}

type BuildInfo struct {
	GeometrylessBlockStates []model.ResourceID         `json:"geometrylessBlockStates"`
	RendererCounts          map[model.RendererKind]int `json:"rendererCounts"`
	Version                 string                     `json:"version"`
	Sources                 []Source                   `json:"sources"`
	BlockStateCount         int                        `json:"blockStateCount"`
	ModelCount              int                        `json:"modelCount"`
	TextureCount            int                        `json:"textureCount"`
	BuiltinModels           []model.ResourceID         `json:"builtinModels"`
}
