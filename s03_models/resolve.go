// SPDX-License-Identifier: MIT
package s03_models

import (
	"fmt"
	"maps"
	"strings"

	"neomega-assets/asset"
)

// ResolvedModel is an inspection view of a parent chain. Output tables retain
// the chain so consumers can still apply variants, rotations and multipart rules.
type ResolvedModel struct {
	Elements []asset.Element
	Textures map[string]asset.TextureBinding
	Builtin  asset.ID
}

type Resolver struct {
	Models   map[asset.ID]asset.Model
	cache    map[asset.ID]ResolvedModel
	visiting map[asset.ID]bool
}

func NewResolver(models map[asset.ID]asset.Model) *Resolver {
	return &Resolver{Models: models, cache: map[asset.ID]ResolvedModel{}, visiting: map[asset.ID]bool{}}
}

func (resolver *Resolver) Resolve(id asset.ID) (ResolvedModel, error) {
	if cached, ok := resolver.cache[id]; ok {
		return cached, nil
	}
	if resolver.visiting[id] {
		return ResolvedModel{}, fmt.Errorf("cyclic model parent at %s", id)
	}
	if strings.HasPrefix(id.Path(), "builtin/") {
		return ResolvedModel{Builtin: id, Textures: map[string]asset.TextureBinding{}}, nil
	}
	model, ok := resolver.Models[id]
	if !ok {
		return ResolvedModel{}, fmt.Errorf("missing model %s", id)
	}
	resolver.visiting[id] = true
	defer delete(resolver.visiting, id)
	resolved := ResolvedModel{Textures: map[string]asset.TextureBinding{}}
	if model.Parent != "" {
		parent, err := resolver.Resolve(model.Parent)
		if err != nil {
			return ResolvedModel{}, err
		}
		resolved = parent
		resolved.Textures = maps.Clone(parent.Textures)
	}
	for key, value := range model.Textures {
		resolved.Textures[key] = value
	}
	if model.Elements != nil {
		resolved.Elements = *model.Elements
		resolved.Builtin = ""
	}
	resolver.cache[id] = resolved
	return resolved, nil
}

func (model ResolvedModel) Texture(ref asset.TextureRef) (asset.ID, error) {
	seen := map[string]bool{}
	for strings.HasPrefix(string(ref), "#") {
		key := strings.TrimPrefix(string(ref), "#")
		if seen[key] {
			return "", fmt.Errorf("cyclic texture variable %s", ref)
		}
		seen[key] = true
		value, ok := model.Textures[key]
		if !ok {
			return "", fmt.Errorf("unbound texture variable %s", ref)
		}
		ref = value.Sprite
	}
	return asset.ParseID(string(ref))
}
