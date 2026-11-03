package main

import (
	"reflect"
	"strings"
	"testing"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/geometry/material"
	"neomega-viewer-core/viewer/javapreset"
	preset "neomega-viewer-core/viewer/resources"
)

// 检查四种输入都能使用真实预置资源构建，并保留各自的渲染特征。
func TestSingleBlockScenes(t *testing.T) {
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := javapreset.NewPipeline(cube.New(1), resources.Tables, resources.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	paddedPipeline, err := javapreset.NewPipeline(cube.New(16), resources.Tables, resources.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"waterlogged_leaves", "fire", "enchanting_table", "decorated_pot"} {
		t.Run(name, func(t *testing.T) {
			scene := scenes[name]
			blocks := blockdata.Uniform(1, scene.State.Tidy())
			result, err := pipeline.Build(
				dispatcher.Section{
					Blocks:        blocks,
					BlockEntities: map[dispatcher.CellPos]nbt.Compound{{0, 0, 0}: scene.NBT},
				},
				dispatcher.Neighbours{},
				dispatcher.Options{},
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Diagnostics) != 0 {
				t.Fatalf("存在渲染诊断：%v", result.Diagnostics)
			}
			// 与旧的 16³ 空气填充输入比较完整产物，保证尺寸变化不改变单格外观。
			padded := blockdata.Uniform(4096, ir.IRBlock("minecraft:air []"))
			padded.Set(0, scene.State.Tidy())
			legacy, err := paddedPipeline.Build(
				dispatcher.Section{Blocks: padded, BlockEntities: map[dispatcher.CellPos]nbt.Compound{{}: scene.NBT}},
				dispatcher.Neighbours{},
				dispatcher.Options{},
			)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Frame, legacy.Frame) {
				t.Fatal("1³ 与 16³ 中的同一个独立方块外观不同")
			}
			frame := result.Frame
			if name == "decorated_pot" {
				if len(frame.Mesh.Quads) != 0 || len(frame.Actors) != 1 || len(frame.Effects) != 0 {
					t.Fatal("陶罐应只输出 actor，不应重复绘制静态本体")
				}
				if frame.Actors[0].Pos != ([3]uint8{}) {
					t.Fatal("陶罐不在原点")
				}
				patterns := map[string]bool{}
				for _, quad := range frame.Actors[0].Mesh.Quads {
					if strings.HasSuffix(quad.Surface.Texture, "_pottery_pattern") {
						patterns[quad.Surface.Texture] = true
					}
				}
				if len(patterns) != 4 {
					t.Fatalf("应有四种陶片纹样，实际 %v", patterns)
				}
				// 含水陶罐在同一个格子中组合 actor 与水层，不重复静态罐体。
				wet := scene.State.States().Set("waterlogged", "true")
				wetState := ir.IRBlock(scene.State.NameWithNameSpace() + " " + string(wet)).Tidy()
				wetResult, err := pipeline.Build(
					dispatcher.Section{Blocks: blockdata.Uniform(1, wetState), BlockEntities: map[dispatcher.CellPos]nbt.Compound{{}: scene.NBT}},
					dispatcher.Neighbours{},
					dispatcher.Options{},
				)
				if err != nil {
					t.Fatal(err)
				}
				if len(wetResult.Diagnostics) != 0 || len(wetResult.Frame.Actors) != 1 || len(wetResult.Frame.Mesh.Quads) == 0 {
					t.Fatal("含水陶罐没有同时产生水和 actor")
				}
				for _, quad := range wetResult.Frame.Mesh.Quads {
					if quad.Surface.Mode != material.Blend || !strings.HasPrefix(quad.Surface.Texture, "minecraft:block/water_") {
						t.Fatalf("含水陶罐的静态层出现非水材质：%+v", quad.Surface)
					}
				}
				return
			}
			if result.Stats.Blocks != 1 || len(frame.Actors) != 0 {
				t.Fatal("应只构建原点的一个普通方块")
			}
			if name == "enchanting_table" {
				if len(frame.Effects) != 1 || frame.Effects[0].Pos != ([3]uint8{}) || frame.Effects[0].Effect != "enchanting_table_book" {
					t.Fatal("附魔台应在原点产生一个书本 effect")
				}
				return
			}
			if len(frame.Effects) != 0 {
				t.Fatal("纹理动画不应产生 effect 消息")
			}
			animated, leaves, water := false, false, false
			for _, quad := range frame.Mesh.Quads {
				animated = animated || quad.Surface.Frames > 1
				leaves = leaves || quad.Surface.Mode == material.Cutout
				if name == "waterlogged_leaves" && quad.Surface.Mode == material.Blend {
					water = true
					for _, point := range quad.Positions {
						if point[0] <= 0 || point[0] >= 1 || point[2] <= 0 || point[2] >= 1 {
							t.Fatal("水层应缩进树叶内部，避免共面闪烁")
						}
					}
				}
			}
			if !animated {
				t.Fatal("水和火焰应保留纹理动画帧")
			}
			if name == "waterlogged_leaves" && (!leaves || !water) {
				t.Fatal("含水树叶应同时包含镂空树叶层和半透明水层")
			}
		})
	}
}
