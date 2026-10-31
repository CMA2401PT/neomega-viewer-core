package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoricalAtlasKeysAreExpandedOnlyAtAuditBoundary(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"blockStatesModels.json": `{"blockstates":{"latest":{}},"models":{"latest":{}}}`,
		"blocksAtlases.json":     `{"latest":{"textures":{"stone":{},"banner/red":{},"entity/chest/normal":{},"demo:panel":{}}}}`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	atlas, _, _, err := readBaseline(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"minecraft:block/stone", "minecraft:block/banner/red", "minecraft:entity/chest/normal", "demo:block/panel"} {
		found := false
		for key := range atlas.Textures {
			found = found || string(key) == id
		}
		if !found {
			t.Fatalf("missing expanded texture %s", id)
		}
	}
}
