// Package pngtextures 为方块处理器读取其内嵌的原尺寸 PNG。
package pngtextures

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"strings"
)

// Load 以不含扩展名的文件名索引图片；返回从左上角逐行排列的非预乘 RGBA 像素。
func Load(files fs.FS) (map[string]*image.NRGBA, error) {
	entries, err := fs.ReadDir(files, "textures")
	if err != nil {
		return nil, err
	}
	images := make(map[string]*image.NRGBA, len(entries))
	for _, entry := range entries {
		path := "textures/" + entry.Name()
		file, err := files.Open(path)
		if err != nil {
			return nil, err
		}
		decoded, err := png.Decode(file)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		pixels := image.NewNRGBA(image.Rect(0, 0, decoded.Bounds().Dx(), decoded.Bounds().Dy()))
		draw.Draw(pixels, pixels.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
		images[strings.TrimSuffix(entry.Name(), ".png")] = pixels
	}
	return images, nil
}
