package java_block_model

import (
	"fmt"
	"maps"
	"strings"
	"sync"
)

type Resolved struct {
	Builtin  ResourceID
	Elements []Element
	Textures map[string]TextureBinding
}

// Resolver caches immutable inherited models. The supplied catalog must remain
// unchanged; returned definitions are read-only and can be shared concurrently.
type Resolver struct {
	catalog Models
	cache   sync.Map
}

func NewResolver(catalog Models) *Resolver { return &Resolver{catalog: catalog} }

func (resolver *Resolver) Resolve(id ResourceID) (*Resolved, error) {
	return resolver.resolve(id, map[ResourceID]bool{})
}

func (resolver *Resolver) resolve(id ResourceID, visiting map[ResourceID]bool) (*Resolved, error) {
	if cached, ok := resolver.cache.Load(id); ok {
		return cached.(*Resolved), nil
	}
	if visiting[id] {
		return nil, fmt.Errorf("model parent cycle at %s", id)
	}
	if strings.HasPrefix(id.Path(), "builtin/") {
		return &Resolved{Builtin: id, Textures: map[string]TextureBinding{}}, nil
	}
	definition, ok := resolver.catalog[id]
	if !ok {
		return nil, fmt.Errorf("model %s not found", id)
	}
	visiting[id] = true
	defer delete(visiting, id)
	result := &Resolved{Textures: map[string]TextureBinding{}}
	if definition.Parent != "" {
		parent, err := resolver.resolve(definition.Parent, visiting)
		if err != nil {
			return nil, err
		}
		result.Builtin = parent.Builtin
		result.Elements = parent.Elements
		result.Textures = maps.Clone(parent.Textures)
	}
	for name, binding := range definition.Textures {
		result.Textures[name] = binding
	}
	if definition.Elements != nil {
		result.Elements = *definition.Elements
		result.Builtin = ""
	}
	resolver.cache.Store(id, result)
	return result, nil
}

// Texture follows slot aliases and retains the material hint along that chain.
func (resolved *Resolved) Texture(reference TextureRef) (ResourceID, bool, error) {
	seen := map[string]bool{}
	translucent := false
	for strings.HasPrefix(string(reference), "#") {
		key := strings.TrimPrefix(string(reference), "#")
		if seen[key] {
			return "", false, fmt.Errorf("texture alias cycle at %s", key)
		}
		seen[key] = true
		binding, ok := resolved.Textures[key]
		if !ok {
			return "", false, fmt.Errorf("texture slot %s missing", key)
		}
		if binding.ForceTranslucent != nil {
			translucent = translucent || *binding.ForceTranslucent
		}
		reference = binding.Sprite
	}
	id, err := ParseResourceID(string(reference))
	return id, translucent, err
}
