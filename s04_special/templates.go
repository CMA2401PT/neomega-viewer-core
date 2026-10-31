// SPDX-License-Identifier: MIT
package s04_special

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"neomega-assets/asset"
	"neomega-assets/resources"
	mcassets "neomega-assets/resources/third_party/mc-assets"
	"neomega-assets/utils/jsonfile"
	jmodel "neomega-blocks/geometry/java_block_model"
)

type templateImporter struct {
	index          mcassets.Index
	snapshot       *asset.Snapshot
	output         string
	importing      map[jmodel.ResourceID]bool
	neededTextures map[jmodel.ResourceID]bool
}

func (importer *templateImporter) modelReference(value jmodel.ResourceID) (jmodel.ResourceID, error) {
	id, err := resources.NormalizeModelReference(value)
	if err != nil {
		return "", err
	}
	if _, ok := importer.index.Models[id]; ok {
		target := resources.ImportedModelID(id)
		if err := importer.importModel(id, target); err != nil {
			return "", err
		}
		return target, nil
	}
	return id, nil
}

func (importer *templateImporter) saveModel(id jmodel.ResourceID, model jmodel.Model, origin string) error {
	relative := "assets/" + id.Namespace() + "/models/" + id.Path() + ".json"
	if err := jsonfile.Write(filepath.Join(importer.output, filepath.FromSlash(relative)), model); err != nil {
		return err
	}
	importer.snapshot.Models[id] = asset.File{Path: relative, Origin: origin}
	return nil
}

func (importer *templateImporter) importModel(source, target jmodel.ResourceID) error {
	if _, ok := importer.snapshot.Models[target]; ok {
		return nil
	}
	if importer.importing[source] {
		return fmt.Errorf("template parent cycle at %s", source)
	}
	importer.importing[source] = true
	defer delete(importer.importing, source)
	path := importer.index.Models[source]
	raw, err := mcassets.Read(path)
	if err != nil {
		return err
	}
	var model jmodel.Model
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
		binding.Sprite, err = resources.NormalizeTextureReference(binding.Sprite, importer.snapshot.Textures)
		if err != nil {
			return err
		}
		model.Textures[key] = binding
		if !strings.HasPrefix(string(binding.Sprite), "#") {
			importer.neededTextures[jmodel.ResourceID(binding.Sprite)] = true
		}
	}
	if model.Elements != nil {
		for i := range *model.Elements {
			for side, face := range (*model.Elements)[i].Faces {
				face.Texture, err = resources.NormalizeTextureReference(face.Texture, importer.snapshot.Textures)
				if err != nil {
					return err
				}
				(*model.Elements)[i].Faces[side] = face
			}
		}
	}
	return importer.saveModel(target, model, importer.index.ModelOrigin(path))
}

func (importer *templateImporter) state(block jmodel.ResourceID) (jmodel.ModelRules, error) {
	path, ok := importer.index.States[block]
	if !ok {
		return jmodel.ModelRules{}, fmt.Errorf("no special model template for %s", block)
	}
	raw, err := mcassets.Read(path)
	if err != nil {
		return jmodel.ModelRules{}, err
	}
	var state jmodel.ModelRules
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	rewrite := func(choice *jmodel.Choice) error {
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
	if variant, ok := resources.TemplateVariantFor(block); ok {
		state, err = importer.modelVariant(variant, state)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}

// modelVariant applies the resource rule without knowing which block requested it.
func (importer *templateImporter) modelVariant(variant resources.TemplateVariant, state jmodel.ModelRules) (jmodel.ModelRules, error) {
	file, ok := importer.snapshot.Models[variant.Source]
	if !ok {
		return state, fmt.Errorf("missing preview source model %s", variant.Source)
	}
	model, err := jsonfile.Read[jmodel.Model](filepath.Join(importer.output, filepath.FromSlash(file.Path)))
	if err != nil {
		return state, err
	}
	for slot, texture := range variant.Textures {
		if _, ok := importer.snapshot.Textures[texture]; !ok {
			return state, fmt.Errorf("missing preview texture %s", texture)
		}
		model.Textures[slot] = jmodel.TextureBinding{Sprite: jmodel.TextureRef(texture)}
	}
	if variant.ClearTint && model.Elements != nil {
		for i := range *model.Elements {
			for side, face := range (*model.Elements)[i].Faces {
				face.TintIndex = nil
				(*model.Elements)[i].Faces[side] = face
			}
		}
	}
	if err := importer.saveModel(variant.Target, model, file.Origin+variant.OriginNote); err != nil {
		return state, err
	}
	replace := func(choice *jmodel.Choice) {
		for i := range choice.Options {
			if choice.Options[i].Model == variant.Source {
				choice.Options[i].Model = variant.Target
			}
		}
	}
	for key, choice := range state.Variants {
		replace(&choice)
		state.Variants[key] = choice
	}
	for i := range state.Multipart {
		replace(&state.Multipart[i].Apply)
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
		data, err := mcassets.Read(path)
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
		importer.snapshot.Textures[id] = asset.File{Path: relative, Origin: importer.index.TextureOrigin(path)}
	}
	return nil
}
