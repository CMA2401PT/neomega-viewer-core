// SPDX-License-Identifier: MIT
// Package inventory checks atlas coverage and compares visible block geometry.
package inventory

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"neomega-viewer-core/viewer-assets/asset"
	"neomega-viewer-core/viewer-assets/utils/jsonfile"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
)

type Options struct {
	BaselineDir  string
	CurrentDir   string
	SnapshotDir  string
	RawModelDir  string
	CompletedDir string
	OutputPath   string
}

type Report struct {
	Baseline                      string              `json:"baseline"`
	Current                       string              `json:"current"`
	Version                       string              `json:"version"`
	BaselineTextures              int                 `json:"baselineTextures"`
	CurrentTextures               int                 `json:"currentTextures"`
	SharedTextures                int                 `json:"sharedTextures"`
	BaselineOnly                  []string            `json:"baselineOnly"`
	CurrentOnly                   []string            `json:"currentOnly"`
	BaselineOnlyStillInJAR        []string            `json:"baselineOnlyStillInJAR"`
	BeforeMissingBaselineTextures []string            `json:"beforeMissingBaselineTextures"`
	CurrentJARBlockPNGs           int                 `json:"currentJARBlockPNGs"`
	MissingCurrentBlockPNGs       []jmodel.ResourceID `json:"missingCurrentBlockPNGs"`
	MissingRequiredTextures       []jmodel.ResourceID `json:"missingRequiredTextures"`
	BeforeMissingGeometry         []jmodel.ResourceID `json:"beforeMissingGeometry"`
	AfterMissingGeometry          []jmodel.ResourceID `json:"afterMissingGeometry"`
	CurrentPNGSize                [2]int              `json:"currentPNGSize"`
	BoundsChecked                 bool                `json:"boundsChecked"`
}

func geometry(blockstates jmodel.RuleTable, table jmodel.Models) (map[jmodel.ResourceID]bool, error) {
	resolver := jmodel.NewResolver(table)
	visible := map[jmodel.ResourceID]bool{}
	for name, state := range blockstates {
		id, err := jmodel.ParseResourceID(string(name))
		if err != nil {
			return nil, err
		}
		visible[id] = false
		for _, application := range state.Applications() {
			modelID, err := jmodel.ParseResourceID(string(application.Model))
			if err != nil {
				return nil, err
			}
			resolved, err := resolver.Resolve(modelID)
			if err != nil {
				return nil, err
			}
			for _, element := range resolved.Elements {
				if len(element.Faces) > 0 {
					visible[id] = true
				}
			}
		}
	}
	return visible, nil
}

func Run(opt Options) (*Report, error) {
	baseline, oldStates, oldModels, err := readBaseline(opt.BaselineDir)
	if err != nil {
		return nil, err
	}
	current, err := jmodel.ReadAtlas(filepath.Join(opt.CurrentDir, "blocksAtlases.json"))
	if err != nil {
		return nil, err
	}
	snapshot, err := jsonfile.Read[asset.Snapshot](filepath.Join(opt.SnapshotDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	required, err := jsonfile.Read[asset.ModelIndex](filepath.Join(opt.CompletedDir, "model-index.json"))
	if err != nil {
		return nil, err
	}
	rawIndex, err := jsonfile.Read[asset.ModelIndex](filepath.Join(opt.RawModelDir, "model-index.json"))
	if err != nil {
		return nil, err
	}
	// Reconstruct the atlas selection before s04: all block PNGs and textures
	// referenced by the raw models. This makes restored entity textures auditable.
	jarTextures := map[string]bool{}
	rawSelected := map[string]bool{}
	for id := range snapshot.Textures {
		jarTextures[string(id)] = true
		if strings.HasPrefix(id.Path(), "block/") || strings.HasPrefix(id.Path(), "blocks/") {
			rawSelected[string(id)] = true
		}
	}
	for _, id := range rawIndex.RequiredTextures {
		rawSelected[string(id)] = true
	}
	report := &Report{
		Baseline:                      opt.BaselineDir,
		Current:                       opt.CurrentDir,
		Version:                       snapshot.Version,
		BaselineTextures:              len(baseline.Textures),
		CurrentTextures:               len(current.Textures),
		BaselineOnly:                  []string{},
		CurrentOnly:                   []string{},
		BaselineOnlyStillInJAR:        []string{},
		BeforeMissingBaselineTextures: []string{},
		MissingCurrentBlockPNGs:       []jmodel.ResourceID{},
		MissingRequiredTextures:       []jmodel.ResourceID{},
		BeforeMissingGeometry:         []jmodel.ResourceID{},
		AfterMissingGeometry:          []jmodel.ResourceID{},
	}
	for name := range baseline.Textures {
		if jarTextures[string(name)] && !rawSelected[string(name)] {
			report.BeforeMissingBaselineTextures = append(report.BeforeMissingBaselineTextures, string(name))
		}
		if _, ok := current.Textures[name]; ok {
			report.SharedTextures++
			continue
		}
		report.BaselineOnly = append(report.BaselineOnly, string(name))
		if jarTextures[string(name)] {
			report.BaselineOnlyStillInJAR = append(report.BaselineOnlyStillInJAR, string(name))
		}
	}
	for name := range current.Textures {
		if _, ok := baseline.Textures[name]; !ok {
			report.CurrentOnly = append(report.CurrentOnly, string(name))
		}
	}
	for id := range snapshot.Textures {
		if strings.HasPrefix(id.Path(), "block/") || strings.HasPrefix(id.Path(), "blocks/") {
			report.CurrentJARBlockPNGs++
			if _, ok := current.Textures[id]; !ok {
				report.MissingCurrentBlockPNGs = append(report.MissingCurrentBlockPNGs, id)
			}
		}
	}
	for _, id := range required.RequiredTextures {
		if _, ok := current.Textures[id]; !ok {
			report.MissingRequiredTextures = append(report.MissingRequiredTextures, id)
		}
	}
	newStates, newModels, err := readTables(opt.CurrentDir)
	if err != nil {
		return nil, err
	}
	rawStates, rawModels, err := readTables(opt.RawModelDir)
	if err != nil {
		return nil, err
	}
	oldVisible, err := geometry(oldStates, oldModels)
	if err != nil {
		return nil, err
	}
	newVisible, err := geometry(newStates, newModels)
	if err != nil {
		return nil, err
	}
	rawVisible, err := geometry(rawStates, rawModels)
	if err != nil {
		return nil, err
	}
	for id, visible := range oldVisible {
		if !visible {
			continue
		}
		if now, exists := rawVisible[id]; exists && !now {
			report.BeforeMissingGeometry = append(report.BeforeMissingGeometry, id)
		}
		if now, exists := newVisible[id]; exists && !now {
			report.AfterMissingGeometry = append(report.AfterMissingGeometry, id)
		}
	}
	file, err := os.Open(filepath.Join(opt.CurrentDir, "blocksAtlasLatest.png"))
	if err != nil {
		return nil, err
	}
	config, err := png.DecodeConfig(file)
	file.Close()
	if err != nil {
		return nil, err
	}
	report.CurrentPNGSize = [2]int{config.Width, config.Height}
	if config.Width != current.Width || config.Height != current.Height {
		return nil, fmt.Errorf("PNG and atlas table dimensions differ")
	}
	bounds := image.Rect(0, 0, config.Width, config.Height)
	for name, texture := range current.Textures {
		su, sv := texture.SU, texture.SV
		if su == 0 {
			su = float64(current.TileSize) / float64(current.Width)
		}
		if sv == 0 {
			sv = float64(current.TileSize) / float64(current.Height)
		}
		x := int(math.Round(texture.U * float64(config.Width)))
		y := int(math.Round(texture.V * float64(config.Height)))
		width := int(math.Round(su * float64(config.Width)))
		height := int(math.Round(sv * float64(config.Height)))
		if width <= 0 || height <= 0 || !image.Rect(x, y, x+width, y+height).In(bounds) {
			return nil, fmt.Errorf("invalid atlas coordinates for %s", string(name))
		}
	}
	report.BoundsChecked = true
	slices.Sort(report.BaselineOnly)
	slices.Sort(report.CurrentOnly)
	slices.Sort(report.BaselineOnlyStillInJAR)
	slices.Sort(report.BeforeMissingBaselineTextures)
	slices.Sort(report.MissingCurrentBlockPNGs)
	slices.Sort(report.MissingRequiredTextures)
	slices.Sort(report.BeforeMissingGeometry)
	slices.Sort(report.AfterMissingGeometry)
	if err := jsonfile.Write(opt.OutputPath, report); err != nil {
		return nil, err
	}
	return report, nil
}

func readTables(dir string) (jmodel.RuleTable, jmodel.Models, error) {
	states, err := jmodel.ReadRules(filepath.Join(dir, "blockstates.json"))
	if err != nil {
		return nil, nil, err
	}
	models, err := jmodel.ReadModels(filepath.Join(dir, "models.json"))
	return states, models, err
}

// Only the comparison baseline may use the historical viewer format. New builds
// and the shared model reader use the split, single-version format with full IDs.
func readBaseline(dir string) (jmodel.Atlas, jmodel.RuleTable, jmodel.Models, error) {
	path := filepath.Join(dir, "blockStatesModels.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		atlas, err := jmodel.ReadAtlas(filepath.Join(dir, "blocksAtlases.json"))
		if err != nil {
			return atlas, nil, nil, err
		}
		states, models, err := readTables(dir)
		return atlas, states, models, err
	} else if err != nil {
		return jmodel.Atlas{}, nil, nil, err
	}
	tables, err := jsonfile.Read[struct {
		BlockStates map[string]jmodel.RuleTable `json:"blockstates"`
		Models      map[string]jmodel.Models    `json:"models"`
	}](path)
	if err != nil {
		return jmodel.Atlas{}, nil, nil, err
	}
	atlas, err := jsonfile.Read[struct {
		Latest jmodel.Atlas `json:"latest"`
	}](filepath.Join(dir, "blocksAtlases.json"))
	if err != nil {
		return jmodel.Atlas{}, nil, nil, err
	}
	// The historical mc-assets atlas stripped block/ (including nested block
	// paths). Expand that archive convention here, outside the shared reader.
	textures := make(map[jmodel.ResourceID]jmodel.AtlasTexture, len(atlas.Latest.Textures))
	for id, entry := range atlas.Latest.Textures {
		path := id.Path()
		if !strings.HasPrefix(path, "entity/") {
			id = jmodel.ResourceID(id.Namespace() + ":block/" + path)
		}
		textures[id] = entry
	}
	atlas.Latest.Textures = textures
	return atlas.Latest, tables.BlockStates["latest"], tables.Models["latest"], nil
}
