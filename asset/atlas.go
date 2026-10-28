// SPDX-License-Identifier: MIT
package asset

import (
	"bytes"
	"encoding/json"
)

type Frame struct {
	Index int
	Time  int
}

func (frame *Frame) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		var value struct {
			Index int `json:"index"`
			Time  int `json:"time"`
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*frame = Frame{Index: value.Index, Time: value.Time}
		return nil
	}
	return json.Unmarshal(data, &frame.Index)
}

type Animation struct {
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FrameTime   int     `json:"frametime"`
	Frames      []Frame `json:"frames"`
	Interpolate bool    `json:"interpolate"`
}
type TextureMetadata struct {
	Animation *Animation `json:"animation"`
}

type AtlasTexture struct {
	U         float64 `json:"u"`
	V         float64 `json:"v"`
	SU        float64 `json:"su"`
	SV        float64 `json:"sv"`
	TileIndex int     `json:"tileIndex"`
}

type Atlas struct {
	Textures map[string]AtlasTexture `json:"textures"`
	Width    int                     `json:"width"`
	Height   int                     `json:"height"`
	TileSize int                     `json:"tileSize"`
	SUSV     float64                 `json:"suSv"`
}

type BuildInfo struct {
	GeometrylessBlockStates []ID                 `json:"geometrylessBlockStates"`
	RendererCounts          map[RendererKind]int `json:"rendererCounts"`
	Version                 string               `json:"version"`
	Sources                 []Source             `json:"sources"`
	BlockStateCount         int                  `json:"blockStateCount"`
	ModelCount              int                  `json:"modelCount"`
	TextureCount            int                  `json:"textureCount"`
	BuiltinModels           []ID                 `json:"builtinModels"`
}
