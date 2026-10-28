// SPDX-License-Identifier: MIT
package asset

// RendererKind describes the information a consumer needs beyond a model lookup.
type RendererKind string

const (
	RendererModel     RendererKind = "model"
	RendererSpecial   RendererKind = "special"
	RendererFluid     RendererKind = "fluid"
	RendererInvisible RendererKind = "invisible"
	RendererDynamic   RendererKind = "dynamic"
)

// BlockRenderer separates static previews from state-dependent rendering. Static
// previews use the augmented blockStatesModels table; DynamicFields are inputs
// required for the game's animated/customized appearance.
type BlockRenderer struct {
	Kind            RendererKind `json:"kind"`
	Family          string       `json:"family,omitempty"`
	StaticPreview   bool         `json:"staticPreview"`
	StateProperties []string     `json:"stateProperties,omitempty"`
	DynamicFields   []string     `json:"dynamicFields,omitempty"`
	TemplateSource  string       `json:"templateSource,omitempty"`
}

type Renderers struct {
	Version string `json:"version"`
	// Keys are canonical resource IDs, unlike the legacy model-table export names.
	Blocks         map[ID]BlockRenderer `json:"blocks"`
	FamilyTextures map[string][]ID      `json:"familyTextures"`
	Counts         map[RendererKind]int `json:"counts"`
	Sources        []Source             `json:"sources"`
}
