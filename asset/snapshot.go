// SPDX-License-Identifier: MIT
package asset

import "time"

// File.Path is relative to the directory containing snapshot.json.
type File struct {
	Path   string `json:"path"`
	Origin string `json:"origin"`
}

// Snapshot is the explicit boundary between extraction, patching and table
// generation. Each category uses resource IDs without extensions as keys.
type Snapshot struct {
	Schema          int         `json:"schema"`
	Version         string      `json:"version"`
	BlockStates     map[ID]File `json:"blockstates"`
	Models          map[ID]File `json:"models"`
	Textures        map[ID]File `json:"textures"`
	TextureMetadata map[ID]File `json:"textureMetadata"`
	Sources         []Source    `json:"sources"`
}

type Source struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
	License  string `json:"license,omitempty"`
}

func NewSnapshot(version string) *Snapshot {
	return &Snapshot{Schema: 1, Version: version, BlockStates: map[ID]File{}, Models: map[ID]File{}, Textures: map[ID]File{}, TextureMetadata: map[ID]File{}}
}

type Download struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

type VersionInfo struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	ReleaseTime time.Time `json:"releaseTime"`
	Downloads   struct {
		Client Download `json:"client"`
	} `json:"downloads"`
}
