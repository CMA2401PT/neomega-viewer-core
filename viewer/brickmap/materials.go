package brickmap

import (
	"image"
	"image/color"
	"image/png"
	"math"

	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
	"neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/geometry/material"
	resources "neomega-viewer-core/viewer/resources"
	"neomega-viewer-core/viewer/topview"
)

// MaterialSource 从既有几何资源与图集提取六面颜色和透明参数，渲染路径不按方块名选择透明算法。
type MaterialSource struct {
	tables *geometry.Tables[uint32]
	atlas  image.Image
	colors topview.Table
}

func OpenMaterialSource() (*MaterialSource, error) {
	// 加载配套的内嵌几何、图集与颜色表，让多个材质编号表共用同一份资源。
	preset, err := resources.Open(resources.OpenOptions{})
	if err != nil {
		return nil, err
	}
	file, err := resources.Resources().Open("atlas.png")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	atlas, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	colors, err := topview.Bundled()
	if err != nil {
		return nil, err
	}
	return &MaterialSource{tables: preset.Tables, atlas: atlas, colors: colors}, nil
}

// Material 将已有模型压成完整立方体；镂空贴图按实心颜色处理，仅 Blend 材质保留整体透明度。
func (s *MaterialSource) Material(block ir.IRBlock) (Material, bool) {
	var result Material
	rgb, known := s.colors.Color(block)
	if !known {
		rgb = 0x808080
	}
	for i := range 6 {
		result.Faces[i] = [4]byte{byte(rgb >> 16), byte(rgb >> 8), byte(rgb), 255}
	}
	target, ok := java.NormalizeSingleLayer(block)
	if !ok {
		return result, known
	}
	ids, exists := s.tables.Index[target.Hash()]
	if !exists {
		return result, known
	}
	var sums [6][4]float64
	var weights [6]float64
	var opaque [6]bool
	for _, id := range ids {
		shape := s.tables.Shapes[id]
		if shape.CullGroup != 0 {
			result.Group = shape.CullGroup
		}
		for _, q := range shape.Quads {
			axis := 0
			for a := 1; a < 3; a++ {
				if math.Abs(q.Normal[a]) > math.Abs(q.Normal[axis]) {
					axis = a
				}
			}
			side := axis * 2
			if q.Normal[axis] > 0 {
				side++
			}
			// 投影面积是完整方块某一面的颜色权重，旋转后的模型自然使用世界面方向。
			a, b := (axis+1)%3, (axis+2)%3
			area := 0.0
			for i, p := range q.Positions {
				next := q.Positions[(i+1)%4]
				area += p[a]*next[b] - next[a]*p[b]
			}
			area = math.Abs(area) / 2
			if area < 1e-8 {
				continue
			}
			sample, ok := s.sample(q)
			if !ok {
				continue
			}
			for c := range 4 {
				sums[side][c] += sample[c] * area
			}
			weights[side] += area
			if q.Surface.Mode != material.Blend {
				opaque[side] = true
			}
		}
	}
	for side := range 6 {
		if weights[side] == 0 {
			continue
		}
		for c := range 4 {
			result.Faces[side][c] = byte(math.Round(sums[side][c] / weights[side]))
		}
		if opaque[side] {
			result.Faces[side][3] = 255
		}
	}
	return result, known
}

// sample 对首帧贴图求代表色；透明表面的 alpha 包含透明像素，RGB 则按 alpha 加权避免透明底色污染。
func (s *MaterialSource) sample(q geometry.Quad) ([4]float64, bool) {
	lo, hi := q.UV[0], q.UV[0]
	for _, uv := range q.UV {
		for a := range 2 {
			lo[a] = min(lo[a], uv[a])
			hi[a] = max(hi[a], uv[a])
		}
	}
	bounds := s.atlas.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	region := q.Surface.Region
	frameH := float64(region[3]) / float64(max(1, q.Surface.Frames))
	rect := image.Rect(int(math.Floor((float64(region[0])+lo[0]*float64(region[2]))*w)), int(math.Floor((float64(region[1])+lo[1]*frameH)*h)), int(math.Ceil((float64(region[0])+hi[0]*float64(region[2]))*w)), int(math.Ceil((float64(region[1])+hi[1]*frameH)*h))).Intersect(bounds)
	var sum [3]float64
	alpha := 0.0
	count := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			pixel := color.NRGBAModel.Convert(s.atlas.At(x, y)).(color.NRGBA)
			alpha += float64(pixel.A)
			count++
			for c, value := range [3]byte{pixel.R, pixel.G, pixel.B} {
				sum[c] += float64(value) * float64(pixel.A)
			}
		}
	}
	if count == 0 || alpha == 0 {
		return [4]float64{}, false
	}
	var result [4]float64
	for c := range 3 {
		tint := float64(q.Color[c])
		if tint <= 0.0031308 {
			tint *= 12.92
		} else {
			tint = 1.055*math.Pow(tint, 1/2.4) - 0.055
		}
		result[c] = max(0, min(255, sum[c]/alpha*tint))
	}
	result[3] = 255
	if q.Surface.Mode == material.Blend {
		result[3] = max(0, min(255, alpha/float64(count)*float64(q.Surface.Opacity)))
	}
	return result, true
}
