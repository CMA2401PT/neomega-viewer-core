package javapreset_test

import (
	"bytes"
	"encoding/binary"
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/javapreset"
	"neomega-viewer-core/viewer/merger"
	resources "neomega-viewer-core/viewer/resources"
	"testing"
)

func TestBannerPipelineAndFallback(t *testing.T) {
	preset, err := resources.Open(resources.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	layout := cube.New(3)
	pipeline, err := javapreset.NewPipeline(layout, preset.Tables, nil)
	if err != nil {
		t.Fatal(err)
	}
	section := dispatcher.Section{Blocks: blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:air []")), Entities: map[dispatcher.CellPos]nbt.Compound{}}
	for x := 0; x < 3; x++ {
		section.Blocks.Set(layout.Index(x, 1, 1), ir.IRBlock(`minecraft:white_banner ["rotation"="0"]`))
		section.Entities[dispatcher.CellPos{uint8(x), 1, 1}] = nbt.Compound{"patterns": []any{map[string]any{"pattern": "minecraft:cross", "color": "red"}}}
	}
	section.Entities[dispatcher.CellPos{1, 1, 1}] = nbt.Compound{"patterns": []any{map[string]any{"pattern": "minecraft:missing", "color": "red"}}}
	result, err := pipeline.Build(section, dispatcher.Neighbours{}, dispatcher.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frame.Actors) != 2 || len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != dispatcher.ActorError {
		t.Fatalf("错误图案应只让当前格降级: %+v", result.Diagnostics)
	}
	if result.Stats.Blocks != 1 {
		t.Fatal("成功的旗帜被重复绘制，或失败的旗帜没有保留静态本体")
	}
	for _, actor := range result.Frame.Actors {
		if len(actor.Textures) != 2 {
			t.Fatal("私有贴图没有传出")
		}
	}
	var encoded bytes.Buffer
	if err = result.Frame.Marshal(&encoded); err != nil {
		t.Fatal(err)
	}
	restored, err := merger.Unmarshal(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for i, actor := range restored.Actors {
		if !bytes.Equal(actor.Textures[0].Pixels, result.Frame.Actors[i].Textures[0].Pixels) {
			t.Fatal("快照丢失像素")
		}
	}
	// 截断与夸大尺寸都必须作为不完整载荷返回，不能读取下一段或分配任意大的图片。
	if _, err = merger.Unmarshal(bytes.NewReader(encoded.Bytes()[:encoded.Len()-4])); err == nil {
		t.Fatal("接受了截断图片")
	}
	// 乘积溢出不能让巨大图片伪装成空载荷。
	broken := bytes.Clone(encoded.Bytes())
	actorOffset := int(binary.LittleEndian.Uint32(broken[40:44]))
	meshLength := int(binary.LittleEndian.Uint32(broken[actorOffset+8 : actorOffset+12]))
	textureHeader := actorOffset + 4 + 8 + meshLength + 4
	binary.LittleEndian.PutUint32(broken[textureHeader+4:], 0x80000000)
	binary.LittleEndian.PutUint32(broken[textureHeader+8:], 0x80000000)
	if _, err = merger.Unmarshal(bytes.NewReader(broken)); err == nil {
		t.Fatal("接受了溢出的图片尺寸")
	}
}
