//go:build ignore

// 在 neomega-viewer-core 模块根目录运行 go run ./viewer/web/tests/fixtures/generate_banner.go。
// 生成两面同名私有贴图、不同图案的真实 Go 快照，供 TypeScript 解码和资源生命周期测试使用。
package main

import (
	"log"
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/javapreset"
	resources "neomega-viewer-core/viewer/resources"
	"os"
)

func main() {
	preset, err := resources.Open(resources.OpenOptions{})
	if err != nil {
		log.Fatal(err)
	}
	layout := cube.New(16)
	pipeline, err := javapreset.NewPipeline(layout, preset.Tables, preset.Items, nil)
	if err != nil {
		log.Fatal(err)
	}
	section := dispatcher.Section{Blocks: blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:air []")), BlockEntities: map[dispatcher.CellPos]nbt.Compound{}}
	for i, color := range []string{"white", "yellow"} {
		pos := dispatcher.CellPos{uint8(3 + 3*i), 1, 3}
		section.Blocks.Set(layout.Index(int(pos[0]), 1, 3), ir.IRBlock("minecraft:"+color+`_banner ["rotation"="0"]`))
		section.BlockEntities[pos] = nbt.Compound{"patterns": []any{map[string]any{"pattern": "minecraft:stripe_bottom", "color": "blue"}, map[string]any{"pattern": "minecraft:cross", "color": "red"}}}
	}
	result, err := pipeline.Build(section, dispatcher.Neighbours{}, dispatcher.Options{})
	if err != nil {
		log.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		log.Fatal(result.Diagnostics)
	}
	file, err := os.Create("viewer/web/tests/fixtures/banners.nvframe")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	if err = result.Frame.Marshal(file); err != nil {
		log.Fatal(err)
	}
}
