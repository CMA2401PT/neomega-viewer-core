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

	"neomega-viewer-core/viewer-assets/asset"
	"neomega-viewer-core/viewer-assets/resources"
	mcassets "neomega-viewer-core/viewer-assets/resources/third_party/mc-assets"
	"neomega-viewer-core/viewer-assets/s03_models"
	"neomega-viewer-core/viewer-assets/utils/jsonfile"
	"neomega-viewer-core/viewer-assets/utils/resourcefiles"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
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
	report, err := jsonfile.Read[asset.ModelIndex](filepath.Join(opt.ModelDir, "model-index.json"))
	if err != nil {
		return nil, err
	}
	blockstates, err := jsonfile.Read[jmodel.RuleTable](filepath.Join(opt.ModelDir, "blockstates.json"))
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
		importing:      map[jmodel.ResourceID]bool{},
		neededTextures: map[jmodel.ResourceID]bool{},
	}
	renderers := &asset.Renderers{
		Version:        snapshot.Version,
		Blocks:         map[jmodel.ResourceID]jmodel.BlockRenderer{},
		FamilyTextures: map[string][]jmodel.ResourceID{},
		Counts:         map[jmodel.RendererKind]int{},
	}
	empty := map[jmodel.ResourceID]bool{}
	for _, id := range report.GeometrylessBlockStates {
		empty[id] = true
	}
	templated := false
	for name, state := range blockstates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := jmodel.ParseResourceID(string(name))
		if err != nil {
			return nil, err
		}
		var profile jmodel.BlockRenderer
		if !empty[id] {
			profile = resources.ModelRenderer(id)
		} else if known, ok := classifyEmpty(id); ok {
			profile = known
		} else {
			state, err = importer.state(id)
			if err != nil {
				return nil, fmt.Errorf("block has no static geometry or supported renderer: %w", err)
			}
			profile = resources.SpecialRenderer(id)
			profile.TemplateSource = index.StateOrigin(id)
			relative := "assets/" + id.Namespace() + "/blockstates/" + id.Path() + ".json"
			if err := jsonfile.Write(filepath.Join(opt.OutputDir, filepath.FromSlash(relative)), state); err != nil {
				return nil, err
			}
			snapshot.BlockStates[id] = asset.File{Path: relative, Origin: profile.TemplateSource}
			templated = true
		}
		profile.StateProperties = stateProperties(state)
		renderers.Blocks[id] = profile
		renderers.Counts[profile.Kind]++
	}
	if templated {
		if err := importer.textures(); err != nil {
			return nil, err
		}
		snapshot.Sources = append(snapshot.Sources, index.Source())
	}
	renderers.Sources = snapshot.Sources
	families := map[string]bool{}
	for _, profile := range renderers.Blocks {
		if families[profile.Family] {
			continue
		}
		families[profile.Family] = true
		for id := range snapshot.Textures {
			if resources.FamilyUsesTexture(profile.Family, id) {
				renderers.FamilyTextures[profile.Family] = append(renderers.FamilyTextures[profile.Family], id)
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
	required := map[jmodel.ResourceID]bool{}
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
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "model-index.json"), completed); err != nil {
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
