package item_render

import (
	"fmt"
	"image"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
)

func generated(resolved *model.Resolved, atlas model.Atlas, pixels image.Image) ([]model.Element, error) {
	// 每层保留两张带透明裁剪的正反面，只沿不透明像素边界生成并合并侧壁。
	var elements []model.Element
	for layer := 0; layer < 5; layer++ {
		slot := fmt.Sprintf("layer%d", layer)
		if _, ok := resolved.Textures[slot]; !ok {
			break
		}
		ref := model.TextureRef("#" + slot)
		tint := layer
		id, _, err := resolved.Texture(ref)
		if err != nil {
			return nil, err
		}
		rect, ok := atlas.PixelRect(id)
		if !ok {
			return nil, fmt.Errorf("generated texture %s missing", id)
		}
		frames := atlas.Textures[id].Frames
		height := rect.Dy() / frames
		width := rect.Dx()
		opaque := func(x, y int) bool {
			if x < 0 || x >= width || y < 0 || y >= height {
				return false
			}
			for frame := 0; frame < frames; frame++ {
				_, _, _, a := pixels.At(rect.Min.X+x, rect.Min.Y+y+frame*height).RGBA()
				if a != 0 {
					return true
				}
			}
			return false
		}
		elements = append(elements, model.Element{From: model.Vec3{0, 0, 7.5}, To: model.Vec3{16, 16, 8.5}, Faces: map[model.Direction]model.Face{
			"north": {Texture: ref, TintIndex: &tint, UV: &model.UV{16, 0, 0, 16}}, "south": {Texture: ref, TintIndex: &tint, UV: &model.UV{0, 0, 16, 16}},
		}})
		// 横向扫描上下边，纵向扫描左右边；相连边共享一张面，UV 采样轮廓像素中心。
		for _, side := range []model.Direction{"up", "down", "west", "east"} {
			rows, cols := height, width
			if side == "west" || side == "east" {
				rows, cols = width, height
			}
			for row := 0; row < rows; row++ {
				edge := func(col int) bool {
					x, y := col, row
					dx, dy := 0, -1
					switch side {
					case "down":
						dy = 1
					case "west":
						x, y = row, col
						dx, dy = -1, 0
					case "east":
						x, y = row, col
						dx, dy = 1, 0
					}
					return opaque(x, y) && !opaque(x+dx, y+dy)
				}
				for col := 0; col < cols; {
					if !edge(col) {
						col++
						continue
					}
					start := col
					for col < cols && edge(col) {
						col++
					}
					var from, to model.Vec3
					var uv model.UV
					if side == "up" || side == "down" {
						y := 16 - float64(row)*16/float64(height)
						if side == "down" {
							y -= 16 / float64(height)
						}
						from = model.Vec3{float64(start) * 16 / float64(width), y, 7.5}
						to = model.Vec3{float64(col) * 16 / float64(width), y, 8.5}
						v := (float64(row) + .5) * 16 / float64(height)
						uv = model.UV{from[0], v, to[0], v}
					} else {
						x := float64(row) * 16 / float64(width)
						if side == "east" {
							x += 16 / float64(width)
						}
						from = model.Vec3{x, 16 - float64(col)*16/float64(height), 7.5}
						to = model.Vec3{x, 16 - float64(start)*16/float64(height), 8.5}
						u := (float64(row) + .5) * 16 / float64(width)
						uv = model.UV{u, float64(start) * 16 / float64(height), u, float64(col) * 16 / float64(height)}
					}
					elements = append(elements, model.Element{From: from, To: to, Faces: map[model.Direction]model.Face{side: {Texture: ref, TintIndex: &tint, UV: &uv}}})
				}
			}
		}
	}
	return elements, nil
}
