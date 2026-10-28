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
func pack(input []rectangle, tileSize int) ([]rectangle, int, error) {
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
	area, minimum := 0, tileSize
	for _, rect := range rectangles {
		area += rect.width * rect.height
		minimum = max(minimum, rect.width, rect.height)
	}
	size := nextPowerOfTwo(minimum)
	for size*size < area {
		size *= 2
	}
	for ; size <= 16384; size *= 2 {
		side := size / tileSize
		occupied := make([]bool, side*side)
		fits := true
		for i := range rectangles {
			rect := &rectangles[i]
			width, height := rect.width/tileSize, rect.height/tileSize
			placed := false
			for y := 0; y <= side-height && !placed; y++ {
				for x := 0; x <= side-width; x++ {
					free := true
					for dy := 0; dy < height && free; dy++ {
						for dx := 0; dx < width; dx++ {
							if occupied[(y+dy)*side+x+dx] {
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
							occupied[(y+dy)*side+x+dx] = true
						}
					}
					rect.position = image.Pt(x*tileSize, y*tileSize)
					placed = true
					break
				}
			}
			if !placed {
				fits = false
				break
			}
		}
		if fits {
			return rectangles, size, nil
		}
	}
	return nil, 0, fmt.Errorf("textures do not fit a 16384×16384 atlas")
}
