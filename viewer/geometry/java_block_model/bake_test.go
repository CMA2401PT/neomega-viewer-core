package java_block_model

import (
	"image"
	"image/color"
	"math"
	"testing"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/geometry/material"
)

func testMaterials() *MaterialLibrary {
	pixels := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			pixels.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	return NewMaterials(Atlas{Width: 16, Height: 16, Textures: map[ResourceID]AtlasTexture{"minecraft:block/stone": {SU: 1, SV: 1}}}, pixels, testPolicy{})
}

func close(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestCubeSlabNormalsAndDefaultUV(t *testing.T) {
	faces := map[Direction]Face{}
	for _, side := range directions {
		faces[Direction(side)] = Face{Texture: "block/stone"}
	}
	source := &Resolved{Elements: []Element{{From: Vec3{}, To: Vec3{16, 8, 16}, Faces: faces}}}
	mesh, err := Bake(source, Apply{}, testMaterials().ForBlock("minecraft:stone []"))
	if err != nil {
		t.Fatal(err)
	}
	if len(mesh.Quads) != 6 {
		t.Fatalf("slab has %d faces", len(mesh.Quads))
	}
	var maxY float64
	for _, quad := range mesh.Quads {
		for _, vertex := range quad.Positions {
			maxY = max(maxY, vertex[1])
		}
	}
	if maxY != 0.5 {
		t.Fatalf("slab height = %v", maxY)
	}
	for _, quad := range mesh.Quads {
		normal := unit(cross(sub(quad.Positions[1], quad.Positions[0]), sub(quad.Positions[2], quad.Positions[0])))
		if dot(normal, quad.Normal) < 0.999 {
			t.Fatal("winding and normal disagree")
		}
		if quad.Normal[0] == 1 {
			for _, uv := range quad.UV {
				if uv[1] < 0.5 || uv[1] > 1 {
					t.Fatalf("slab's default side UV ignored height: %v", uv)
				}
			}
		}
	}
}

func TestVariantRotationAndUVLock(t *testing.T) {
	source := &Resolved{Elements: []Element{{From: Vec3{}, To: Vec3{16, 16, 16}, Faces: map[Direction]Face{
		"north": {Texture: "block/stone", CullFace: "north"}, "up": {Texture: "block/stone"},
	}}}}
	mesh, err := Bake(source, Apply{Y: 90, UVLock: true}, testMaterials().ForBlock("minecraft:stone []"))
	if err != nil {
		t.Fatal(err)
	}
	for _, quad := range mesh.Quads {
		if quad.CullFace != geometry.NoCullFace {
			if quad.CullFace != geometry.East || !close(quad.Normal[0], 1) {
				t.Fatalf("north did not rotate east: %+v", quad)
			}
			for _, p := range quad.Positions {
				if !close(p[0], 1) {
					t.Fatal("rotation origin is not block center")
				}
			}
		} else {
			for i, p := range quad.Positions {
				if !close(quad.UV[i][0], p[0]) || !close(quad.UV[i][1], p[2]) {
					t.Fatalf("UV lock rotated the top texture: %v %v", p, quad.UV[i])
				}
			}
		}
	}
}

func TestElementRotationProducesAPlaneNormal(t *testing.T) {
	source := &Resolved{Elements: []Element{{From: Vec3{0, 0, 8}, To: Vec3{16, 16, 8}, ShadeDirectionOverride: "up", Rotation: &ElementRotation{Origin: Vec3{8, 8, 8}, Axis: "y", Angle: 45}, Faces: map[Direction]Face{"north": {Texture: "block/stone"}}}}}
	mesh, err := Bake(source, Apply{}, testMaterials().ForBlock("minecraft:stone []"))
	if err != nil || len(mesh.Quads) != 1 {
		t.Fatalf("cross plane missing: %v", err)
	}
	if mesh.Quads[0].ShadeNormal != (geometry.Vec3{0, 1, 0}) {
		t.Fatal("lighting override was lost")
	}
	n := mesh.Quads[0].Normal
	if !close(math.Abs(n[0]), math.Sqrt(0.5)) || !close(math.Abs(n[2]), math.Sqrt(0.5)) {
		t.Fatalf("normal ignored element rotation: %v", n)
	}
}

func TestCutoutVolumeFacesAndStandalonePlanesHaveDifferentSidedness(t *testing.T) {
	pixels := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	library := NewMaterials(Atlas{Width: 2, Height: 1, Textures: map[ResourceID]AtlasTexture{"minecraft:block/stone": {SU: 1, SV: 1}}}, pixels, testPolicy{})
	for _, check := range []struct {
		To     Vec3
		Faces  map[Direction]Face
		Double bool
	}{
		{Vec3{16, 16, 16}, map[Direction]Face{"north": {Texture: "block/stone"}}, false},
		{Vec3{16, 16, 0}, map[Direction]Face{"north": {Texture: "block/stone"}}, true},
		{Vec3{16, 16, 0}, map[Direction]Face{"north": {Texture: "block/stone"}, "south": {Texture: "block/stone"}}, false},
	} {
		source := &Resolved{Elements: []Element{{To: check.To, Faces: check.Faces}}}
		mesh, err := Bake(source, Apply{}, library.ForBlock("minecraft:oak_leaves []"))
		if err != nil {
			t.Fatal(err)
		}
		for _, quad := range mesh.Quads {
			if quad.Surface.Mode != material.Cutout || quad.Surface.DoubleSided != check.Double {
				t.Fatalf("incorrect cutout sidedness: %+v", quad.Surface)
			}
		}
	}
}

// testPolicy exercises the material interface without selecting a preset.
type testPolicy struct{}

func (testPolicy) Mode(state ir.IRBlock, sampled material.Mode) material.Mode {
	if state.ShortName() == "glass" {
		return material.Blend
	}
	return sampled
}
func (testPolicy) Tint(ir.IRBlock, int) [3]float32 { return [3]float32{1, 1, 1} }
func (testPolicy) Opacity(ir.IRBlock) float32      { return 1 }
