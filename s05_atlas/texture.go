// SPDX-License-Identifier: MIT
package s05_atlas

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"

	"neomega-assets/asset"
)

func loadPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}

// FirstFrame follows the frame grid described by .png.mcmeta. A numeric frame
// or an object {index,time} may choose a frame other than the top-left one.
func FirstFrame(img image.Image, metadata asset.TextureMetadata) (image.Rectangle, error) {
	bounds := img.Bounds()
	if metadata.Animation == nil {
		return bounds, nil
	}
	animation := metadata.Animation
	width, height := animation.Width, animation.Height
	switch {
	case width == 0 && height == 0:
		width = min(bounds.Dx(), bounds.Dy())
		height = width
	case width == 0:
		width = bounds.Dx()
	case height == 0:
		height = bounds.Dy()
	}
	if width <= 0 || height <= 0 || bounds.Dx()%width != 0 || bounds.Dy()%height != 0 {
		return image.Rectangle{}, fmt.Errorf("invalid animation frame size %dx%d for %v", width, height, bounds)
	}
	index := 0
	if len(animation.Frames) > 0 {
		index = animation.Frames[0].Index
	}
	columns, rows := bounds.Dx()/width, bounds.Dy()/height
	if index < 0 || index >= columns*rows {
		return image.Rectangle{}, fmt.Errorf("animation frame %d is out of range", index)
	}
	origin := bounds.Min.Add(image.Pt(index%columns*width, index/columns*height))
	return image.Rectangle{Min: origin, Max: origin.Add(image.Pt(width, height))}, nil
}

func renderTexture(img image.Image, source image.Rectangle, width, height int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	if source.Dx() == width && source.Dy() == height {
		draw.Draw(result, result.Bounds(), img, source.Min, draw.Src)
		return result
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			result.Set(x, y, img.At(source.Min.X+x*source.Dx()/width, source.Min.Y+y*source.Dy()/height))
		}
	}
	return result
}
