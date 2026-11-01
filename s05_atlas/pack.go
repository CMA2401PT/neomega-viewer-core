// SPDX-License-Identifier: MIT
package s05_atlas

import (
	"fmt"
	"image"
	"slices"
)

type rectangle struct {
	name          string
	width, height int
	position      image.Point
}

func nextPowerOfTwo(n int) int {
	size := 1
	for size < n {
		size *= 2
	}
	return size
}

// Textures are padded to tiles before packing. Place large rectangles first,
// then reuse the holes they leave. A tile grid avoids losing the lower half of
// a row when entity textures have different heights.
func pack(input []rectangle, tileSize int) ([]rectangle, image.Point, error) {
	rectangles := slices.Clone(input)
	slices.SortFunc(rectangles, func(a, b rectangle) int {
		if a.width*a.height != b.width*b.height {
			return b.width*b.height - a.width*a.height
		}
		if a.height != b.height {
			return b.height - a.height
		}
		if a.width != b.width {
			return b.width - a.width
		}
		if a.name < b.name {
			return -1
		}
		if a.name > b.name {
			return 1
		}
		return 0
	})

	area, minWidth, minHeight := 0, tileSize, tileSize
	for _, rect := range rectangles {
		area += rect.width * rect.height
		minWidth = max(minWidth, rect.width)
		minHeight = max(minHeight, rect.height)
	}
	// Try the smallest area first; at equal area prefer balanced, then wide layouts.
	var candidates []image.Point
	for width := nextPowerOfTwo(minWidth); width <= 16384; width *= 2 {
		for height := nextPowerOfTwo(minHeight); height <= 16384; height *= 2 {
			if width*height >= area {
				candidates = append(candidates, image.Pt(width, height))
			}
		}
	}
	slices.SortFunc(candidates, func(a, b image.Point) int {
		if a.X*a.Y != b.X*b.Y {
			return a.X*a.Y - b.X*b.Y
		}
		if max(a.X, a.Y) != max(b.X, b.Y) {
			return max(a.X, a.Y) - max(b.X, b.Y)
		}
		return b.X - a.X
	})
	for _, size := range candidates {
		if place(rectangles, tileSize, size) {
			return rectangles, size, nil
		}
	}
	return nil, image.Point{}, fmt.Errorf("textures do not fit a 16384x16384 atlas")
}

func place(rectangles []rectangle, tileSize int, size image.Point) bool {
	columns, rows := size.X/tileSize, size.Y/tileSize
	occupied := make([]bool, columns*rows)
	for i := range rectangles {
		rect := &rectangles[i]
		width, height := rect.width/tileSize, rect.height/tileSize
		placed := false
		for y := 0; y <= rows-height && !placed; y++ {
			for x := 0; x <= columns-width; x++ {
				free := true
				for dy := 0; dy < height && free; dy++ {
					for dx := 0; dx < width; dx++ {
						if occupied[(y+dy)*columns+x+dx] {
							free = false
							break
						}
					}
				}
				if !free {
					continue
				}
				for dy := 0; dy < height; dy++ {
					for dx := 0; dx < width; dx++ {
						occupied[(y+dy)*columns+x+dx] = true
					}
				}
				rect.position = image.Pt(x*tileSize, y*tileSize)
				placed = true
				break
			}
		}
		if !placed {
			return false
		}
	}
	return true
}
