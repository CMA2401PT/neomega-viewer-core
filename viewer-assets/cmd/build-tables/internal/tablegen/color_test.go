package tablegen

import (
	"image"
	"image/color"
	"testing"

	"neomega-viewer-core/viewer/geometry"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
	"neomega-viewer-core/viewer/geometry/material"
)

func TestAverageTopColor(t *testing.T) {
	// 内存图像独立验证采样规则，避免资源更换后掩盖算法错误。
	quad := geometry.Quad{
		Positions: [4]geometry.Vec3{{0, 1, 0}, {1, 1, 0}, {1, 1, 1}, {0, 1, 1}},
		Normal:    geometry.Vec3{0, 1, 0},
		UV:        [4]geometry.Vec2{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
		Color:     [3]float32{1, 1, 1},
		Surface:   material.Surface{Texture: "test", Region: [4]float32{0, 0, 1, 1}, Frames: 1},
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 21, G: 31, B: 41, A: 128})
	img.SetNRGBA(0, 1, color.NRGBA{R: 31, G: 42, B: 53, A: 255})
	img.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, B: 255})
	t.Run("visible pixel mean", func(t *testing.T) {
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{quad}}}, img)
		// 三个可见像素的均值为 20.666…、31、41.333…，透明白像素不参与。
		if !ok || got != 0x151f29 {
			t.Fatalf("got %#06x, %v; want 0x151f29, true", got, ok)
		}
	})
	t.Run("linear tint", func(t *testing.T) {
		tinted := quad
		tinted.Color = jmodel.Linear(0x91bd59)
		for channel, want := range []uint8{0x91, 0xbd, 0x59} {
			if got := srgb8(tinted.Color[channel]); got != want {
				t.Fatalf("tint channel %d: got %x, want %x", channel, got, want)
			}
		}
		white := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		white.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{tinted}}}, white)
		if !ok || got != 0x90bc58 {
			t.Fatalf("got %#06x, %v; want 0x90bc58, true", got, ok)
		}
	})
	t.Run("first animation frame", func(t *testing.T) {
		animated := quad
		animated.Surface.Frames = 2
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{animated}}}, img)
		if !ok || got != 0x101a24 {
			t.Fatalf("got %#06x, %v; want 0x101a24, true", got, ok)
		}
	})
	t.Run("unspecified animation frames", func(t *testing.T) {
		static := quad
		static.Surface.Frames = 0
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{static}}}, img)
		if !ok || got != 0x151f29 {
			t.Fatalf("got %#06x, %v; want 0x151f29, true", got, ok)
		}
	})
	t.Run("vertical fallback", func(t *testing.T) {
		vertical := quad
		vertical.Normal = geometry.Vec3{1, 0, 0}
		vertical.Positions = [4]geometry.Vec3{{1, 0, 0}, {1, 1, 0}, {1, 1, 1}, {1, 0, 1}}
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{vertical}}}, img)
		if !ok || got != 0x151f29 {
			t.Fatalf("got %#06x, %v; want 0x151f29, true", got, ok)
		}
	})
	t.Run("top area weighting", func(t *testing.T) {
		redBlue := image.NewNRGBA(image.Rect(0, 0, 2, 1))
		redBlue.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
		redBlue.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})
		red, blue := quad, quad
		red.Surface.Region = [4]float32{0, 0, 0.5, 1}
		blue.Surface.Region = [4]float32{0.5, 0, 0.5, 1}
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{red, blue}}}, redBlue)
		if !ok || got != 0x800080 {
			t.Fatalf("equal areas: got %#06x, %v; want 0x800080, true", got, ok)
		}
		blue.Positions[1][0] = 0.5
		blue.Positions[2][0] = 0.5
		got, ok = averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{red, blue}}}, redBlue)
		if !ok || got != 0xaa0055 {
			t.Fatalf("unequal areas: got %#06x, %v; want 0xaa0055, true", got, ok)
		}
		// 有顶面时，竖直面的颜色不能影响结果。
		blue.Normal = geometry.Vec3{1, 0, 0}
		got, ok = averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{red, blue}}}, redBlue)
		if !ok || got != 0xff0000 {
			t.Fatalf("top only: got %#06x, %v; want 0xff0000, true", got, ok)
		}
		red.Normal = geometry.Vec3{1, 0, 0}
		duplicate := red
		duplicate.UV = [4]geometry.Vec2{{1, 1}, {0, 1}, {0, 0}, {1, 0}}
		got, ok = averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{red, duplicate, blue}}}, redBlue)
		if !ok || got != 0x800080 {
			t.Fatalf("fallback deduplication: got %#06x, %v; want 0x800080, true", got, ok)
		}
	})
	t.Run("UV rectangle and image bounds", func(t *testing.T) {
		cropped := quad
		cropped.UV = [4]geometry.Vec2{{0.5, 0}, {1.5, 0}, {1.5, 0.5}, {0.5, 0.5}}
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{cropped}}}, img)
		if !ok || got != 0x151f29 {
			t.Fatalf("got %#06x, %v; want 0x151f29, true", got, ok)
		}
	})
	t.Run("premultiplied image", func(t *testing.T) {
		rgba := image.NewRGBA(image.Rect(3, 4, 4, 5))
		rgba.SetRGBA(3, 4, color.RGBA{R: 64, G: 32, B: 16, A: 128})
		got, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{quad}}}, rgba)
		if !ok || got != 0x7f3f1f {
			t.Fatalf("got %#06x, %v; want 0x7f3f1f, true", got, ok)
		}
	})
	t.Run("no contribution", func(t *testing.T) {
		if _, ok := averageTopColor(nil, img); ok {
			t.Fatal("empty meshes have a color")
		}
		transparent := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		if _, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{quad}}}, transparent); ok {
			t.Fatal("transparent texture has a color")
		}
		empty := quad
		empty.Surface.Region = [4]float32{2, 2, 1, 1}
		if _, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{empty}}}, img); ok {
			t.Fatal("empty sample rectangle has a color")
		}
		empty = quad
		empty.UV = [4]geometry.Vec2{{0.25, 0}, {0.25, 0}, {0.25, 1}, {0.25, 1}}
		if _, ok := averageTopColor([]geometry.Mesh{{Quads: []geometry.Quad{empty}}}, img); ok {
			t.Fatal("zero-width UV rectangle has a color")
		}
	})
}
