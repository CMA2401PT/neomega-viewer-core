package geometry

import (
	"testing"

	"neomega-viewer-core/viewer/geometry/material"
)

var opaque = material.Surface{Mode: material.Opaque, Opacity: 1}

func face(axis, sign int, plane float64, lo, hi [2]float64, surface material.Surface) Quad {
	q := Quad{Surface: surface, Color: [3]float32{1, 1, 1}}
	q.Normal[axis] = float64(sign)
	a, b := (axis+1)%3, (axis+2)%3
	corners := [4][2]float64{{lo[0], lo[1]}, {hi[0], lo[1]}, {hi[0], hi[1]}, {lo[0], hi[1]}}
	if sign < 0 {
		corners[1], corners[3] = corners[3], corners[1]
	}
	for i, corner := range corners {
		q.Positions[i][axis] = plane
		q.Positions[i][a], q.Positions[i][b] = corner[0], corner[1]
		q.UV[i] = Vec2(corner)
	}
	q.CullFace = CullFace(axis*2 + (sign+1)/2 + 1)
	return q
}

func cuboid(height float64, surface material.Surface) Mesh {
	var mesh Mesh
	hi := Vec3{1, height, 1}
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []int{-1, 1} {
			plane := 0.0
			if sign > 0 {
				plane = hi[axis]
			}
			mesh.Quads = append(mesh.Quads, face(axis, sign, plane, [2]float64{}, [2]float64{hi[(axis+1)%3], hi[(axis+2)%3]}, surface))
		}
	}
	return mesh
}

func TestOpaqueBoundary(t *testing.T) {
	left := face(0, 1, 1, [2]float64{}, [2]float64{.4, 1}, opaque)
	right := face(0, 1, 1, [2]float64{.4, 0}, [2]float64{1, 1}, opaque)
	left.CullFace, right.CullFace = NoCullFace, NoCullFace
	union := Mesh{Quads: []Quad{left, right}}
	if !OpaqueBoundary(union.Quads, "east") || OpaqueBoundary(union.Quads, "west") || OpaqueBoundary(union.Quads, "invalid") {
		t.Fatal("boundary union or requested direction incorrect")
	}
	for _, test := range []struct {
		name string
		mesh Mesh
	}{
		{"missing half", Mesh{Quads: []Quad{left}}},
		{"slab side", cuboid(.5, opaque)},
		{"cutout", cuboid(1, material.Surface{Mode: material.Cutout, Opacity: 1})},
		{"blend", cuboid(1, material.Surface{Mode: material.Blend, Opacity: .5})},
		{"reduced opacity", cuboid(1, material.Surface{Mode: material.Opaque, Opacity: .5})},
		{"hole", Mesh{Quads: []Quad{
			face(0, 1, 1, [2]float64{}, [2]float64{.2, 1}, opaque),
			face(0, 1, 1, [2]float64{.8, 0}, [2]float64{1, 1}, opaque),
			face(0, 1, 1, [2]float64{.2, 0}, [2]float64{.8, .2}, opaque),
			face(0, 1, 1, [2]float64{.2, .8}, [2]float64{.8, 1}, opaque),
		}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if OpaqueBoundary(test.mesh.Quads, "east") {
				t.Fatal("incomplete or nonopaque boundary accepted")
			}
		})
	}
	for _, side := range sideNames {
		if !OpaqueBoundary(cuboid(1, opaque).Quads, side) {
			t.Fatalf("cube missing opaque %s boundary", side)
		}
	}
	if !OpaqueBoundary(cuboid(.5, opaque).Quads, "down") || OpaqueBoundary(cuboid(.5, opaque).Quads, "up") {
		t.Fatal("slab boundary inferred from shape volume instead of face location")
	}
}
