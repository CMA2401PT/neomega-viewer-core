// SPDX-License-Identifier: MIT
package s05_atlas

import (
	"image"
	"neomega-assets/asset"
	"testing"
)

func TestAnimationGridCoordinates(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	metadata := asset.TextureMetadata{Animation: &asset.Animation{Width: 32, Height: 16, Frames: []asset.Frame{{Index: 3}}}}
	got, err := FirstFrame(img, metadata)
	if err != nil || got != image.Rect(32, 16, 64, 32) {
		t.Fatalf("first frame = %v, %v", got, err)
	}
	metadata.Animation.Frames[0].Index = 8
	if _, err := FirstFrame(img, metadata); err == nil {
		t.Fatal("out-of-range animation frame accepted")
	}
}

func TestPackReusesSpaceBelowShortRectangles(t *testing.T) {
	// These exactly fill a 64x64 image; row-only packing wastes the space under b.
	packed, size, err := pack([]rectangle{{name: "a", width: 32, height: 64}, {name: "b", width: 32, height: 32}, {name: "c", width: 32, height: 32}}, 16)
	if err != nil || size != image.Pt(64, 64) {
		t.Fatalf("packed size %v: %v", size, err)
	}
	for i, a := range packed {
		ra := image.Rectangle{Min: a.position, Max: a.position.Add(image.Pt(a.width, a.height))}
		if !ra.In(image.Rect(0, 0, size.X, size.Y)) {
			t.Fatal("out of bounds")
		}
		for _, b := range packed[:i] {
			rb := image.Rectangle{Min: b.position, Max: b.position.Add(image.Pt(b.width, b.height))}
			if ra.Overlaps(rb) {
				t.Fatal("overlapping textures")
			}
		}
	}
}

func TestPackUsesIndependentPowerOfTwoDimensions(t *testing.T) {
	// Seventeen tiles exceed 64x64, but fit 128x64 without a 128x128 square.
	input := make([]rectangle, 17)
	for i := range input {
		input[i] = rectangle{width: 16, height: 16}
	}
	packed, size, err := pack(input, 16)
	if err != nil || size != image.Pt(128, 64) {
		t.Fatalf("size=%v error=%v", size, err)
	}
	for i, rect := range packed {
		bounds := image.Rectangle{Min: rect.position, Max: rect.position.Add(image.Pt(rect.width, rect.height))}
		if !bounds.In(image.Rectangle{Max: size}) {
			t.Fatal("outside rectangular atlas")
		}
		for _, previous := range packed[:i] {
			if bounds.Overlaps(image.Rectangle{Min: previous.position, Max: previous.position.Add(image.Pt(previous.width, previous.height))}) {
				t.Fatal("overlapping textures")
			}
		}
	}
	_, size, err = pack([]rectangle{{width: 16, height: 128}}, 16)
	if err != nil || size != image.Pt(16, 128) {
		t.Fatalf("vertical strip: size=%v error=%v", size, err)
	}
}
