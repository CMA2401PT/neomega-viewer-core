// SPDX-License-Identifier: MIT
package s04_special

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"neomega-assets/asset"
	"neomega-assets/internal/jsonfile"
	mcassets "neomega-assets/third_party/mc-assets"
)

type templateImporter struct {
	index          mcassets.Index
	snapshot       *asset.Snapshot
	output         string
	importing      map[asset.ID]bool
	neededTextures map[asset.ID]bool
}

func sourceModel(value asset.ID) (asset.ID, error) {
	id, err := asset.ParseID(string(value))
	if err != nil {
		return "", err
	}
	path := id.Path()
	if !strings.HasPrefix(path, "block/") && !strings.HasPrefix(path, "item/") && !strings.HasPrefix(path, "builtin/") {
		path = "block/" + path
	}
	return asset.ID(id.Namespace() + ":" + path), nil
}

func (importer *templateImporter) modelReference(value asset.ID) (asset.ID, error) {
	id, err := sourceModel(value)
	if err != nil {
		return "", err
	}
	if _, ok := importer.index.Models[id]; ok {
		target := asset.ID("neomega:" + id.Path())
		if err := importer.importModel(id, target); err != nil {
			return "", err
		}
		return target, nil
	}
	return id, nil
}

func templateTexture(value asset.TextureRef) (asset.TextureRef, error) {
	if strings.HasPrefix(string(value), "#") {
		return value, nil
	}
	id, err := asset.ParseID(string(value))
	if err != nil {
		return "", err
	}
	// This entity texture moved in modern client resources.
	if id == "minecraft:entity/banner_base" {
		id = "minecraft:entity/banner/banner_base"
	}
	return asset.TextureRef(id), nil
}

func (importer *templateImporter) saveModel(id asset.ID, model asset.Model, origin string) error {
	relative := "assets/" + id.Namespace() + "/models/" + id.Path() + ".json"
	if err := jsonfile.Write(filepath.Join(importer.output, filepath.FromSlash(relative)), model); err != nil {
		return err
	}
	importer.snapshot.Models[id] = asset.File{Path: relative, Origin: origin}
	return nil
}

func (importer *templateImporter) importModel(source, target asset.ID) error {
	if _, ok := importer.snapshot.Models[target]; ok {
		return nil
	}
	if importer.importing[source] {
		return fmt.Errorf("template parent cycle at %s", source)
	}
	importer.importing[source] = true
	defer delete(importer.importing, source)
	path := importer.index.Models[source]
	raw, err := mcassets.Files.ReadFile("templates/" + path)
	if err != nil {
		return err
	}
	var model asset.Model
	if err := json.Unmarshal(raw, &model); err != nil {
		return err
	}
	if model.Parent != "" {
		model.Parent, err = importer.modelReference(model.Parent)
		if err != nil {
			return err
		}
	}
	for key, binding := range model.Textures {
		binding.Sprite, err = templateTexture(binding.Sprite)
		if err != nil {
			return err
		}
		model.Textures[key] = binding
		if !strings.HasPrefix(string(binding.Sprite), "#") {
			importer.neededTextures[asset.ID(binding.Sprite)] = true
		}
	}
	if model.Elements != nil {
		for i := range *model.Elements {
			for side, face := range (*model.Elements)[i].Faces {
				face.Texture, err = templateTexture(face.Texture)
				if err != nil {
					return err
				}
				(*model.Elements)[i].Faces[side] = face
			}
		}
	}
	return importer.saveModel(target, model, importer.index.Repository+"@"+importer.index.Revision+"/custom/blockentities/latest/blockModels/"+strings.TrimPrefix(path, "models/"))
}

func (importer *templateImporter) state(block asset.ID) (asset.BlockState, error) {
	path, ok := importer.index.States[block]
	if !ok {
		return asset.BlockState{}, fmt.Errorf("no special model template for %s", block)
	}
	raw, err := mcassets.Files.ReadFile("templates/" + path)
	if err != nil {
		return asset.BlockState{}, err
	}
	var state asset.BlockState
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	rewrite := func(choice *asset.Choice) error {
		for i := range choice.Options {
			id, err := importer.modelReference(choice.Options[i].Model)
			if err != nil {
				return err
			}
			choice.Options[i].Model = id
		}
		return nil
	}
	for key, choice := range state.Variants {
		if err := rewrite(&choice); err != nil {
			return state, err
		}
		state.Variants[key] = choice
	}
	for i := range state.Multipart {
		if err := rewrite(&state.Multipart[i].Apply); err != nil {
			return state, err
		}
	}
	if strings.HasSuffix(block.Path(), "_shulker_box") {
		state, err = importer.shulkerColor(block, state)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}

func (importer *templateImporter) shulkerColor(block asset.ID, state asset.BlockState) (asset.BlockState, error) {
	color := strings.TrimSuffix(block.Path(), "_shulker_box")
	source := asset.ID("neomega:block/entity/dyed_shulker_box")
	file, ok := importer.snapshot.Models[source]
	if !ok {
		return state, fmt.Errorf("missing dyed shulker template")
	}
	model, err := jsonfile.Read[asset.Model](filepath.Join(importer.output, filepath.FromSlash(file.Path)))
	if err != nil {
		return state, err
	}
	texture := asset.ID("minecraft:entity/shulker/shulker_" + color)
	if _, ok := importer.snapshot.Textures[texture]; !ok {
		return state, fmt.Errorf("missing shulker skin %s", texture)
	}
	model.Textures["0"] = asset.TextureBinding{Sprite: asset.TextureRef(texture)}
	// Use the actual colored skin once, rather than tinting it a second time.
	if model.Elements != nil {
		for i := range *model.Elements {
			for side, face := range (*model.Elements)[i].Faces {
				face.TintIndex = nil
				(*model.Elements)[i].Faces[side] = face
			}
		}
	}
	target := asset.ID("neomega:block/preview/" + block.Path())
	if err := importer.saveModel(target, model, file.Origin+" (selected "+color+" skin)"); err != nil {
		return state, err
	}
	for key, choice := range state.Variants {
		for i := range choice.Options {
			if choice.Options[i].Model == source {
				choice.Options[i].Model = target
			}
		}
		state.Variants[key] = choice
	}
	return state, nil
}

func (importer *templateImporter) textures() error {
	for id, path := range importer.index.Textures {
		if !importer.neededTextures[id] {
			continue
		}
		// Explicit resource-pack overrides take precedence over bundled defaults.
		if _, exists := importer.snapshot.Textures[id]; exists {
			continue
		}
		relative := "assets/" + id.Namespace() + "/textures/" + id.Path() + ".png"
		data, err := mcassets.Files.ReadFile("templates/" + path)
		if err != nil {
			return err
		}
		destination := filepath.Join(importer.output, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(destination, data, 0644); err != nil {
			return err
		}
		importer.snapshot.Textures[id] = asset.File{Path: relative, Origin: importer.index.Repository + "@" + importer.index.Revision + "/custom/" + path}
	}
	return nil
}
