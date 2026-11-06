package mesher

import (
	"strings"
	"testing"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/geometry/material"
	"neomega-viewer-core/viewer/mesher/meshing"
	preset "neomega-viewer-core/viewer/resources"
)

// 同时检查入库资源与网格输出，避免只修改烘焙规则却继续发布旧的双面几何。
func TestPresetTransparentVolumesCullBackFaces(t *testing.T) {
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	layout := cube.New(2)
	renderer := NewMesher(layout, resources.Tables)
	for _, name := range []string{
		"glass", "white_stained_glass", "red_stained_glass", "tinted_glass",
		"glass_pane", "red_stained_glass_pane", "ice", "honey_block", "slime_block",
	} {
		t.Run(name, func(t *testing.T) {
			state := ir.IRBlock("minecraft:" + name + " []")
			if strings.HasSuffix(name, "_pane") {
				state = ir.IRBlock("minecraft:" + name + ` ["north"="true","east"="false","south"="true","west"="false"]`)
			}
			mesh, err := renderer.MeshBlock(state.Tidy().Hash())
			if err != nil {
				t.Fatal(err)
			}
			if len(mesh.Quads) == 0 {
				t.Fatal("半透明方块没有生成几何")
			}
			for _, quad := range mesh.Quads {
				if quad.Surface.Mode != material.Blend || quad.Surface.DoubleSided {
					t.Fatalf("半透明实体应保持透明混合并剔除背面: %+v", quad.Surface)
				}
			}
		})
	}

	// 两块玻璃的公共面仍需剔除，单面绘制不能替代相邻方块的剔面。
	blocks := make([]uint32, layout.Len())
	for i := range blocks {
		blocks[i] = ir.IRBlock("minecraft:air []").Hash()
	}
	blocks[layout.Index(0, 0, 0)] = ir.IRBlock("minecraft:glass []").Hash()
	blocks[layout.Index(1, 0, 0)] = blocks[layout.Index(0, 0, 0)]
	result, err := renderer.MeshLayer(blocks, Options{Meshing: meshing.Options{DisableGreedy: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mesh.Quads) != 10 {
		t.Fatalf("两块相邻玻璃应只保留 10 个外表面，实际为 %d", len(result.Mesh.Quads))
	}
	for _, quad := range result.Mesh.Quads {
		if quad.Surface.DoubleSided {
			t.Fatal("玻璃网格重新启用了背面绘制")
		}
	}
}
