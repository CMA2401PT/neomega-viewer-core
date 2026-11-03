package s05_atlas

import (
	"os"
	"path/filepath"
	"testing"

	"neomega-viewer-core/viewer-assets/asset"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
)

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
