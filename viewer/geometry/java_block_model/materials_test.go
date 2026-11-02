package java_block_model

import (
	"image"
	"image/color"
	"testing"

	"neomega-viewer-core/viewer/geometry/material"
)

func TestAlphaHolesAndTranslucencyAreDifferent(t *testing.T) {
	pixels := image.NewNRGBA(image.Rect(0, 0, 6, 1))
	for x, alpha := range []uint8{255, 255, 0, 255, 128, 255} {
		pixels.SetNRGBA(x, 0, color.NRGBA{R: 255, A: alpha})
	}
	atlas := Atlas{Width: 6, Height: 1, Textures: map[ResourceID]AtlasTexture{
		"minecraft:block/solid": {U: 0, SU: 2.0 / 6, SV: 1}, "minecraft:block/holes": {U: 2.0 / 6, SU: 2.0 / 6, SV: 1}, "minecraft:block/partial": {U: 4.0 / 6, SU: 2.0 / 6, SV: 1},
	}}
	library := NewMaterials(atlas, pixels, testPolicy{})
	for texture, want := range map[ResourceID]material.Mode{"minecraft:block/solid": material.Opaque, "minecraft:block/holes": material.Cutout, "minecraft:block/partial": material.Blend} {
		surface, err := library.ForBlock("minecraft:stone []").Surface(texture, false, false)
		if err != nil || surface.Mode != want {
			t.Fatalf("%s: %+v %v", texture, surface, err)
		}
	}
	glass, err := library.ForBlock("minecraft:glass []").Surface("minecraft:block/holes", false, false)
	if err != nil || glass.Mode != material.Blend || !glass.DoubleSided {
		t.Fatal("glass lost its transparent surface semantics")
	}
}

func TestAtlasFrameMetadata(t *testing.T) {
	atlas := Atlas{Width: 1, Height: 32, Textures: map[ResourceID]AtlasTexture{
		"minecraft:block/static":   {SU: 1, SV: 1.0 / 32, Frames: 1},
		"minecraft:block/animated": {SU: 1, SV: 1, Frames: 32, FrameTime: 2},
	}}
	library := NewMaterials(atlas, image.NewNRGBA(image.Rect(0, 0, 1, 32)), testPolicy{})
	for name, entry := range atlas.Textures {
		surface, err := library.ForBlock("minecraft:stone []").Surface(name, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if surface.Frames != entry.Frames || surface.FrameTime != entry.FrameTime || surface.Region != [4]float32{0, 0, 1, float32(entry.SV)} {
			t.Fatalf("atlas metadata changed: %+v", surface)
		}
	}
}
