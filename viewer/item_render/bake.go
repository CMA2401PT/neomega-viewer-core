package item_render

import (
	"errors"
	"fmt"
	"image"
	"math"

	"neomega-viewer-core/viewer/geometry"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
	"neomega-viewer-core/viewer/geometry/material"
)

// ErrUnsupportedModel 表示资源需要静态物品之外的渲染能力。
var ErrUnsupportedModel = errors.New("unsupported static item model")

// Bake 在资源生成时编译模型；elements 复用方块模型烘焙，generated 按 alpha 轮廓挤出。
func Bake(resolved *model.Resolved, atlas model.Atlas, pixels image.Image, tints ...uint32) (geometry.Mesh, error) {
	if resolved.Builtin != "" && resolved.Builtin != "minecraft:builtin/generated" {
		return geometry.Mesh{}, fmt.Errorf("%w: builtin %s", ErrUnsupportedModel, resolved.Builtin)
	}
	bound := atlasMaterials{atlas: atlas, pixels: pixels, tints: tints}
	definition := *resolved
	if resolved.Builtin == "minecraft:builtin/generated" {
		elements, err := generated(resolved, atlas, pixels)
		if err != nil {
			return geometry.Mesh{}, err
		}
		definition.Elements, definition.Builtin = elements, ""
	}
	mesh, err := model.Bake(&definition, model.Apply{}, bound)
	if err != nil {
		return geometry.Mesh{}, err
	}
	fixed := resolved.Display["fixed"]
	scale := model.Vec3{1, 1, 1}
	if fixed.Scale != nil {
		scale = *fixed.Scale
	}
	rotation, translation := model.Vec3{}, model.Vec3{}
	if fixed.Rotation != nil {
		rotation = *fixed.Rotation
	}
	if fixed.Translation != nil {
		translation = *fixed.Translation
	}
	rotate := func(v geometry.Vec3) geometry.Vec3 {
		// Java rotationXYZ 的矩阵先作用 Z，再 Y，最后 X。
		for axis := 2; axis >= 0; axis-- {
			a := rotation[axis] * math.Pi / 180
			c, s := math.Cos(a), math.Sin(a)
			i, j := (axis+1)%3, (axis+2)%3
			v[i], v[j] = c*v[i]-s*v[j], s*v[i]+c*v[j]
		}
		return v
	}
	normal := func(v geometry.Vec3) geometry.Vec3 {
		for axis := range v {
			v[axis] /= scale[axis]
		}
		v = rotate(v)
		length := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
		for axis := range v {
			v[axis] /= length
		}
		return v
	}
	for i := range mesh.Quads {
		q := &mesh.Quads[i]
		q.CullFace = geometry.NoCullFace
		for j, p := range q.Positions {
			for axis := range p {
				p[axis] = (p[axis] - .5) * scale[axis]
			}
			p = rotate(p)
			for axis := range p {
				p[axis] += math.Max(-80, math.Min(80, translation[axis])) / 16
			}
			q.Positions[j] = p
		}
		q.Normal, q.ShadeNormal = normal(q.Normal), normal(q.ShadeNormal)
	}
	return mesh, nil
}

type atlasMaterials struct {
	atlas  model.Atlas
	pixels image.Image
	tints  []uint32
}

func (m atlasMaterials) Surface(id model.ResourceID, forceBlend, unlit bool) (material.Surface, error) {
	// 物品直接使用纹理 alpha，不借用任何虚构方块状态或方块 tint 策略。
	entry, ok := m.atlas.Textures[id]
	if !ok {
		return material.Surface{}, fmt.Errorf("atlas texture %s missing", id)
	}
	rect, _ := m.atlas.PixelRect(id)
	mode := material.Opaque
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			_, _, _, a := m.pixels.At(x, y).RGBA()
			if a > 0 && a < 65535 {
				mode = material.Blend
			} else if a == 0 && mode == material.Opaque {
				mode = material.Cutout
			}
		}
	}
	if forceBlend {
		mode = material.Blend
	}
	return material.Surface{Texture: string(id), Region: [4]float32{float32(entry.U), float32(entry.V), float32(entry.SU), float32(entry.SV)}, Mode: mode, Opacity: 1, Unlit: unlit, Frames: entry.Frames, FrameTime: entry.FrameTime}, nil
}
func (m atlasMaterials) Tint(index int) [3]float32 {
	// 缺少 tint 源的面保持原色；有常量 tint 时按客户端 RGB 逐面着色。
	if index >= 0 && index < len(m.tints) {
		return model.Linear(m.tints[index])
	}
	return [3]float32{1, 1, 1}
}
