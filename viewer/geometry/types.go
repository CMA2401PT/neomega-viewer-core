// There are no neighbor queries, chunk coordinates or workers in this package.
package geometry

import "neomega-viewer-core/viewer/geometry/material"

type Vec3 [3]float64
type Vec2 [2]float64

type CullFace uint8

const (
	// 零值表示不可剔除，六方向编号与 Shape.Boundary 对齐。
	NoCullFace CullFace = iota
	West
	East
	Down
	Up
	North
	South
)

type Quad struct {
	Positions [4]Vec3
	Normal    Vec3
	// ShadeNormal overrides lighting only; zero uses the geometric normal.
	ShadeNormal Vec3
	UV          [4]Vec2
	Color       [3]float32
	Surface     material.Surface
	// CullFace is the rotated neighbor direction from the source model. An
	// isolated preview retains the face; a section mesher may later examine it.
	CullFace CullFace
}

type Bounds struct {
	Min Vec3 `json:"min"`
	Max Vec3 `json:"max"`
}

type Mesh struct{ Quads []Quad }
