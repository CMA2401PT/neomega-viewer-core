package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"neomega-viewer-core/viewer-assets/cmd/build-tables/internal/tablegen"
)

func main() {
	// 默认路径以源码位置为准，可从任何工作目录生成配套资源。
	_, source, _, _ := runtime.Caller(0)
	inputDir := filepath.Dir(source)
	moduleRoot := filepath.Clean(filepath.Join(inputDir, "../../.."))
	assets := flag.String("assets", filepath.Join(moduleRoot, "viewer-assets/dist/final"), "同一次资源构建的输出目录")
	output := flag.String("output", filepath.Join(moduleRoot, "viewer/resources/geometry.nvtables"), "几何表输出路径")
	solid := flag.String("solid", filepath.Join(moduleRoot, "fixer/solidtable/preset/geometry.nvsolid"), "实心表输出路径")
	textures := flag.String("textures", filepath.Join(moduleRoot, "viewer/resources/textures"), "贴图与声明输出目录")
	flag.Parse()
	if err := generate(*assets, *output, *solid); err != nil {
		log.Fatal(err)
	}
	if err := copyTextures(*assets, *textures); err != nil {
		log.Fatal(err)
	}
}

// generate 由匹配的模型和图集生成几何、元数据及实心表。
func generate(assetsDir, output, solidPath string) error {
	builder, err := tablegen.Open(tablegen.OpenOptions{AssetsDir: assetsDir})
	if err != nil {
		return err
	}
	items, err := builder.BuildItems(assetsDir)
	if err != nil {
		return err
	}
	itemFile, err := os.Create(output + ".items.json.gz")
	if err != nil {
		return err
	}
	compressed := gzip.NewWriter(itemFile)
	if err := json.NewEncoder(compressed).Encode(items); err != nil {
		itemFile.Close()
		return err
	}
	if err := compressed.Close(); err != nil {
		itemFile.Close()
		return err
	}
	if err := itemFile.Close(); err != nil {
		return err
	}
	tables, err := builder.Build()
	if err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	if err := tables.Encode(file); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	solidFile, err := os.Create(solidPath)
	if err != nil {
		return err
	}
	if err := tables.Solid.Encode(solidFile); err != nil {
		solidFile.Close()
		return err
	}
	if err := solidFile.Close(); err != nil {
		return err
	}
	metadata, err := json.MarshalIndent(tables.Metadata, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(output+".java.json", append(metadata, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("%s: indexed=%d shapes=%d skipped=%d\n", output, len(tables.Index), len(tables.Shapes), len(tables.Skipped))
	fmt.Printf("%s: entries=%d\n", solidPath, len(tables.Solid))
	return nil
}

// copyTextures 交付与几何表相同构建的图集与来源声明。
func copyTextures(assetsDir, outputDir string) error {
	files := [][2]string{
		{"blocksAtlasLatest.png", "atlas.png"},
		{"THIRD_PARTY_NOTICES.txt", "THIRD_PARTY_NOTICES.txt"},
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(assetsDir, file[0]))
		if err != nil {
			return err
		}
		target := filepath.Join(outputDir, file[1])
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
