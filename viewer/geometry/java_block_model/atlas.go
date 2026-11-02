package java_block_model

import (
	"image"
	"math"
)

type AtlasTexture struct {
	U         float64 `json:"u"`
	V         float64 `json:"v"`
	SU        float64 `json:"su"`
	SV        float64 `json:"sv"`
	TileIndex int     `json:"tileIndex"`
	Frames    int     `json:"frames"`
	FrameTime int     `json:"frameTime"`
}

type Atlas struct {
	Textures map[ResourceID]AtlasTexture `json:"textures"`
	Width    int                         `json:"width"`
	Height   int                         `json:"height"`
	TileSize int                         `json:"tileSize"`
	// SUSV is read only for legacy square atlases. New outputs use per-texture SU/SV.
	SUSV float64 `json:"suSv,omitempty"`
}

// PixelRect converts a texture's normalized atlas region into pixels. Animated
// textures cover the whole frame strip, not just the first frame.
func (atlas Atlas) PixelRect(id ResourceID) (image.Rectangle, bool) {
	texture, ok := atlas.Textures[id]
	if !ok {
		return image.Rectangle{}, false
	}
	x := int(math.Round(texture.U * float64(atlas.Width)))
	y := int(math.Round(texture.V * float64(atlas.Height)))
	width := int(math.Round(texture.SU * float64(atlas.Width)))
	height := int(math.Round(texture.SV * float64(atlas.Height)))
	return image.Rect(x, y, x+width, y+height), true
}
