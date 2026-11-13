package brickmap_test

import (
	"slices"
	"testing"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/brickmap"
)

func TestPaletteAirMaterialsAndUnknown(t *testing.T) {
	// 验证调用方只用公开接口就能取得稳定编号、共享材质和未知方块报告。
	source, err := brickmap.OpenMaterialSource()
	if err != nil {
		t.Fatal(err)
	}
	palette := brickmap.NewPalette(source)
	for _, block := range []ir.IRBlock{
		"", "minecraft:air []", "minecraft:cave_air []", "minecraft:void_air []",
		"minecraft:barrier []", "minecraft:light []", "minecraft:structure_void []",
	} {
		if id := palette.ID(block); id != 0 {
			t.Fatalf("%q 应为空气，得到编号 %d", block, id)
		}
	}
	if len(palette.Materials()) != 1 || len(palette.Unknown()) != 0 {
		t.Fatal("空气不应增加材质或未知方块记录")
	}
	stone := ir.IRBlock("minecraft:stone []")
	stoneID := palette.ID(stone)
	material, known := source.Material(stone)
	if !known || stoneID != 1 || palette.Materials()[stoneID] != material {
		t.Fatal("方块编号没有引用提取出的材质")
	}
	if palette.ID(stone) != stoneID || len(palette.Materials()) != 2 {
		t.Fatal("重复状态没有复用编号")
	}
	fallbackID := palette.ID("test:zzz_unknown []")
	if fallbackID == 0 || fallbackID == stoneID || palette.ID("test:aaa_unknown []") != fallbackID {
		t.Fatal("未知方块的相同回退材质没有共用独立编号")
	}
	if palette.ID("other:zzz_unknown []") != fallbackID || len(palette.Materials()) != 3 {
		t.Fatal("不同状态的相同材质没有去重")
	}
	if !slices.Equal(palette.Unknown(), []string{"aaa_unknown", "zzz_unknown"}) {
		t.Fatalf("未知方块短名没有去重排序：%v", palette.Unknown())
	}
}
