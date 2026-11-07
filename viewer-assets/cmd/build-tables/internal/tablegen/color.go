package tablegen

import (
	"image"
	"math"

	"neomega-viewer-core/viewer/geometry"
)

type quadColorSample struct {
	// 相同贴图、采样范围与染色表示同一份颜色，回退采样时十字模型的重复面只贡献一次。
	Texture string
	Frames  int
	Region  [4]float32
	MinUV   geometry.Vec2
	MaxUV   geometry.Vec2
	Color   [3]float32
}

func averageTopColor(meshes []geometry.Mesh, img image.Image) (uint32, bool) {
	// 旋转后的朝上面决定俯视颜色；没有顶面时用去重贴图保留花草的颜色。
	var upward []geometry.Quad
	var all []geometry.Quad
	for _, mesh := range meshes {
		for _, quad := range mesh.Quads {
			all = append(all, quad)
			if quad.Normal[1] > 0.5 && projectedXZArea(quad) > 1e-6 {
				upward = append(upward, quad)
			}
		}
	}
	quads := upward
	if len(upward) == 0 {
		quads = all
	}
	seen := make(map[quadColorSample]bool)
	var sum [3]float64
	var weight float64
	for _, quad := range quads {
		sample := quadColorSample{
			Texture: quad.Surface.Texture,
			Frames:  quad.Surface.Frames,
			Region:  quad.Surface.Region,
			MinUV:   quad.UV[0],
			MaxUV:   quad.UV[0],
			Color:   quad.Color,
		}
		for _, uv := range quad.UV[1:] {
			for axis := range 2 {
				sample.MinUV[axis] = min(sample.MinUV[axis], uv[axis])
				sample.MaxUV[axis] = max(sample.MaxUV[axis], uv[axis])
			}
		}
		faceWeight := 1.0
		if len(upward) != 0 {
			faceWeight = projectedXZArea(quad)
		} else {
			if seen[sample] {
				continue
			}
			seen[sample] = true
		}
		color, ok := sample.average(img)
		if !ok {
			continue
		}
		for channel := range sum {
			sum[channel] += color[channel] * faceWeight
		}
		weight += faceWeight
	}
	if weight == 0 {
		return 0, false
	}
	var color uint32
	for _, value := range sum {
		color = color<<8 | uint32(math.Round(value/weight))
	}
	return color, true
}

func projectedXZArea(quad geometry.Quad) float64 {
	// 方块空间的投影面积让大小不同的顶面按实际覆盖比例贡献颜色。
	var twiceArea float64
	for i, position := range quad.Positions {
		next := quad.Positions[(i+1)%4]
		twiceArea += position[0]*next[2] - next[0]*position[2]
	}
	return math.Abs(twiceArea) / 2
}

func (sample quadColorSample) average(img image.Image) ([3]float64, bool) {
	// 静态预览只取动画首帧，并让所有可见像素等权，避免半透明像素被预乘压暗。
	if sample.MinUV[0] == sample.MaxUV[0] || sample.MinUV[1] == sample.MaxUV[1] {
		return [3]float64{}, false
	}
	bounds := img.Bounds()
	width, height := float64(bounds.Dx()), float64(bounds.Dy())
	u, v := float64(sample.Region[0]), float64(sample.Region[1])
	w := float64(sample.Region[2])
	h := float64(sample.Region[3]) / float64(max(sample.Frames, 1))
	rect := image.Rect(
		bounds.Min.X+int(math.Floor((u+sample.MinUV[0]*w)*width)),
		bounds.Min.Y+int(math.Floor((v+sample.MinUV[1]*h)*height)),
		bounds.Min.X+int(math.Ceil((u+sample.MaxUV[0]*w)*width)),
		bounds.Min.Y+int(math.Ceil((v+sample.MaxUV[1]*h)*height)),
	).Intersect(bounds)
	var tint [3]uint32
	tinted := sample.Color != [3]float32{1, 1, 1}
	if tinted {
		for channel, value := range sample.Color {
			tint[channel] = uint32(srgb8(value))
		}
	}
	nrgba, direct := img.(*image.NRGBA)
	var sum [3]uint64
	var count uint64
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			var pixel [3]uint8
			if direct {
				offset := nrgba.PixOffset(x, y)
				if nrgba.Pix[offset+3] == 0 {
					continue
				}
				pixel = [3]uint8{nrgba.Pix[offset], nrgba.Pix[offset+1], nrgba.Pix[offset+2]}
			} else {
				r, g, b, a := img.At(x, y).RGBA()
				if a == 0 {
					continue
				}
				pixel = [3]uint8{uint8(r * 255 / a), uint8(g * 255 / a), uint8(b * 255 / a)}
			}
			for channel, value := range pixel {
				tex := uint32(value)
				if tinted {
					tex = (tex * tint[channel]) >> 8
				}
				sum[channel] += uint64(tex)
			}
			count++
		}
	}
	if count == 0 {
		return [3]float64{}, false
	}
	var color [3]float64
	for channel, value := range sum {
		color[channel] = float64(value) / float64(count)
	}
	return color, true
}

func srgb8(linear float32) uint8 {
	// 烘焙染色以线性光保存，采样时还原到贴图使用的 sRGB 再相乘。
	v := float64(linear)
	if v <= 0.0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(max(0, min(255, math.Round(v*255))))
}
