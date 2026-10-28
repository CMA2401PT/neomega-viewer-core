// SPDX-License-Identifier: MIT
package s04_atlas

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
