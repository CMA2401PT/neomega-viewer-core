package dispatcher_test

import (
	"fmt"
	"testing"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/effector"
	"neomega-viewer-core/viewer/mesher/geometry"
	preset "neomega-viewer-core/viewer/resources"
)

// 非二次幂尺寸能发现残留的位移索引；六个邻居分别检查边界寻址与剔面。
func TestConfiguredSizesAndNeighbours(t *testing.T) {
	resources := tables(t)
	for _, side := range []int{1, 2, 3, 16} {
		t.Run(fmt.Sprint(side), func(t *testing.T) {
			layout := cube.New(side)
			pipeline := dispatcher.NewPipeline(layout, resources, effector.New(nil), blockactor.New(), nil, nil)
			input := dispatcher.Section{Blocks: blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:stone []"))}
			for direction := -1; direction < 6; direction++ {
				var neighbours dispatcher.Neighbours
				if direction >= 0 {
					neighbours[direction] = &input
				}
				result, err := pipeline.Build(input, neighbours, dispatcher.Options{})
				if err != nil {
					t.Fatal(err)
				}
				faces := 6
				if direction >= 0 {
					faces--
				}
				if result.Stats.Blocks != layout.Len() || result.Stats.VisibleQuads != faces*side*side || result.Stats.OutputQuads != faces {
					t.Fatalf("side=%d neighbour=%d stats=%+v", side, direction, result.Stats)
				}
				for _, quad := range result.Frame.Mesh.Quads {
					for _, point := range quad.Positions {
						for _, value := range point {
							if value < 0 || value > float64(side) {
								t.Fatalf("位置越界：%v", point)
							}
						}
					}
				}
			}
			// 数据长度必须服从已创建的构建器，不允许另一尺寸被静默截断。
			wrong := dispatcher.Section{Blocks: blockdata.Uniform(layout.Len()+1, ir.IRBlock("minecraft:stone []"))}
			if _, err := pipeline.Build(wrong, dispatcher.Neighbours{}, dispatcher.Options{}); err == nil {
				t.Fatal("接受了错误尺寸的中心输入")
			}
			if _, err := pipeline.Build(input, dispatcher.Neighbours{&wrong}, dispatcher.Options{}); err == nil {
				t.Fatal("接受了错误尺寸的邻居")
			}
		})
	}
}

// 水的满高选择既要跨区域边界，也要保留区域内相邻水格的行为。
func TestConfiguredFluidHeight(t *testing.T) {
	resources := tables(t)
	for _, side := range []int{1, 3} {
		layout := cube.New(side)
		pipeline := dispatcher.NewPipeline(layout, resources, effector.New(nil), blockactor.New(), nil, nil)
		input := dispatcher.Section{Blocks: blockdata.Uniform(layout.Len(), ir.IRBlock(`minecraft:water ["level"="0"]`))}
		for _, above := range []bool{false, true} {
			var neighbours dispatcher.Neighbours
			if above {
				neighbours[3] = &input
			}
			result, err := pipeline.Build(input, neighbours, dispatcher.Options{})
			if err != nil {
				t.Fatal(err)
			}
			height := 0.0
			for _, quad := range result.Frame.Mesh.Quads {
				for _, point := range quad.Positions {
					height = max(height, point[1])
				}
			}
			if above && height != float64(side) || !above && !(height > float64(side-1) && height < float64(side)) {
				t.Fatalf("side=%d above=%v height=%f", side, above, height)
			}
		}
	}
}

// 在非二次幂区域的非原点放置特效与 actor，验证坐标和 NBT 不依赖旧位移索引。
func TestSmallRegionActorCoordinates(t *testing.T) {
	layout := cube.New(3)
	handler := &actorHandler{}
	pipeline := dispatcher.NewPipeline(
		layout,
		tables(t),
		effector.New(map[string]string{"minecraft:enchanting_table": "enchanting_table_book"}),
		blockactor.New(handler), nil,
		nil,
	)
	input := dispatcher.Section{
		Blocks:        blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:air []")),
		BlockEntities: map[dispatcher.CellPos]nbt.Compound{{2, 1, 0}: {"marker": "pot"}},
	}
	input.Blocks.Set(layout.Index(2, 1, 0), ir.IRBlock(`minecraft:decorated_pot ["cracked"="false","facing"="east","waterlogged"="true"]`))
	input.Blocks.Set(layout.Index(0, 2, 1), ir.IRBlock("minecraft:enchanting_table []"))
	result, err := pipeline.Build(input, dispatcher.Neighbours{}, dispatcher.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frame.Actors) != 1 || result.Frame.Actors[0].Pos != ([3]uint8{2, 1, 0}) || result.Frame.Actors[0].Mesh.Quads[0].Positions[0] != (geometry.Vec3{2, 1, 0}) {
		t.Fatal("actor 的位置不正确")
	}
	if len(handler.inputs) != 1 || handler.inputs[0].NBT["marker"] != "pot" || !handler.inputs[0].Waterlogged {
		t.Fatal("actor 的 NBT 或含水信息丢失")
	}
	if len(result.Frame.Effects) != 1 || result.Frame.Effects[0].Pos != ([3]uint8{0, 2, 1}) {
		t.Fatal("特效的位置不正确")
	}
}

// 比较同一个含水方块分别放入 1³ 和 16³ 输入时的实际构建开销。
func BenchmarkIsolatedWaterloggedBlock(b *testing.B) {
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		b.Fatal(err)
	}
	for _, side := range []int{1, 16} {
		b.Run(fmt.Sprint(side), func(b *testing.B) {
			layout := cube.New(side)
			pipeline := dispatcher.NewPipeline(layout, resources.Tables, effector.New(nil), blockactor.New(), nil, nil)
			blocks := blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:air []"))
			blocks.Set(0, ir.IRBlock(`minecraft:oak_leaves ["distance"="1","persistent"="true","waterlogged"="true"]`))
			input := dispatcher.Section{Blocks: blocks}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := pipeline.Build(input, dispatcher.Neighbours{}, dispatcher.Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
