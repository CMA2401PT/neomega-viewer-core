// SPDX-License-Identifier: MIT
package java_block_model

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
// previews use the augmented blockstates and models tables; DynamicFields are inputs
// required for the game's animated/customized appearance.
type BlockRenderer struct {
	Kind            RendererKind `json:"kind"`
	Family          string       `json:"family,omitempty"`
	StaticPreview   bool         `json:"staticPreview"`
	StateProperties []string     `json:"stateProperties,omitempty"`
	DynamicFields   []string     `json:"dynamicFields,omitempty"`
	TemplateSource  string       `json:"templateSource,omitempty"`
}
