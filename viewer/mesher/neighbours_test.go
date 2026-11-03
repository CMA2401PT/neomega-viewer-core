package mesher

import (
	"fmt"
	"math"
	"testing"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/mesher/geometry"
	"neomega-viewer-core/viewer/mesher/meshing"
	preset "neomega-viewer-core/viewer/resources"
)

// 连续流体跨过水平边界的顶部时不能留下细条；真实高度差必须保留。
func TestFluidAcrossHorizontalSectionTop(t *testing.T) {
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, fluid := range []string{"water", "lava"} {
		for _, size := range []int{1, 3, 16} {
			layout := cube.New(size)
			renderer := NewMesher(layout, resources.Tables)
			liquid, air := make([]uint32, layout.Len()), make([]uint32, layout.Len())
			for i := range liquid {
				liquid[i] = ir.IRBlock(`minecraft:` + fluid + ` ["level"="0"]`).Hash()
				air[i] = ir.IRBlock("minecraft:air []").Hash()
			}
			for _, offset := range [][3]int{{-1, 0, 0}, {1, 0, 0}, {0, 0, -1}, {0, 0, 1}} {
				for _, connected := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/size=%d/offset=%v/connected=%t", fluid, size, offset, connected), func(t *testing.T) {
						neighbours := Neighbours[uint32]{
							{0, 1, 0}: {liquid},
							offset:    {liquid},
						}
						if connected {
							aboveOffset := offset
							aboveOffset[1] = 1
							// 上方流体在另一层，邻域选择不能假定层号相同。
							neighbours[aboveOffset] = [][]uint32{air, liquid}
						}
						result, err := renderer.MeshWithNeighbours(
							[][]uint32{liquid},
							neighbours,
							Options{Meshing: meshing.Options{DisableGreedy: true}},
						)
						if err != nil {
							t.Fatal(err)
						}
						normal := geometry.Vec3{float64(offset[0]), 0, float64(offset[2])}
						faces := 0
						for _, quad := range result.Mesh.Quads {
							if quad.Normal != normal {
								continue
							}
							faces++
							for _, point := range quad.Positions {
								if math.Abs(point[1]-float64(size)) > 1e-6 && math.Abs(point[1]-(float64(size)-1.0/9)) > 1e-6 {
									t.Fatalf("真实高度差之外还残留了侧面：%v", quad.Positions)
								}
							}
						}
						want := size
						if connected {
							want = 0
						}
						if faces != want {
							t.Fatalf("侧面数量为 %d，期望 %d", faces, want)
						}
					})
				}
			}
		}
	}
}
