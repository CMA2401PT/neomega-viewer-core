// Package banner 将 Java 旗帜的底色和 NBT 图案合成为私有贴图，复用几何表中的立式和挂墙模型。
package banner

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"strings"

	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/blockactor/internal/pngtextures"
	"neomega-viewer-core/viewer/cube"
	core "neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/geometry/material"
	"neomega-viewer-core/viewer/mesher"
	"neomega-viewer-core/viewer/splitlayer"
)

//go:embed textures/*.png
var files embed.FS

type Handler struct {
	models   *mesher.Mesher[uint32]
	textures map[string]*image.NRGBA
}

// New 加载随处理器交付的旗帜图片；构建方块时只合成像素，不再读取文件。
func New(tables *core.Tables[uint32]) (*Handler, error) {
	images, err := pngtextures.Load(files)
	if err != nil {
		return nil, err
	}
	textures := make(map[string]*image.NRGBA, len(images))
	for name, pixels := range images {
		textures["minecraft:"+name] = pixels
	}
	return &Handler{models: mesher.NewMesher(cube.New(1), tables), textures: textures}, nil
}

func (h *Handler) BlockNames() []string {
	names := make([]string, 0, len(dyes)*2)
	for _, dye := range dyes {
		names = append(names, "minecraft:"+dye.name+"_banner", "minecraft:"+dye.name+"_wall_banner")
	}
	return names
}

// Build 按 NBT 列表合成旗面，连同旗杆图片一起交付；形状、UV 和状态朝向复用几何表。
func (h *Handler) Build(input blockactor.Input) (blockactor.Appearance, error) {
	name := strings.TrimPrefix(input.State.NameWithNameSpace(), "minecraft:")
	baseColor := strings.TrimSuffix(strings.TrimSuffix(name, "_banner"), "_wall")
	layers, err := patterns(input.NBT, baseColor)
	if err != nil {
		return blockactor.Appearance{}, err
	}
	pixels, err := h.compose(layers)
	if err != nil {
		return blockactor.Appearance{}, err
	}
	mesh, err := h.models.MeshBlock(splitlayer.SplitBlock(input.State)[0])
	if err != nil {
		return blockactor.Appearance{}, err
	}
	// 私有名称只在当前 actor 内解析，因此两面不同旗帜可以使用相同名称。
	const textureName = "banner:face"
	for i := range mesh.Quads {
		q := &mesh.Quads[i]
		if strings.HasPrefix(q.Surface.Texture, "minecraft:block/banner/banner_") {
			q.Surface = material.Surface{Texture: textureName, Region: [4]float32{0, 0, 1, 1}, Mode: material.Cutout, Opacity: 1, Frames: 1}
			q.Color = [3]float32{1, 1, 1}
		} else {
			// 旗杆和横梁使用自己的原图，Region 不再引用公共图集。
			q.Surface.Texture = "banner:stand"
			q.Surface.Region = [4]float32{0, 0, 1, 1}
		}
	}
	stand := h.textures["minecraft:banner_base"]
	return blockactor.Appearance{
		Mesh: mesh,
		Textures: []blockactor.Texture{
			{Name: textureName, Width: uint32(pixels.Rect.Dx()), Height: uint32(pixels.Rect.Dy()), Pixels: pixels.Pix},
			{Name: "banner:stand", Width: uint32(stand.Rect.Dx()), Height: uint32(stand.Rect.Dy()), Pixels: stand.Pix},
		},
	}, nil
}

// compose 保留图案自带的布料明暗，以染料颜色调制 RGB，再按 alpha 从底到顶覆盖。
func (h *Handler) compose(layers []layer) (*image.NRGBA, error) {
	base, found := h.textures["minecraft:base"]
	if !found {
		return nil, fmt.Errorf("缺少旗帜底图 minecraft:base")
	}
	out := image.NewNRGBA(image.Rect(0, 0, base.Rect.Dx(), base.Rect.Dy()))
	for _, layer := range layers {
		src, found := h.textures[layer.pattern]
		if !found {
			return nil, fmt.Errorf("未知旗帜图案 %q", layer.pattern)
		}
		if src.Rect.Size() != base.Rect.Size() {
			return nil, fmt.Errorf("旗帜图案 %q 与底图尺寸不同", layer.pattern)
		}
		for y := 0; y < out.Rect.Dy(); y++ {
			for x := 0; x < out.Rect.Dx(); x++ {
				p := src.NRGBAAt(src.Rect.Min.X+x, src.Rect.Min.Y+y)
				d := out.NRGBAAt(x, y)
				sa, da := uint32(p.A), uint32(d.A)
				alpha := sa*255 + da*(255-sa)
				if alpha == 0 {
					continue
				}
				tint := layer.color
				blend := func(s, t, old uint8) uint8 {
					value := uint32(s) * uint32(t) / 255
					return uint8((value*sa*255 + uint32(old)*da*(255-sa) + alpha/2) / alpha)
				}
				out.SetNRGBA(x, y, color.NRGBA{blend(p.R, tint.R, d.R), blend(p.G, tint.G, d.G), blend(p.B, tint.B, d.B), uint8((alpha + 127) / 255)})
			}
		}
	}
	return out, nil
}
