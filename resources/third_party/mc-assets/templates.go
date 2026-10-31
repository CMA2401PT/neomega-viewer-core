// SPDX-License-Identifier: MIT
// Community models are copied from mc-assets under its MIT license. Original
// per-model credits are preserved in JSON. See LICENSE and the embedded index.
package mcassets

import (
	"embed"
	"encoding/json"

	"neomega-assets/asset"
	jmodel "neomega-blocks/geometry/java_block_model"
)

//go:embed templates
var Files embed.FS

type Index struct {
	Repository string                       `json:"repository"`
	Revision   string                       `json:"revision"`
	States     map[jmodel.ResourceID]string `json:"states"`
	Models     map[jmodel.ResourceID]string `json:"models"`
	Textures   map[jmodel.ResourceID]string `json:"textures"`
	Origins    map[string]string            `json:"origins"`
	Credits    []string                     `json:"credits"`
}

func Load() (Index, error) {
	var index Index
	data, err := Read("index.json")
	if err != nil {
		return index, err
	}
	err = json.Unmarshal(data, &index)
	return index, err
}

// Read returns an embedded template by the path stored in the index.
func Read(path string) ([]byte, error) {
	return Files.ReadFile("templates/" + path)
}

// StateOrigin maps an embedded state back to its original upstream path.
func (index Index) StateOrigin(id jmodel.ResourceID) string {
	return index.origin(index.States[id])
}

// ModelOrigin maps an embedded model back to its original upstream path.
func (index Index) ModelOrigin(path string) string {
	return index.origin(path)
}

// TextureOrigin maps an embedded texture back to its original upstream path.
func (index Index) TextureOrigin(path string) string {
	return index.origin(path)
}

// Source records both the upstream code license and the original asset rights.
func (index Index) Source() asset.Source {
	return asset.Source{
		Kind:     "community-static-previews",
		Location: index.Repository + "/tree/" + index.Revision + "/custom",
		License:  "MIT; original model credits retained; Minecraft-derived material retains original rights",
	}
}

func (index Index) origin(path string) string {
	return index.Repository + "@" + index.Revision + "/" + index.Origins[path]
}
