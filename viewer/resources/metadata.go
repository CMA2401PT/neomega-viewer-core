package geometry

import (
	"neomega-blocks/ir"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
)

// SkippedState records a canonical Java IR block with no generated static shape.
type SkippedState struct {
	Block  ir.IRBlock
	Reason string
}

type Renderer struct {
	Kind   model.RendererKind `json:"kind"`
	Family string             `json:"family,omitempty"`
}

// Metadata describes the Java resource version and static geometry coverage.
type Metadata struct {
	Version   string
	Renderers map[string]Renderer
	Skipped   []SkippedState
}
