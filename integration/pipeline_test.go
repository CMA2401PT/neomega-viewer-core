// SPDX-License-Identifier: MIT
package integration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"neomega-assets/asset"
	"neomega-assets/s01_extract"
	"neomega-assets/s02_patch"
	"neomega-assets/s03_models"
	"neomega-assets/s04_atlas"
)

func fixturePNG(t *testing.T, width, height int, paint func(int, int) color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, paint(x, y))
		}
	}
	var result bytes.Buffer
	if err := png.Encode(&result, img); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}

func writeJar(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, value := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func readJSON[T any](t *testing.T, path string) T {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestJARToTablesAndPixelCoordinates(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "client.jar")
	entries := map[string][]byte{
		"version.json": []byte(`{"id":"fixture"}`),
		"assets/minecraft/blockstates/stone.json":          []byte(`{"variants":{"": [{"model":"minecraft:block/stone","weight":2},{"model":"minecraft:block/stone","y":90}]}}`),
		"assets/minecraft/models/block/cube_all.json":      []byte(`{"elements":[{"name":"test cube","shade_direction_override":"north","from":[0,0,0],"to":[16,16,16],"faces":{"north":{"texture":"#all"}}}]}`),
		"assets/minecraft/models/block/stone.json":         []byte(`{"parent":"minecraft:block/cube_all","textures":{"all":{"sprite":"minecraft:block/stone","force_translucent":true}},"texture_size":[16,16]}`),
		"assets/demo/blockstates/panel.json":               []byte(`{"multipart":[{"when":{"OR":[{"axis":"x"},{"AND":[{"axis":"y"},{"lit":"true"}]}]},"apply":{"model":"demo:block/panel"}}]}`),
		"assets/demo/models/block/panel.json":              []byte(`{"textures":{"skin":"demo:entity/panel"},"elements":[{"from":[0,0,0],"to":[16,16,16],"faces":{"up":{"texture":"#skin"}}}]}`),
		"assets/minecraft/textures/block/stone.png.mcmeta": []byte(`{"animation":{"frames":[{"index":1,"time":3},0]}}`),
		"assets/minecraft/textures/block/stone.png": fixturePNG(t, 16, 32, func(x, y int) color.NRGBA {
			if y >= 16 {
				return color.NRGBA{G: 255, A: 255}
			}
			return color.NRGBA{R: 255, A: 255}
		}),
		"assets/demo/textures/entity/panel.png": fixturePNG(t, 48, 32, func(x, y int) color.NRGBA { return color.NRGBA{B: 255, A: 255} }),
		"assets/minecraft/../../escape.txt":     []byte("not a resource"),
		"com/example/NotAnAsset.class":          []byte("not a resource"),
	}
	writeJar(t, jar, entries)
	extracted := filepath.Join(root, "01")
	patched := filepath.Join(root, "02")
	models := filepath.Join(root, "03")
	output := filepath.Join(root, "dist")
	snapshot, err := s01_extract.Run(context.Background(), s01_extract.Options{JarPath: jar, OutputDir: extracted})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.BlockStates["demo:panel"]; !ok {
		t.Fatal("non-vanilla namespace lost")
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("archive escaped the output directory")
	}
	if _, err := s02_patch.Run(context.Background(), s02_patch.Options{InputDir: extracted, OutputDir: patched}); err != nil {
		t.Fatal(err)
	}
	if _, err := s03_models.Run(s03_models.Options{InputDir: patched, OutputDir: models}); err != nil {
		t.Fatal(err)
	}
	atlas, err := s04_atlas.Run(context.Background(), s04_atlas.Options{InputDir: patched, ModelDir: models, OutputDir: output})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(output, "blocksAtlasLatest.png"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	stone := atlas.Textures["stone"]
	pixel := color.NRGBAModel.Convert(img.At(int(stone.U*float64(atlas.Width)), int(stone.V*float64(atlas.Height)))).(color.NRGBA)
	if pixel != (color.NRGBA{G: 255, A: 255}) {
		t.Fatalf("selected wrong animation frame: %v", pixel)
	}
	panel := atlas.Textures["demo:entity/panel"]
	if panel.SU*float64(atlas.Width) != 48 || panel.SV*float64(atlas.Height) != 32 {
		t.Fatal("entity texture dimensions lost")
	}
	var occupied []image.Rectangle
	for name, texture := range atlas.Textures {
		rect := image.Rect(int(texture.U*float64(atlas.Width)), int(texture.V*float64(atlas.Height)), int((texture.U+texture.SU)*float64(atlas.Width)), int((texture.V+texture.SV)*float64(atlas.Height)))
		if !rect.In(img.Bounds()) {
			t.Fatalf("texture %s is outside the PNG", name)
		}
		for _, previous := range occupied {
			if rect.Overlaps(previous) {
				t.Fatalf("overlapping texture %s", name)
			}
		}
		occupied = append(occupied, rect)
	}
	tables := readJSON[asset.Tables](t, filepath.Join(output, "blockStatesModels.json"))
	base := tables.Models["latest"]["block/cube_all"]
	if (*base.Elements)[0].ShadeDirectionOverride != "north" {
		t.Fatal("modern shading direction lost")
	}
	model := tables.Models["latest"]["block/stone"]
	binding := model.Textures["all"]
	if !binding.Object || binding.ForceTranslucent == nil || !*binding.ForceTranslucent {
		t.Fatal("modern texture material information lost")
	}
	if _, ok := model.Extra["texture_size"]; !ok {
		t.Fatal("unknown model extension lost")
	}
	choice := tables.BlockStates["latest"]["stone"].Variants[""]
	if !choice.Array || len(choice.Options) != 2 || *choice.Options[0].Weight != 2 {
		t.Fatal("weighted alternatives lost")
	}
	if tables.BlockStates["latest"]["demo:panel"].Multipart[0].When.OR[1].AND[1].Properties["lit"] != "true" {
		t.Fatal("nested multipart condition lost")
	}
	if _, err := os.Stat(filepath.Join(output, "THIRD_PARTY_NOTICES.txt")); err != nil {
		t.Fatal("license notices not exported")
	}
	second := filepath.Join(root, "again")
	if _, err := s04_atlas.Run(context.Background(), s04_atlas.Options{InputDir: patched, ModelDir: models, OutputDir: second}); err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := os.ReadFile(filepath.Join(output, "blocksAtlasLatest.png"))
	secondBytes, _ := os.ReadFile(filepath.Join(second, "blocksAtlasLatest.png"))
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("atlas placement is not deterministic")
	}
}

func TestOverlayIsExplicitAndLeavesExtractedAssetsIntact(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	output := filepath.Join(root, "output")
	pack := filepath.Join(root, "pack")
	snapshot := asset.NewSnapshot("fixture")
	original := "assets/minecraft/models/block/cube_all.json"
	snapshot.Models["minecraft:block/cube_all"] = asset.File{Path: original, Origin: "fixture"}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(input, original)), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(input, original), []byte(`{"textures":{"source":"block/old"}}`), 0644)
	raw, _ := json.Marshal(snapshot)
	os.WriteFile(filepath.Join(input, "snapshot.json"), raw, 0644)
	os.MkdirAll(filepath.Dir(filepath.Join(pack, original)), 0755)
	os.WriteFile(filepath.Join(pack, original), []byte(`{"textures":{"source":"block/new"}}`), 0644)
	if _, err := s02_patch.Run(context.Background(), s02_patch.Options{InputDir: input, OutputDir: output, ResourcePacks: []string{pack}}); err != nil {
		t.Fatal(err)
	}
	old := readJSON[asset.Model](t, filepath.Join(input, original))
	updated := readJSON[asset.Model](t, filepath.Join(output, original))
	if old.Textures["source"].Sprite != "block/old" || updated.Textures["source"].Sprite != "block/new" {
		t.Fatal("overlay did not preserve stage boundaries")
	}
}
