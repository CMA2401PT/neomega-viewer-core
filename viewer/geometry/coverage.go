// Package geometry describes block-local shapes and their static coverage.
package geometry

import (
	"math"

	"neomega-viewer-core/viewer/geometry/material"
)

// Epsilon accepts floating-point noise from model rotations.
const Epsilon = 1e-9

func Close(a, b float64) bool { return math.Abs(a-b) <= Epsilon }

// Rect is an axis-aligned rectangle in the face's local coordinate plane.
type Rect struct{ Min, Max [2]float64 }

// Rectangle records the actual geometry of a quad, independent of UV mapping.
type Rectangle struct {
	Rect
	Axis, A, B int
	Plane      float64
	Sign       int
	Corners    [4][2]int
}

func (r Rectangle) Side() int {
	if r.Sign < 0 && Close(r.Plane, 0) {
		return r.Axis * 2
	}
	if r.Sign > 0 && Close(r.Plane, 1) {
		return r.Axis*2 + 1
	}
	return -1
}

var sideNames = [6]string{"west", "east", "down", "up", "north", "south"}

// InspectRectangle accepts planar, axis-aligned rectangles with cyclic corners.
func InspectRectangle(positions [4]Vec3) (Rectangle, bool) {
	r := Rectangle{Axis: -1}
	lo, hi := positions[0], positions[0]
	for _, p := range positions {
		for axis := range p {
			if math.IsNaN(p[axis]) || math.IsInf(p[axis], 0) {
				return r, false
			}
			lo[axis], hi[axis] = min(lo[axis], p[axis]), max(hi[axis], p[axis])
		}
	}
	for axis := range lo {
		if Close(lo[axis], hi[axis]) {
			if r.Axis >= 0 {
				return r, false
			}
			r.Axis = axis
		}
	}
	if r.Axis < 0 {
		return r, false
	}
	// Cyclic axes ensure the canonical (a,b) orientation has positive normal.
	r.A, r.B = (r.Axis+1)%3, (r.Axis+2)%3
	r.Plane = positions[0][r.Axis]
	r.Rect = Rect{[2]float64{lo[r.A], lo[r.B]}, [2]float64{hi[r.A], hi[r.B]}}
	var seen [2][2]bool
	for i, p := range positions {
		for component, axis := range [2]int{r.A, r.B} {
			switch {
			case Close(p[axis], lo[axis]):
				r.Corners[i][component] = 0
			case Close(p[axis], hi[axis]):
				r.Corners[i][component] = 1
			default:
				return r, false
			}
		}
		corner := r.Corners[i]
		if seen[corner[0]][corner[1]] {
			return r, false
		}
		seen[corner[0]][corner[1]] = true
	}
	for i, corner := range r.Corners {
		next := r.Corners[(i+1)%4]
		if (corner[0] == next[0]) == (corner[1] == next[1]) {
			return r, false // diagonal edges / bow-tie ordering
		}
	}
	p, q, s := r.Corners[0], r.Corners[1], r.Corners[2]
	r.Sign = (q[0]-p[0])*(s[1]-p[1]) - (q[1]-p[1])*(s[0]-p[0])
	return r, true
}

// OpaqueBoundary checks whether opaque rectangles jointly cover one whole unit face.
// Parts must be combined before checking; CullFace is not used for solidity.
func OpaqueBoundary(quads []Quad, side string) bool {
	sideIndex := -1
	for i, name := range sideNames {
		if name == side {
			sideIndex = i
			break
		}
	}
	if sideIndex < 0 {
		return false
	}
	remaining := []Rect{{Max: [2]float64{1, 1}}}
	for _, quad := range quads {
		if !material.FullyOpaque(quad.Surface) {
			continue
		}
		r, ok := InspectRectangle(quad.Positions)
		if !ok || r.Side() != sideIndex {
			continue
		}
		var after []Rect
		for _, piece := range remaining {
			after = append(after, Subtract(piece, r.Rect)...)
		}
		remaining = after
		if len(remaining) == 0 {
			return true
		}
	}
	return false
}

// Subtract returns disjoint rectangles covering source minus cover.
func Subtract(source, cover Rect) []Rect {
	lo := [2]float64{max(source.Min[0], cover.Min[0]), max(source.Min[1], cover.Min[1])}
	hi := [2]float64{min(source.Max[0], cover.Max[0]), min(source.Max[1], cover.Max[1])}
	if hi[0]-lo[0] <= Epsilon || hi[1]-lo[1] <= Epsilon {
		return []Rect{source}
	}
	pieces := []Rect{
		{source.Min, [2]float64{lo[0], source.Max[1]}},
		{[2]float64{hi[0], source.Min[1]}, source.Max},
		{[2]float64{lo[0], source.Min[1]}, [2]float64{hi[0], lo[1]}},
		{[2]float64{lo[0], hi[1]}, [2]float64{hi[0], source.Max[1]}},
	}
	result := make([]Rect, 0, 4)
	for _, piece := range pieces {
		if piece.Max[0]-piece.Min[0] > Epsilon && piece.Max[1]-piece.Min[1] > Epsilon {
			result = append(result, piece)
		}
	}
	return result
}

func BoundaryCoverage(quads []Quad) [6][]BoundaryRect {
	// 建表与现有剔面共用矩形识别，避免旋转误差和局部覆盖的判定分叉。
	var result [6][]BoundaryRect
	for index, quad := range quads {
		rect, ok := InspectRectangle(quad.Positions)
		if !ok {
			continue
		}
		if side := rect.Side(); side >= 0 {
			result[side] = append(result[side], BoundaryRect{
				Quad: index,
				Min:  Vec2(rect.Min),
				Max:  Vec2(rect.Max),
			})
		}
	}
	return result
}
