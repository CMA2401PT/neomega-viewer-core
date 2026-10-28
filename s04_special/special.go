// SPDX-License-Identifier: MIT
// Package s04_special classifies render requirements and supplies community
// static previews for game-rendered block entities. It preserves runtime needs
// separately rather than claiming a closed chest is a fully animated renderer.
package s04_special

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"neomega-assets/asset"
	"neomega-assets/internal/jsonfile"
	"neomega-assets/internal/resourcefiles"
	"neomega-assets/s03_models"
	mcassets "neomega-assets/third_party/mc-assets"
)

type Options struct {
	InputDir  string
	ModelDir  string
	OutputDir string
}

func Run(ctx context.Context, opt Options) (*asset.Renderers, error) {
	inputPath, err := filepath.Abs(opt.InputDir)
	if err != nil {
		return nil, err
	}
	outputPath, err := filepath.Abs(opt.OutputDir)
	if err != nil {
		return nil, err
	}
	if inputPath == outputPath {
		return nil, fmt.Errorf("special-render output must differ from input")
	}
	snapshot, err := jsonfile.Read[asset.Snapshot](filepath.Join(opt.InputDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	report, err := jsonfile.Read[asset.ModelIndex](filepath.Join(opt.ModelDir, "models.json"))
	if err != nil {
		return nil, err
	}
	tables, err := jsonfile.Read[asset.Tables](filepath.Join(opt.ModelDir, "blockStatesModels.json"))
	if err != nil {
		return nil, err
	}
	if report.GeometrylessBlockStates == nil {
		return nil, fmt.Errorf("model report has no geometry coverage; rerun models before special")
	}
	if snapshot.Version != report.Version {
		return nil, fmt.Errorf("model and snapshot versions differ")
	}
	if err := resourcefiles.CopySnapshot(ctx, snapshot, opt.InputDir, opt.OutputDir); err != nil {
		return nil, err
	}
	index, err := mcassets.Load()
	if err != nil {
		return nil, err
	}
	importer := templateImporter{
		index:          index,
		snapshot:       &snapshot,
		output:         opt.OutputDir,
		importing:      map[asset.ID]bool{},
		neededTextures: map[asset.ID]bool{},
	}
	renderers := &asset.Renderers{
		Version:        snapshot.Version,
		Blocks:         map[asset.ID]asset.BlockRenderer{},
		FamilyTextures: map[string][]asset.ID{},
		Counts:         map[asset.RendererKind]int{},
	}
	empty := map[asset.ID]bool{}
	for _, id := range report.GeometrylessBlockStates {
		empty[id] = true
	}
	templated := false
	for name, state := range tables.BlockStates["latest"] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := asset.ParseID(name)
		if err != nil {
			return nil, err
		}
		profile := asset.BlockRenderer{Kind: asset.RendererModel, StaticPreview: true, StateProperties: stateProperties(state)}
		if !empty[id] {
			if strings.HasSuffix(id.Path(), "_sign") {
				profile.Family = "sign"
				profile.DynamicFields = []string{"frontText", "backText"}
			}
		} else if known, ok := classifyEmpty(id); ok {
			profile = known
			profile.StateProperties = stateProperties(state)
		} else {
			state, err = importer.state(id)
			if err != nil {
				return nil, fmt.Errorf("block has no static geometry or supported renderer: %w", err)
			}
			profile = specialProfile(id)
			profile.StateProperties = stateProperties(state)
			profile.TemplateSource = index.Repository + "@" + index.Revision + "/custom/blockentities/latest/blockStates/" + strings.TrimPrefix(index.States[id], "states/")
			relative := "assets/" + id.Namespace() + "/blockstates/" + id.Path() + ".json"
			if err := jsonfile.Write(filepath.Join(opt.OutputDir, filepath.FromSlash(relative)), state); err != nil {
				return nil, err
			}
			snapshot.BlockStates[id] = asset.File{Path: relative, Origin: profile.TemplateSource}
			templated = true
		}
		renderers.Blocks[id] = profile
		renderers.Counts[profile.Kind]++
	}
	if templated {
		if err := importer.textures(); err != nil {
			return nil, err
		}
		snapshot.Sources = append(snapshot.Sources, asset.Source{Kind: "community-static-previews", Location: index.Repository + "/tree/" + index.Revision + "/custom", License: "MIT; original model credits retained; Minecraft-derived material retains original rights"})
	}
	renderers.Sources = snapshot.Sources
	for _, profile := range renderers.Blocks {
		if _, exists := renderers.FamilyTextures[profile.Family]; exists {
			continue
		}
		prefixes := familyPrefixes[profile.Family]
		if len(prefixes) == 0 {
			continue
		}
		for id := range snapshot.Textures {
			if id.Namespace() != "minecraft" {
				continue
			}
			for _, prefix := range prefixes {
				if strings.HasPrefix(id.Path(), prefix) {
					renderers.FamilyTextures[profile.Family] = append(renderers.FamilyTextures[profile.Family], id)
					break
				}
			}
		}
		slices.Sort(renderers.FamilyTextures[profile.Family])
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "snapshot.json"), snapshot); err != nil {
		return nil, err
	}
	completed, err := s03_models.Run(s03_models.Options{InputDir: opt.OutputDir, OutputDir: opt.OutputDir})
	if err != nil {
		return nil, err
	}
	// Keep the dynamic family's resources even when a closed/static preview
	// does not directly sample them.
	required := map[asset.ID]bool{}
	for _, id := range completed.RequiredTextures {
		required[id] = true
	}
	for _, ids := range renderers.FamilyTextures {
		for _, id := range ids {
			required[id] = true
		}
	}
	completed.RequiredTextures = completed.RequiredTextures[:0]
	for id := range required {
		if _, ok := snapshot.Textures[id]; !ok {
			return nil, fmt.Errorf("special renderer references missing texture %s", id)
		}
		completed.RequiredTextures = append(completed.RequiredTextures, id)
	}
	slices.Sort(completed.RequiredTextures)
	for _, id := range completed.GeometrylessBlockStates {
		profile := renderers.Blocks[id]
		if profile.StaticPreview {
			return nil, fmt.Errorf("static preview for %s has no geometry", id)
		}
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "models.json"), completed); err != nil {
		return nil, err
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "blockRenderers.json"), renderers); err != nil {
		return nil, err
	}
	// A short, explicit stage marker helps manual pipelines identify the result.
	if err := os.WriteFile(filepath.Join(opt.OutputDir, "README.txt"), []byte("Completed model previews and blockRenderers.json. Feed this directory to s05_atlas.\n"), 0644); err != nil {
		return nil, err
	}
	return renderers, nil
}
