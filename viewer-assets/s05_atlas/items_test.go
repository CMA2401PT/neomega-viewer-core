package s05_atlas

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"neomega-viewer-core/viewer-assets/asset"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
)

func TestFixedDisplayModelSelection(t *testing.T) {
	for _, data := range []string{
		`{"type":"minecraft:range_dispatch","property":"minecraft:use_cycle","entries":[{"threshold":0.25,"model":{"type":"minecraft:model","model":"example:wrong"}}],"fallback":{"type":"minecraft:model","model":"example:fixed"}}`,
		`{"type":"minecraft:range_dispatch","property":"minecraft:use_duration","entries":[{"threshold":0,"model":{"type":"minecraft:model","model":"example:fixed"}}],"fallback":{"type":"minecraft:model","model":"example:wrong"}}`,
		`{"type":"minecraft:condition","property":"minecraft:using_item","on_false":{"type":"minecraft:model","model":"example:fixed"},"on_true":{"type":"minecraft:special"}}`,
		`{"type":"minecraft:select","property":"minecraft:display_context","cases":[{"when":["gui","fixed"],"model":{"type":"minecraft:model","model":"example:fixed"}}],"fallback":{"type":"minecraft:special"}}`,
		`{"type":"minecraft:select","property":"minecraft:display_context","cases":[{"when":"fixed","model":{"type":"minecraft:model","model":"example:fixed"}}]}`,
		`{"type":"minecraft:select","property":"minecraft:charge_type","cases":[],"fallback":{"type":"minecraft:condition","property":"minecraft:using_item","on_false":{"type":"minecraft:model","model":"example:fixed"}}}`,
	} {
		var definition staticItemModel
		if err := json.Unmarshal([]byte(data), &definition); err != nil {
			t.Fatal(err)
		}
		if id, err := fixedItemReference(&definition); err != nil || id != "example:fixed" {
			t.Fatalf("%s: %s, %v", data, id, err)
		}
	}
	for _, data := range []string{
		`{"type":"minecraft:condition","property":"example:unknown","on_false":{"type":"minecraft:model","model":"example:wrong"}}`,
		`{"type":"minecraft:model","model":"example:wrong","tints":[{"type":"minecraft:constant","value":123}]}`,
		`{"type":"minecraft:select","property":"minecraft:display_context","cases":[]}`,
	} {
		var definition staticItemModel
		if err := json.Unmarshal([]byte(data), &definition); err != nil {
			t.Fatal(err)
		}
		if _, err := fixedItemReference(&definition); !errors.Is(err, errDynamicItem) {
			t.Fatalf("unsupported definition: %s: %v", data, err)
		}
	}
}

func TestItemsUseExplicitModelReference(t *testing.T) {
	// 物品 ID 与模型路径故意不同；复杂定义中的 model 是对象，不能提前按字符串解码。
	dir := t.TempDir()
	snapshot := *asset.NewSnapshot("test")
	for id, data := range map[model.ResourceID]string{
		"example:item":    `{"model":{"type":"minecraft:model","model":"example:arbitrary/path"}}`,
		"example:complex": `{"model":{"type":"minecraft:special","model":{"type":"minecraft:shulker_box"}}}`,
	} {
		name := id.Path() + ".json"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		snapshot.Items[id] = asset.File{Path: name}
	}
	elements := []model.Element{{Faces: map[model.Direction]model.Face{"south": {Texture: "example:chosen_texture"}}}}
	models := model.Models{"example:arbitrary/path": {Elements: &elements}, "minecraft:block/item_frame": {}, "minecraft:block/glow_item_frame": {}}
	textures := map[model.ResourceID]bool{}
	items, err := staticItems(snapshot, dir, models, textures)
	if err != nil {
		t.Fatal(err)
	}
	if items.Models["example:item"] != "example:arbitrary/path" || !textures["example:chosen_texture"] || items.Unsupported["example:complex"] == "" {
		t.Fatalf("items=%+v, textures=%v", items, textures)
	}
	delete(models, "example:arbitrary/path")
	if _, err := staticItems(snapshot, dir, models, map[model.ResourceID]bool{}); err == nil {
		t.Fatal("missing explicit model was silently skipped")
	}
}
