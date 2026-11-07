package tablegen

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"neomega-viewer-core/fixer/solidtable"
	javasolid "neomega-viewer-core/fixer/solidtable/preset"
	"neomega-viewer-core/viewer/geometry"
	blockmesh "neomega-viewer-core/viewer/resources"
	"neomega-viewer-core/viewer/topview"
)

func TestPersistedTablesMatchBuild(t *testing.T) {
	dir := os.Getenv("NEOMEGA_ASSETS_DIR")
	if dir == "" {
		t.Skip("set NEOMEGA_ASSETS_DIR to the local generator input directory")
	}
	builder, err := Open(OpenOptions{AssetsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	want, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	items, err := builder.BuildItems(dir)
	if err != nil {
		t.Fatal(err)
	}
	var itemBytes bytes.Buffer
	compressed := gzip.NewWriter(&itemBytes)
	if err := json.NewEncoder(compressed).Encode(items); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	var solidBytes bytes.Buffer
	if err := want.Solid.Encode(&solidBytes); err != nil {
		t.Fatal(err)
	}
	got, err := solidtable.Decode(bytes.NewReader(solidBytes.Bytes()))
	if err != nil || !maps.Equal(got, want.Solid) {
		t.Fatalf("roundtrip solid table differs: error=%v", err)
	}
	embedded, err := javasolid.Bundled()
	if err != nil || !maps.Equal(embedded, want.Solid) {
		t.Fatalf("Java preset embedded solid table differs: error=%v", err)
	}
	if len(want.Solid) != len(want.Index) {
		t.Fatal("solid table and geometry index contain different numbers of states")
	}
	var colorBytes bytes.Buffer
	if err := want.Colors.Encode(&colorBytes); err != nil {
		t.Fatal(err)
	}
	colors, err := topview.Decode(bytes.NewReader(colorBytes.Bytes()))
	if err != nil || !maps.Equal(colors, want.Colors) {
		t.Fatalf("roundtrip color table differs: error=%v", err)
	}
	embeddedColors, err := topview.Bundled()
	if err != nil || !maps.Equal(embeddedColors, colors) {
		t.Fatalf("embedded color table differs: error=%v", err)
	}
	committedColors, err := os.ReadFile("../../../../../viewer/topview/colors.nvcolor")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(committedColors, colorBytes.Bytes()) {
		t.Fatal("committed color table differs")
	}
	path := filepath.Join(t.TempDir(), "geometry.nvtables")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = want.Encode(file)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("write table: %v; close: %v", err, closeErr)
	}
	metadata, err := json.Marshal(want.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".java.json", metadata, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".items.json.gz", itemBytes.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		Name string
		Path string
	}{
		{Name: "roundtrip", Path: path},
		{Name: "committed", Path: "../../../../../viewer/resources/geometry.nvtables"},
		{Name: "embedded"},
	} {
		t.Run(check.Name, func(t *testing.T) {
			resolver, err := blockmesh.Open(blockmesh.OpenOptions{TablesPath: check.Path})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(resolver.Items, items) {
				t.Fatal("static item catalog differs")
			}
			got := resolver.Tables
			// 空切片与 nil 的查询语义相同，按元素比较。
			if len(got.Index) != len(want.Index) || len(got.Shapes) != len(want.Shapes) {
				t.Fatalf("table sizes differ: states=%d/%d shapes=%d/%d", len(got.Index), len(want.Index), len(got.Shapes), len(want.Shapes))
			}
			for hash, ids := range want.Index {
				loaded, err := got.ModelIDs(hash)
				if err != nil || !slices.Equal(loaded, ids) {
					t.Fatalf("Java hash %d: IDs=%v, want %v, error=%v", hash, loaded, ids, err)
				}
			}
			for id, shape := range want.Shapes {
				loaded, err := got.Shape(geometry.ModelID(id))
				if err != nil {
					t.Fatal(err)
				}
				if loaded.CullGroup != shape.CullGroup || !slices.Equal(loaded.Quads, shape.Quads) {
					t.Fatalf("shape %d geometry differs", id)
				}
				for side, covers := range shape.Boundary {
					if !slices.Equal(loaded.Boundary[side], covers) {
						t.Fatalf("shape %d boundary %d differs", id, side)
					}
				}
			}
			if !maps.Equal(got.FullHeight, want.FullHeight) {
				t.Fatal("fluid full-height references differ")
			}
			if resolver.Metadata.Version != want.Version || !maps.Equal(resolver.Metadata.Renderers, want.Renderers) || !reflect.DeepEqual(resolver.Metadata.Skipped, want.Skipped) {
				t.Fatal("response metadata differs")
			}
			t.Logf("matched %d Java hashes, %d shapes, %d fluid references", len(want.Index), len(want.Shapes), len(want.FullHeight))
		})
	}
}
