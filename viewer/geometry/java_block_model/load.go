// SPDX-License-Identifier: MIT
// Bundle loading consumes the single-version resource contract without importing
// any extraction, patching or atlas-generation stages.
package java_block_model

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// Bundle contains the typed tables and the decoded atlas from one final directory.
// Models retain their parent references; loading does not choose state variants.
type Bundle struct {
	Rules      RuleTable
	Models     Models
	Atlas      Atlas
	Renderers  RendererTable
	Info       BundleInfo
	AtlasImage image.Image
}

func ReadRules(path string) (RuleTable, error) {
	return readJSON[RuleTable](path)
}

func ReadModels(path string) (Models, error) {
	return readJSON[Models](path)
}

func ReadAtlas(path string) (Atlas, error) {
	return readJSON[Atlas](path)
}

func ReadRenderers(path string) (RendererTable, error) {
	return readJSON[RendererTable](path)
}

func ReadInfo(path string) (BundleInfo, error) {
	return readJSON[BundleInfo](path)
}

func ReadAtlasImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}

// Load reads all data files from one final output directory. For consumers that
// need only one table, the Read functions avoid loading the rest of the bundle.
func Load(dir string) (*Bundle, error) {
	var result Bundle
	var err error
	result.Rules, err = ReadRules(filepath.Join(dir, "blockstates.json"))
	if err != nil {
		return nil, err
	}
	result.Models, err = ReadModels(filepath.Join(dir, "models.json"))
	if err != nil {
		return nil, err
	}
	result.Atlas, err = ReadAtlas(filepath.Join(dir, "blocksAtlases.json"))
	if err != nil {
		return nil, err
	}
	result.Renderers, err = ReadRenderers(filepath.Join(dir, "blockRenderers.json"))
	if err != nil {
		return nil, err
	}
	result.Info, err = ReadInfo(filepath.Join(dir, "build-info.json"))
	if err != nil {
		return nil, err
	}
	result.AtlasImage, err = ReadAtlasImage(filepath.Join(dir, "blocksAtlasLatest.png"))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// BundleInfo is the consumer's version check, not the producer's build report.
type BundleInfo struct {
	Version string `json:"version"`
}

// RendererTable describes the resource bundle's supported block appearances.
type RendererTable struct {
	Version string                       `json:"version"`
	Blocks  map[ResourceID]BlockRenderer `json:"blocks"`
}

func readJSON[T any](path string) (T, error) {
	var value T
	data, err := os.ReadFile(path)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("%s: %w", path, err)
	}
	return value, nil
}
