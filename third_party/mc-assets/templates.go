// SPDX-License-Identifier: MIT
// Community models are copied from mc-assets under its MIT license. Original
// per-model credits are preserved in JSON. See LICENSE and the embedded index.
package mcassets

import (
	"embed"
	"encoding/json"
	"neomega-assets/asset"
)

//go:embed templates
var Files embed.FS

type Index struct {
	Repository string              `json:"repository"`
	Revision   string              `json:"revision"`
	States     map[asset.ID]string `json:"states"`
	Models     map[asset.ID]string `json:"models"`
	Textures   map[asset.ID]string `json:"textures"`
	Credits    []string            `json:"credits"`
}

func Load() (Index, error) {
	var index Index
	data, err := Files.ReadFile("templates/index.json")
	if err != nil {
		return index, err
	}
	err = json.Unmarshal(data, &index)
	return index, err
}
