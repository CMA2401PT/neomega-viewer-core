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
	"reflect"
	"testing"
)

func TestSignPipelineWaterAndSnapshot(t *testing.T) {
	preset, err := resources.Open(resources.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	layout := cube.New(2)
	pipeline, err := javapreset.NewPipeline(layout, preset.Tables, preset.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	state := ir.IRBlock(`minecraft:oak_sign ["rotation"="0","waterlogged"="true"]`).Tidy()
	section := dispatcher.Section{
		Blocks: blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:air []")),
		BlockEntities: map[dispatcher.CellPos]nbt.Compound{
			{0, 1, 0}: {"front_text": map[string]any{"messages": []string{"你好 Java"}, "has_glowing_text": int8(1)}},
			{1, 1, 0}: {"front_text": map[string]any{"color": "invalid"}},
		},
	}
	section.Blocks.Set(layout.Index(0, 1, 0), state)
	section.Blocks.Set(layout.Index(1, 1, 0), state)
	result, err := pipeline.Build(section, dispatcher.Neighbours{}, dispatcher.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frame.Actors) != 1 || len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != dispatcher.ActorError {
		t.Fatal("只有无效文字所在格应退回静态牌体", result.Diagnostics)
	}
	water, wood := false, false
	for _, q := range result.Frame.Mesh.Quads {
		if q.Surface.Texture == "minecraft:block/water_still" || q.Surface.Texture == "minecraft:block/water_flow" {
			water = true
		}
		if q.Surface.Texture == "minecraft:entity/signs/oak" {
			wood = true
		}
	}
	if !water || !wood {
		t.Fatal("actor 替换本体时丢了水层，或错误牌体没有回退")
	}
	actor := result.Frame.Actors[0]
	if actor.Pos != ([3]uint8{0, 1, 0}) || len(actor.Textures) != 2 {
		t.Fatal("actor 坐标或文字贴图没有传出")
	}
	var encoded bytes.Buffer
	if err := result.Frame.Marshal(&encoded); err != nil {
		t.Fatal(err)
	}
	restored, err := merger.Unmarshal(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Actors[0].Textures, actor.Textures) {
		t.Fatal("快照没有原样保留文字像素")
	}
	unlit := false
	for _, q := range restored.Actors[0].Mesh.Quads {
		if q.Surface.Texture == "sign:front" {
			unlit = q.Surface.Unlit
			if q.Positions[0][1] < 1 {
				t.Fatal("格内网格未平移")
			}
		}
	}
	if !unlit {
		t.Fatal("发光文字的 Unlit 属性没有通过协议")
	}
}

func TestBannerPipelineAndFallback(t *testing.T) {
	preset, err := resources.Open(resources.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	layout := cube.New(3)
	pipeline, err := javapreset.NewPipeline(layout, preset.Tables, preset.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	section := dispatcher.Section{Blocks: blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:air []")), BlockEntities: map[dispatcher.CellPos]nbt.Compound{}}
	for x := 0; x < 3; x++ {
		section.Blocks.Set(layout.Index(x, 1, 1), ir.IRBlock(`minecraft:white_banner ["rotation"="0"]`))
		section.BlockEntities[dispatcher.CellPos{uint8(x), 1, 1}] = nbt.Compound{"patterns": []any{map[string]any{"pattern": "minecraft:cross", "color": "red"}}}
	}
	section.BlockEntities[dispatcher.CellPos{1, 1, 1}] = nbt.Compound{"patterns": []any{map[string]any{"pattern": "minecraft:missing", "color": "red"}}}
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
