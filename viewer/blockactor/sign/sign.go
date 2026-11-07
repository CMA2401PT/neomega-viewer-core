// Package sign 将 Java 告示牌的双面文本烘焙为图片，浏览器只接收牌体和文字面。
package sign

import (
	"embed"
	"fmt"
	"math"
	"strconv"
	"strings"

	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/blockactor/internal/pngtextures"
	"neomega-viewer-core/viewer/cube"
	core "neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/geometry/material"
	"neomega-viewer-core/viewer/mesher"
	"neomega-viewer-core/viewer/splitlayer"
)

// 固定素材随处理器交付，不需要资源生成器或额外 HTTP 路由。
//
//go:embed textures/*.png font/*
var files embed.FS

type Handler struct {
	models   *mesher.Mesher[uint32]
	textures map[string]blockactor.Texture
	font     *font
}

// New 加载牌体与字形图片；返回的处理器只读共享资源，每次 Build 独立生成文字图片。
func New(tables *core.Tables[uint32]) (*Handler, error) {
	images, err := pngtextures.Load(files)
	if err != nil {
		return nil, err
	}
	textures := make(map[string]blockactor.Texture, len(images))
	for name, img := range images {
		id := "minecraft:block/" + name
		textures[id] = blockactor.Texture{Name: id, Width: uint32(img.Rect.Dx()), Height: uint32(img.Rect.Dy()), Pixels: img.Pix}
	}
	font, err := loadFont()
	if err != nil {
		return nil, err
	}
	return &Handler{models: mesher.NewMesher(cube.New(1), tables), textures: textures, font: font}, nil
}

func (h *Handler) BlockNames() []string {
	// 从目标状态收集牌体名称，使处理器与几何表使用相同的木材和形态。
	var names []string
	seen := make(map[string]bool)
	for state := range java.JavaFullStates() {
		name := state.NameWithNameSpace()
		if strings.HasSuffix(name, "_sign") && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names
}

// Build 保留牌体的状态旋转，为正反面分别添加透明文字面；文字略离开木板，避免共面闪烁。
func (h *Handler) Build(input blockactor.Input) (blockactor.Appearance, error) {
	sides, err := signSides(input.NBT)
	if err != nil {
		return blockactor.Appearance{}, err
	}
	mesh, err := h.models.MeshBlock(splitlayer.SplitBlock(input.State)[0])
	if err != nil {
		return blockactor.Appearance{}, err
	}
	front, err := frontNormal(input.State)
	if err != nil {
		return blockactor.Appearance{}, err
	}
	hanging := strings.HasSuffix(input.State.NameWithNameSpace(), "_hanging_sign")
	result := blockactor.Appearance{Mesh: mesh}
	used := make(map[string]bool)
	for i := range result.Mesh.Quads {
		surface := &result.Mesh.Quads[i].Surface
		texture, found := h.textures[surface.Texture]
		if !found {
			return blockactor.Appearance{}, fmt.Errorf("未知告示牌贴图 %q", surface.Texture)
		}
		surface.Region = [4]float32{0, 0, 1, 1}
		if !used[texture.Name] {
			used[texture.Name] = true
			result.Textures = append(result.Textures, texture)
		}
	}
	for sideIndex, side := range sides {
		pixels, visible := h.font.rasterize(side, hanging)
		if !visible {
			continue
		}
		normal := front
		if sideIndex == 1 {
			for i := range normal {
				normal[i] = -normal[i]
			}
		}
		board, err := boardFace(mesh.Quads, normal)
		if err != nil {
			return blockactor.Appearance{}, err
		}
		name := []string{"sign:front", "sign:back"}[sideIndex]
		board.Surface = material.Surface{Texture: name, Region: [4]float32{0, 0, 1, 1}, Mode: material.Cutout, Unlit: side.glow, Opacity: 1, Frames: 1}
		board.Color = [3]float32{1, 1, 1}
		board.ShadeNormal = core.Vec3{}
		board.CullFace = core.NoCullFace
		// 从观看这一面的方向定义左右，背面的文字不会镜像；不依赖木纹的 UV 布局。
		right := core.Vec3{board.Normal[2], 0, -board.Normal[0]}
		lo, hi, top, bottom := math.Inf(1), math.Inf(-1), math.Inf(-1), math.Inf(1)
		for _, p := range board.Positions {
			u := p[0]*right[0] + p[2]*right[2]
			lo, hi, top, bottom = min(lo, u), max(hi, u), max(top, p[1]), min(bottom, p[1])
		}
		for i, p := range board.Positions {
			u := p[0]*right[0] + p[2]*right[2]
			board.UV[i] = core.Vec2{(u - lo) / (hi - lo), (top - p[1]) / (top - bottom)}
			if hanging {
				// 悬挂牌每字体单位为 0.9/64 格，文字中心比木板中心低 0.0075 格。
				delta := (board.UV[i][0]-0.5)*0.9 - (u - (lo+hi)/2)
				board.Positions[i][0] += right[0] * delta
				board.Positions[i][2] += right[2] * delta
				board.Positions[i][1] = (top+bottom)/2 - 0.0075 + (0.5-board.UV[i][1])*0.675
			}
			for axis := range p {
				board.Positions[i][axis] += board.Normal[axis] / 1024
			}
		}
		result.Mesh.Quads = append(result.Mesh.Quads, board)
		result.Textures = append(result.Textures, blockactor.Texture{Name: name, Width: uint32(pixels.Rect.Dx()), Height: uint32(pixels.Rect.Dy()), Pixels: pixels.Pix})
	}
	return result, nil
}

// frontNormal 使用 Java 方块的观看方向；悬挂牌模板的本地朝向不同，不能用模型纹理名猜正面。
func frontNormal(state ir.IRBlock) (core.Vec3, error) {
	states := state.States()
	if value, exists := states.Get("facing"); exists {
		switch value {
		case "north":
			return core.Vec3{0, 0, -1}, nil
		case "south":
			return core.Vec3{0, 0, 1}, nil
		case "east":
			return core.Vec3{1, 0, 0}, nil
		case "west":
			return core.Vec3{-1, 0, 0}, nil
		}
		return core.Vec3{}, fmt.Errorf("未知告示牌朝向 %v", value)
	}
	rotation, _ := states.Get("rotation")
	value, err := strconv.Atoi(fmt.Sprint(rotation))
	if err != nil {
		return core.Vec3{}, err
	}
	angle := float64(value) * math.Pi / 8
	return core.Vec3{-math.Sin(angle), 0, math.Cos(angle)}, nil
}

// boardFace 取朝向指定方向的最大竖直面，排除牌柱、边框和链条，直接复用已旋转的牌面。
func boardFace(quads []core.Quad, normal core.Vec3) (core.Quad, error) {
	var chosen core.Quad
	area := 0.0
	for _, quad := range quads {
		dot := quad.Normal[0]*normal[0] + quad.Normal[2]*normal[2]
		if dot < 0.999 {
			continue
		}
		var a, b core.Vec3
		for axis := range a {
			a[axis] = quad.Positions[1][axis] - quad.Positions[0][axis]
			b[axis] = quad.Positions[3][axis] - quad.Positions[0][axis]
		}
		cross := core.Vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
		size := math.Sqrt(cross[0]*cross[0] + cross[1]*cross[1] + cross[2]*cross[2])
		if size > area {
			area, chosen = size, quad
		}
	}
	if area == 0 {
		return chosen, fmt.Errorf("告示牌模型缺少朝向 %v 的文字面", normal)
	}
	return chosen, nil
}
