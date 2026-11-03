package item_render_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"neomega-viewer-core/viewer/geometry"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
	"neomega-viewer-core/viewer/item_render"
	preset "neomega-viewer-core/viewer/resources"
)

func TestGeneratedFollowsAlphaOutline(t *testing.T) {
	// 两个分离像素需要各自的四侧，不能用整张图片的矩形边界替代。
	pixels := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	pixels.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 255})
	pixels.SetNRGBA(2, 2, color.NRGBA{255, 255, 255, 255})
	atlas := model.Atlas{Width: 3, Height: 3, Textures: map[model.ResourceID]model.AtlasTexture{"example:sprite": {SU: 1, SV: 1, Frames: 1}}}
	resolved := &model.Resolved{Builtin: "minecraft:builtin/generated", Textures: map[string]model.TextureBinding{"layer0": {Sprite: "example:sprite"}}}
	mesh, err := item_render.Bake(resolved, atlas, pixels)
	if err != nil {
		t.Fatal(err)
	}
	if len(mesh.Quads) != 10 {
		t.Fatalf("expected front/back and eight outline sides, got %d", len(mesh.Quads))
	}
	sides := 0
	for _, q := range mesh.Quads {
		if q.Normal[2] == 0 {
			sides++
		}
		for _, p := range q.Positions {
			if math.Abs(math.Abs(p[2])-1.0/32) > 1e-10 {
				t.Fatalf("wrong generated thickness: %v", p)
			}
		}
	}
	if sides != 8 {
		t.Fatalf("outline sides: %d", sides)
	}
}

func TestInheritedFixedAppliedOnceAndRenderCopies(t *testing.T) {
	// 子模型继承 fixed；生成期应用一次后，运行时只复制，不能重复缩放或旋转。
	elements := []model.Element{{From: model.Vec3{0, 0, 0}, To: model.Vec3{16, 16, 16}, Faces: map[model.Direction]model.Face{"south": {Texture: "example:white"}}}}
	models := model.Models{
		"example:parent": {Elements: &elements, Display: map[string]model.Transform{"fixed": {Scale: &model.Vec3{.5, .25, 1}, Translation: &model.Vec3{4, 0, 0}, Rotation: &model.Vec3{0, 0, 90}}}},
		"example:child":  {Parent: "example:parent"},
	}
	resolved, err := model.NewResolver(models).Resolve("example:child")
	if err != nil {
		t.Fatal(err)
	}
	pixels := image.NewUniform(color.White)
	atlas := model.Atlas{Width: 1, Height: 1, Textures: map[model.ResourceID]model.AtlasTexture{"example:white": {SU: 1, SV: 1, Frames: 1}}}
	mesh, err := item_render.Bake(resolved, atlas, pixels)
	if err != nil {
		t.Fatal(err)
	}
	catalog := &item_render.Catalog{Items: map[string]geometry.Mesh{"example:item": mesh}}
	renderer := item_render.New(catalog)
	for range 2 {
		result, err := renderer.Render("example:item")
		if err != nil {
			t.Fatal(err)
		}
		lo, hi := geometry.Vec3{1, 1, 1}, geometry.Vec3{-1, -1, -1}
		for _, p := range result.Quads[0].Positions {
			for axis := range p {
				lo[axis] = min(lo[axis], p[axis])
				hi[axis] = max(hi[axis], p[axis])
			}
		}
		for axis, want := range (geometry.Vec3{.125, -.25, .5}) {
			if math.Abs(lo[axis]-want) > 1e-10 {
				t.Fatalf("fixed lower bound: %v", lo)
			}
		}
		for axis, want := range (geometry.Vec3{.375, .25, .5}) {
			if math.Abs(hi[axis]-want) > 1e-10 {
				t.Fatalf("fixed upper bound: %v", hi)
			}
		}
		result.Quads[0].Positions[0] = geometry.Vec3{99, 99, 99}
	}
}

func TestBundledOrdinaryItems(t *testing.T) {
	// 验收物品属于资源行为断言，不限制生成器可处理的物品集合。
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	renderer := item_render.New(resources.Items)
	sword, err := renderer.Render("minecraft:diamond_sword")
	if err != nil {
		t.Fatal(err)
	}
	sides := 0
	for _, q := range sword.Quads {
		if q.Normal[2] == 0 {
			sides++
		}
	}
	if sides == 0 {
		t.Fatal("diamond sword has no extruded sides")
	}
	lantern, err := renderer.Render("minecraft:sea_lantern")
	if err != nil {
		t.Fatal(err)
	}
	if len(lantern.Quads) != 6 {
		t.Fatalf("sea lantern faces: %d", len(lantern.Quads))
	}
	for _, q := range lantern.Quads {
		if q.Surface.Frames != 5 || q.Surface.FrameTime != 5 {
			t.Fatalf("sea lantern lost animation metadata: %+v", q.Surface)
		}
	}
}
