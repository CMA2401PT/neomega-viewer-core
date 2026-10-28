// SPDX-License-Identifier: MIT
// Package s03_models builds typed model/blockstate tables from patched assets.
// Resource naming follows mc-assets (MIT); each output is a single-version table.
package s03_models

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"neomega-assets/asset"
	"neomega-assets/utils/jsonfile"
)

type Options struct {
	InputDir  string
	OutputDir string
}

func canonicalModelRef(value asset.ID, models map[asset.ID]asset.Model) (asset.ID, error) {
	id, err := asset.ParseID(string(value))
	if err != nil {
		return "", err
	}
	// Some supplied block-entity templates use a short parent name.
	if _, ok := models[id]; !ok && !strings.HasPrefix(id.Path(), "builtin/") {
		candidate := asset.ID(id.Namespace() + ":block/" + id.Path())
		if _, ok := models[candidate]; ok {
			id = candidate
		}
	}
	return id, nil
}

func canonicalTexture(ref asset.TextureRef) (asset.TextureRef, error) {
	if strings.HasPrefix(string(ref), "#") {
		return ref, nil
	}
	id, err := asset.ParseID(string(ref))
	return asset.TextureRef(id), err
}

func normalizeModel(model asset.Model, models map[asset.ID]asset.Model) (asset.Model, error) {
	var err error
	if model.Parent != "" {
		model.Parent, err = canonicalModelRef(model.Parent, models)
		if err != nil {
			return model, err
		}
	}
	for key, value := range model.Textures {
		value.Sprite, err = canonicalTexture(value.Sprite)
		model.Textures[key] = value
		if err != nil {
			return model, err
		}
	}
	if model.Elements != nil {
		for i := range *model.Elements {
			element := &(*model.Elements)[i]
			for side, face := range element.Faces {
				// Vanilla heavy_core uses an unprefixed local texture slot.
				// Normalize the slot spelling without changing its meaning.
				if _, local := model.Textures[string(face.Texture)]; local {
					face.Texture = asset.TextureRef("#" + string(face.Texture))
				}
				face.Texture, err = canonicalTexture(face.Texture)
				if err != nil {
					return model, err
				}
				element.Faces[side] = face
			}
		}
	}
	for i := range model.Overrides {
		model.Overrides[i].Model, err = canonicalModelRef(model.Overrides[i].Model, models)
		if err != nil {
			return model, err
		}
	}
	return model, nil
}

func normalizeState(state asset.BlockState, models map[asset.ID]asset.Model) (asset.BlockState, error) {
	normalize := func(choice *asset.Choice) error {
		if len(choice.Options) == 0 {
			return fmt.Errorf("empty model alternatives")
		}
		for i := range choice.Options {
			id, err := canonicalModelRef(choice.Options[i].Model, models)
			if err != nil {
				return err
			}
			choice.Options[i].Model = id
		}
		return nil
	}
	for key, choice := range state.Variants {
		if err := normalize(&choice); err != nil {
			return state, err
		}
		state.Variants[key] = choice
	}
	for i := range state.Multipart {
		if err := normalize(&state.Multipart[i].Apply); err != nil {
			return state, err
		}
	}
	return state, nil
}

func exportTexture(ref asset.TextureRef) asset.TextureRef {
	if strings.HasPrefix(string(ref), "#") {
		return ref
	}
	return asset.TextureRef(asset.ID(ref).ExportName())
}
func exportModel(input asset.Model) asset.Model {
	// Clone before stripping namespaces so the resolver's canonical catalog stays intact.
	raw, _ := json.Marshal(input)
	var model asset.Model
	json.Unmarshal(raw, &model)
	if model.Parent != "" {
		model.Parent = asset.ID(model.Parent.ExportName())
	}
	for key, value := range model.Textures {
		value.Sprite = exportTexture(value.Sprite)
		model.Textures[key] = value
	}
	if model.Elements != nil {
		for i := range *model.Elements {
			for side, face := range (*model.Elements)[i].Faces {
				face.Texture = exportTexture(face.Texture)
				(*model.Elements)[i].Faces[side] = face
			}
		}
	}
	for i := range model.Overrides {
		model.Overrides[i].Model = asset.ID(model.Overrides[i].Model.ExportName())
	}
	return model
}
func exportState(state asset.BlockState) asset.BlockState {
	for key, choice := range state.Variants {
		for i := range choice.Options {
			choice.Options[i].Model = asset.ID(choice.Options[i].Model.ExportName())
		}
		state.Variants[key] = choice
	}
	for i := range state.Multipart {
		for j := range state.Multipart[i].Apply.Options {
			choice := &state.Multipart[i].Apply.Options[j]
			choice.Model = asset.ID(choice.Model.ExportName())
		}
	}
	return state
}

func Run(opt Options) (*asset.ModelIndex, error) {
	snapshot, err := jsonfile.Read[asset.Snapshot](filepath.Join(opt.InputDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	models := map[asset.ID]asset.Model{}
	states := map[asset.ID]asset.BlockState{}
	for id, file := range snapshot.Models {
		value, err := jsonfile.Read[asset.Model](filepath.Join(opt.InputDir, filepath.FromSlash(file.Path)))
		if err != nil {
			return nil, err
		}
		models[id] = value
	}
	for id, value := range models {
		normalized, err := normalizeModel(value, models)
		if err != nil {
			return nil, fmt.Errorf("model %s: %w", id, err)
		}
		models[id] = normalized
	}
	for id, file := range snapshot.BlockStates {
		value, err := jsonfile.Read[asset.BlockState](filepath.Join(opt.InputDir, filepath.FromSlash(file.Path)))
		if err != nil {
			return nil, err
		}
		normalized, err := normalizeState(value, models)
		if err != nil {
			return nil, fmt.Errorf("blockstate %s: %w", id, err)
		}
		states[id] = normalized
	}
	resolver := NewResolver(models)
	required := map[asset.ID]bool{}
	builtins := map[asset.ID]bool{}
	geometryless := []asset.ID{}
	for block, state := range states {
		visible := false
		for _, application := range state.Applications() {
			model, err := resolver.Resolve(application.Model)
			if err != nil {
				return nil, fmt.Errorf("block %s: %w", block, err)
			}
			if model.Builtin != "" {
				builtins[application.Model] = true
			}
			for _, element := range model.Elements {
				if len(element.Faces) > 0 {
					visible = true
				}
				for _, face := range element.Faces {
					texture, err := model.Texture(face.Texture)
					if err != nil {
						return nil, fmt.Errorf("block %s, model %s: %w", block, application.Model, err)
					}
					required[texture] = true
				}
			}
		}
		if !visible {
			geometryless = append(geometryless, block)
		}
	}
	slices.Sort(geometryless)
	report := &asset.ModelIndex{Version: snapshot.Version, ModelCount: len(models), BlockStateCount: len(states), GeometrylessBlockStates: geometryless}
	for id := range required {
		report.RequiredTextures = append(report.RequiredTextures, id)
	}
	for id := range builtins {
		report.BuiltinModels = append(report.BuiltinModels, id)
	}
	slices.Sort(report.RequiredTextures)
	slices.Sort(report.BuiltinModels)
	modelTable := asset.Models{}
	blockstates := asset.BlockStates{}
	for id, model := range models {
		modelTable[id.ExportName()] = exportModel(model)
	}
	for id, state := range states {
		blockstates[id.ExportName()] = exportState(state)
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "blockstates.json"), blockstates); err != nil {
		return nil, err
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "models.json"), modelTable); err != nil {
		return nil, err
	}
	if err := jsonfile.Write(filepath.Join(opt.OutputDir, "model-index.json"), report); err != nil {
		return nil, err
	}
	return report, nil
}
