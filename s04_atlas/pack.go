// SPDX-License-Identifier: MIT
package s04_atlas

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

// Deterministic shelf packing keeps placement easy to inspect. Larger images
// are placed first; ordinary block textures then fill rows of the same height.
func pack(input []rectangle, tileSize int) ([]rectangle, int, error) {
	rectangles := slices.Clone(input)
	slices.SortFunc(rectangles, func(a, b rectangle) int {
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
		x, y, rowHeight := 0, 0, 0
		fits := true
		for i := range rectangles {
			rect := &rectangles[i]
			if x+rect.width > size {
				x = 0
				y += rowHeight
				rowHeight = 0
			}
			if y+rect.height > size {
				fits = false
				break
			}
			rect.position = image.Pt(x, y)
			x += rect.width
			rowHeight = max(rowHeight, rect.height)
		}
		if fits {
			return rectangles, size, nil
		}
	}
	return nil, 0, fmt.Errorf("textures do not fit a 16384×16384 atlas")
}
