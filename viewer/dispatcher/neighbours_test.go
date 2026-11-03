package dispatcher_test

import (
	"context"
	"fmt"
	"testing"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/effector"
	"neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/geometry/material"
)

// 完全浸没的区域不应产生任何水面，所有接触面和上方邻域都由批量入口读取。
func TestBuildSubmergedSections(t *testing.T) {
	resources := tables(t)
	for _, size := range []int{1, 3, 16} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			layout := cube.New(size)
			pipeline := dispatcher.NewPipeline(layout, resources, effector.New(nil), blockactor.New(), nil, nil)
			water := &dispatcher.Section{Blocks: blockdata.Uniform(layout.Len(), ir.IRBlock(`minecraft:water ["level"="0"]`))}
			s := &source{
				sections: map[dispatcher.SectionPos]*dispatcher.Section{},
				calls:    map[dispatcher.SectionPos]int{},
			}
			// 两个连续区域共享大部分邻域，覆盖读取缓存与负坐标换算。
			positions := []dispatcher.SectionPos{{X: -2, Y: -3, Z: 4}, {X: -2, Y: -2, Z: 4}}
			for y := int32(-4); y <= 0; y++ {
				for _, horizontal := range [][2]int32{{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					s.sections[dispatcher.SectionPos{X: -2 + horizontal[0], Y: y, Z: 4 + horizontal[1]}] = water
				}
			}
			results, err := pipeline.BuildSections(context.Background(), s, positions, dispatcher.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(positions) {
				t.Fatalf("返回了未请求的区域：%d", len(results))
			}
			for i, result := range results {
				if result.Pos != positions[i] || len(result.Result.Frame.Mesh.Quads) != 0 {
					t.Fatalf("水下区域 %v 残留 %d 个面", result.Pos, len(result.Result.Frame.Mesh.Quads))
				}
			}
			wantReads := 16 // 两个中心、上下端点，以及三层高度的四根水平邻柱。
			if size == 1 {
				wantReads++
			}
			if len(s.calls) != wantReads {
				t.Fatalf("读取了 %d 个区域，期望 %d", len(s.calls), wantReads)
			}
			for pos, count := range s.calls {
				if count != 1 {
					t.Fatalf("区域 %v 重复读取了 %d 次", pos, count)
				}
			}
		})
	}
}

// 一格区域的上邻居形状取决于再上一格，自定义满高形状的底面也必须正确参与剔面。
func TestSingleCellUpperNeighbourHeight(t *testing.T) {
	top := geometry.Quad{
		Positions: [4]geometry.Vec3{{0, 1, 0}, {0, 1, 1}, {1, 1, 1}, {1, 1, 0}},
		UV:        [4]geometry.Vec2{{0, 0}, {0, 1}, {1, 1}, {1, 0}},
		Normal:    geometry.Vec3{0, 1, 0},
		CullFace:  geometry.Up,
		Surface:   material.Surface{Mode: material.Blend},
	}
	bottom := geometry.Quad{
		Positions: [4]geometry.Vec3{{0, 0, 0}, {1, 0, 0}, {1, 0, 1}, {0, 0, 1}},
		Normal:    geometry.Vec3{0, -1, 0},
		Surface:   material.Surface{Mode: material.Blend},
	}
	partial := bottom
	for i := range partial.Positions {
		partial.Positions[i][0] *= .5
	}
	centerState, fluidState := ir.IRBlock("example:surface []"), ir.IRBlock("example:fluid []")
	resources := &geometry.Tables[uint32]{
		Index: map[uint32][]geometry.ModelID{centerState.Hash(): {0}, fluidState.Hash(): {1}},
		Shapes: []geometry.Shape{
			{Quads: []geometry.Quad{top}, CullGroup: 7},
			{Quads: []geometry.Quad{partial}, CullGroup: 7},
			{Quads: []geometry.Quad{bottom}, CullGroup: 7},
		},
		FullHeight: map[geometry.ModelID]geometry.ModelID{1: 2},
	}
	for i := range resources.Shapes {
		resources.Shapes[i].Boundary = geometry.BoundaryCoverage(resources.Shapes[i].Quads)
	}
	pipeline := dispatcher.NewPipeline(cube.New(1), resources, effector.New(nil), blockactor.New(), nil, nil)
	center := dispatcher.Section{Blocks: blockdata.Uniform(1, centerState)}
	fluid := dispatcher.Section{Blocks: blockdata.Uniform(1, fluidState)}
	for _, connected := range []bool{false, true} {
		t.Run(fmt.Sprint(connected), func(t *testing.T) {
			neighbours := dispatcher.Neighbours{{0, 1, 0}: &fluid}
			s := &source{
				sections: map[dispatcher.SectionPos]*dispatcher.Section{{}: &center, {Y: 1}: &fluid},
				calls:    map[dispatcher.SectionPos]int{},
			}
			if connected {
				neighbours[[3]int{0, 2, 0}] = &fluid
				s.sections[dispatcher.SectionPos{Y: 2}] = &fluid
			}
			single, err := pipeline.Build(center, neighbours, dispatcher.Options{})
			if err != nil {
				t.Fatal(err)
			}
			batch, err := pipeline.BuildSections(context.Background(), s, []dispatcher.SectionPos{{}}, dispatcher.Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, result := range []*dispatcher.Result{single, batch[0].Result} {
				want := 1
				if connected {
					want = 0
				}
				if len(result.Frame.Mesh.Quads) != want {
					t.Fatalf("上邻居覆盖错误：connected=%t，残留 %d 个面", connected, len(result.Frame.Mesh.Quads))
				}
				if !connected {
					for _, point := range result.Frame.Mesh.Quads[0].Positions {
						if point[0] < .5 {
							t.Fatalf("部分覆盖未剔除：%v", point)
						}
					}
				}
			}
		})
	}
}
