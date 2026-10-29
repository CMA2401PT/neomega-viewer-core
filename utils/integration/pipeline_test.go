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
	"strings"
	"testing"

	"neomega-assets/asset"
	"neomega-assets/parse"
	"neomega-assets/s01_extract"
	"neomega-assets/s02_patch"
	"neomega-assets/s03_models"
	"neomega-assets/s04_special"
	"neomega-assets/s05_atlas"
	"neomega-assets/utils/inventory"
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
		"assets/demo/blockstates/panel.json":               []byte(`{"multipart":[{"when":{"OR":[{"axis":"x"},{"AND":[{"axis":"y"},{"lit":true}]}]},"apply":{"model":"demo:block/panel"}}]}`),
		"assets/demo/models/block/panel.json":              []byte(`{"textures":{"skin":"demo:entity/panel","animated":"demo:entity/animated_panel"},"elements":[{"from":[0,0,0],"to":[16,16,16],"faces":{"up":{"texture":"#skin"},"down":{"texture":"#animated"}}}]}`),
		"assets/minecraft/textures/block/stone.png.mcmeta": []byte(`{"animation":{"frames":[{"index":1,"time":3},0,1]}}`),
		"assets/minecraft/textures/block/stone.png": fixturePNG(t, 16, 32, func(x, y int) color.NRGBA {
			if y >= 16 {
				return color.NRGBA{G: 255, A: 255}
			}
			return color.NRGBA{R: 255, A: 255}
		}),
		"assets/minecraft/textures/blocks/sequence.png.mcmeta": []byte(`{"animation":{"frametime":4}}`),
		"assets/minecraft/textures/blocks/sequence.png": fixturePNG(t, 8, 24, func(x, y int) color.NRGBA {
			return []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}}[y/8]
		}),
		"assets/demo/textures/entity/animated_panel.png.mcmeta": []byte(`{"animation":{"width":48,"height":16,"frametime":5,"frames":[1,0]}}`),
		"assets/demo/textures/entity/animated_panel.png": fixturePNG(t, 48, 32, func(x, y int) color.NRGBA {
			if y >= 16 {
				return color.NRGBA{G: 255, A: 255}
			}
			return color.NRGBA{R: 255, A: 255}
		}),
		"assets/demo/textures/entity/panel.png": fixturePNG(t, 48, 32, func(x, y int) color.NRGBA { return color.NRGBA{B: 255, A: 255} }),
		"assets/minecraft/../../escape.txt":     []byte("not a resource"),
		"com/example/NotAnAsset.class":          []byte("not a resource"),
	}
	// A real game-style placeholder must become visible through s04, rather
	// than being incorrectly counted as an ordinary model with zero faces.
	entries["assets/minecraft/blockstates/chest.json"] = []byte(`{"variants":{"":{"model":"minecraft:block/chest"}}}`)
	entries["assets/minecraft/models/block/chest.json"] = []byte(`{"textures":{"particle":"minecraft:block/stone"}}`)
	entries["assets/minecraft/models/block/block.json"] = []byte(`{}`)
	for _, name := range []string{"normal", "normal_left", "normal_right"} {
		entries["assets/minecraft/textures/entity/chest/"+name+".png"] = fixturePNG(t, 64, 64, func(x, y int) color.NRGBA { return color.NRGBA{R: 120, G: 60, A: 255} })
	}
	writeJar(t, jar, entries)
	extracted := filepath.Join(root, "01")
	patched := filepath.Join(root, "02")
	models := filepath.Join(root, "03")
	output := filepath.Join(root, "dist")
	completed := filepath.Join(root, "04")
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
	rawReport := readJSON[asset.ModelIndex](t, filepath.Join(models, "model-index.json"))
	if len(rawReport.GeometrylessBlockStates) != 1 || rawReport.GeometrylessBlockStates[0] != "minecraft:chest" {
		t.Fatalf("placeholder was not detected: %v", rawReport.GeometrylessBlockStates)
	}
	plans, err := s04_special.Run(context.Background(), s04_special.Options{InputDir: patched, ModelDir: models, OutputDir: completed})
	if err != nil {
		t.Fatal(err)
	}
	if plan := plans.Blocks["minecraft:chest"]; plan.Kind != asset.RendererSpecial || !plan.StaticPreview || len(plan.DynamicFields) == 0 {
		t.Fatalf("chest render contract missing: %+v", plan)
	}
	completedReport := readJSON[asset.ModelIndex](t, filepath.Join(completed, "model-index.json"))
	if len(completedReport.GeometrylessBlockStates) != 0 {
		t.Fatal("chest still has no geometry")
	}
	atlas, err := s05_atlas.Run(context.Background(), s05_atlas.Options{InputDir: completed, ModelDir: completed, OutputDir: output})
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
	for _, test := range []struct {
		name      string
		width     int
		frameTime int
		colors    []color.NRGBA
	}{
		{
			name:      "stone",
			width:     16,
			frameTime: 1,
			colors:    []color.NRGBA{{G: 255, A: 255}, {R: 255, A: 255}, {G: 255, A: 255}},
		},
		{
			name:      "sequence",
			width:     16,
			frameTime: 4,
			colors:    []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}},
		},
		{
			name:   "demo:entity/animated_panel",
			width:  48,
			colors: []color.NRGBA{{G: 255, A: 255}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			texture := atlas.Textures[test.name]
			if texture.Frames != len(test.colors) || texture.FrameTime != test.frameTime {
				t.Fatalf("animation metadata = %+v", texture)
			}
			if texture.SU*float64(atlas.Width) != float64(test.width) || texture.SV*float64(atlas.Height) != float64(16*len(test.colors)) {
				t.Fatalf("wrong strip dimensions: %+v", texture)
			}
			x0, y0 := int(texture.U*float64(atlas.Width)), int(texture.V*float64(atlas.Height))
			for i, want := range test.colors {
				for y := 0; y < 16; y++ {
					for x := 0; x < test.width; x++ {
						got := color.NRGBAModel.Convert(img.At(x0+x, y0+i*16+y)).(color.NRGBA)
						if got != want {
							t.Fatalf("frame %d pixel (%d,%d) = %v, want %v", i, x, y, got, want)
						}
					}
				}
			}
		})
	}
	for _, name := range []string{"unknown", "demo:entity/panel"} {
		if texture := atlas.Textures[name]; texture.Frames != 1 || texture.FrameTime != 0 {
			t.Fatalf("static texture %s metadata = %+v", name, texture)
		}
	}
	if _, ok := atlas.Textures["entity/chest/normal_left"]; !ok {
		t.Fatal("double chest entity texture was dropped")
	}
	if _, err := os.Stat(filepath.Join(output, "blockRenderers.json")); err != nil {
		t.Fatal("renderer table was not exported")
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
	parsed, err := parse.Load(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed.BlockState("minecraft:stone"); !ok {
		t.Fatal("parser could not look up a canonical vanilla block ID")
	}
	if _, ok := parsed.Model("demo:block/panel"); !ok {
		t.Fatal("parser lost the non-vanilla model namespace")
	}
	if texture, ok := parsed.Texture("minecraft:block/stone"); !ok || texture.Frames != 3 || texture.FrameTime != 1 {
		t.Fatalf("exported animation metadata lost: %+v", texture)
	}
	if rect, ok := parsed.TextureRect("minecraft:block/stone"); !ok || rect.Dx() != 16 || rect.Dy() != 48 {
		t.Fatalf("parser did not return the whole animation strip: %v", rect)
	}
	rect, ok := parsed.TextureRect("demo:entity/panel")
	if !ok || rect.Dx() != 48 || rect.Dy() != 32 || !rect.In(parsed.AtlasImage.Bounds()) {
		t.Fatalf("parser returned the wrong entity pixel rectangle: %v", rect)
	}
	for _, name := range []string{"blockstates.json", "models.json", "blocksAtlases.json"} {
		fields := readJSON[map[string]json.RawMessage](t, filepath.Join(output, name))
		if _, ok := fields["latest"]; ok {
			t.Fatalf("%s still has the legacy version envelope", name)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "blockStatesModels.json")); !os.IsNotExist(err) {
		t.Fatal("legacy combined output still exists")
	}
	base := parsed.Models["block/cube_all"]
	if (*base.Elements)[0].ShadeDirectionOverride != "north" {
		t.Fatal("modern shading direction lost")
	}
	model := parsed.Models["block/stone"]
	binding := model.Textures["all"]
	if !binding.Object || binding.ForceTranslucent == nil || !*binding.ForceTranslucent {
		t.Fatal("modern texture material information lost")
	}
	if _, ok := model.Extra["texture_size"]; !ok {
		t.Fatal("unknown model extension lost")
	}
	choice := parsed.BlockStates["stone"].Variants[""]
	if !choice.Array || len(choice.Options) != 2 || *choice.Options[0].Weight != 2 {
		t.Fatal("weighted alternatives lost")
	}
	if parsed.BlockStates["demo:panel"].Multipart[0].When.OR[1].AND[1].Properties["lit"] != "true" {
		t.Fatal("nested multipart condition lost")
	}
	if _, err := os.Stat(filepath.Join(output, "THIRD_PARTY_NOTICES.txt")); err != nil {
		t.Fatal("license notices not exported")
	}
	second := filepath.Join(root, "again")
	if _, err := s05_atlas.Run(context.Background(), s05_atlas.Options{InputDir: completed, ModelDir: completed, OutputDir: second}); err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := os.ReadFile(filepath.Join(output, "blocksAtlasLatest.png"))
	secondBytes, _ := os.ReadFile(filepath.Join(second, "blocksAtlasLatest.png"))
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("atlas placement is not deterministic")
	}
	audit, err := inventory.Run(inventory.Options{BaselineDir: output, CurrentDir: output, SnapshotDir: extracted, RawModelDir: models, CompletedDir: completed, OutputPath: filepath.Join(root, "inventory.json")})
	if err != nil {
		t.Fatal(err)
	}
	if len(audit.BeforeMissingGeometry) != 1 || len(audit.BeforeMissingBaselineTextures) != 3 || len(audit.AfterMissingGeometry) != 0 || len(audit.MissingRequiredTextures) != 0 || !audit.BoundsChecked {
		t.Fatalf("coverage audit failed: %+v", audit)
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

func TestJava1214BedHangingSignAndBannerPreviews(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "client.jar")
	entries := map[string][]byte{
		"version.json": []byte(`{"id":"1.21.4"}`),
		"assets/minecraft/models/block/block.json":    []byte(`{}`),
		"assets/minecraft/models/block/cube_all.json": []byte(`{"elements":[{"from":[0,0,0],"to":[16,16,16],"faces":{"up":{"texture":"#all"}}}]}`),
	}
	for _, name := range []string{"red_bed", "oak_wall_hanging_sign", "red_banner"} {
		entries["assets/minecraft/blockstates/"+name+".json"] = []byte(`{"variants":{"":{"model":"block/` + name + `"}}}`)
		entries["assets/minecraft/models/block/"+name+".json"] = []byte(`{}`)
	}
	for _, name := range []string{"entity/bed/red", "entity/signs/hanging/oak", "entity/banner_base"} {
		entries["assets/minecraft/textures/"+name+".png"] = fixturePNG(t, 64, 64, func(x, y int) color.NRGBA {
			return color.NRGBA{R: 180, G: 80, A: 255}
		})
	}
	writeJar(t, jar, entries)
	extracted, patched := filepath.Join(root, "01"), filepath.Join(root, "02")
	models, completed := filepath.Join(root, "03"), filepath.Join(root, "04")
	output := filepath.Join(root, "final")
	if _, err := s01_extract.Run(context.Background(), s01_extract.Options{JarPath: jar, OutputDir: extracted}); err != nil {
		t.Fatal(err)
	}
	if _, err := s02_patch.Run(context.Background(), s02_patch.Options{InputDir: extracted, OutputDir: patched}); err != nil {
		t.Fatal(err)
	}
	if _, err := s03_models.Run(s03_models.Options{InputDir: patched, OutputDir: models}); err != nil {
		t.Fatal(err)
	}
	plans, err := s04_special.Run(context.Background(), s04_special.Options{InputDir: patched, ModelDir: models, OutputDir: completed})
	if err != nil {
		t.Fatal(err)
	}
	for name, family := range map[asset.ID]string{"minecraft:red_bed": "bed", "minecraft:oak_wall_hanging_sign": "sign", "minecraft:red_banner": "banner"} {
		plan := plans.Blocks[name]
		if plan.Kind != asset.RendererSpecial || !plan.StaticPreview || plan.Family != family {
			t.Fatalf("wrong static preview for %s: %+v", name, plan)
		}
	}
	if !strings.Contains(plans.Blocks["minecraft:oak_wall_hanging_sign"].TemplateSource, "/1.20/blockStates/sign/") {
		t.Fatal("hanging sign attribution does not point to the actual upstream file")
	}
	atlas, err := s05_atlas.Run(context.Background(), s05_atlas.Options{InputDir: completed, ModelDir: completed, OutputDir: output})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"entity/bed/red", "entity/signs/hanging/oak", "entity/banner_base"} {
		if _, ok := atlas.Textures[name]; !ok {
			t.Fatalf("missing old-version skin %s", name)
		}
	}
}
