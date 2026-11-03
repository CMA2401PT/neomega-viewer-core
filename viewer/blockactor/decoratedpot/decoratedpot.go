package decoratedpot

import (
	"embed"
	"fmt"
	"math"
	"strings"

	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/blockactor/internal/pngtextures"
	"neomega-viewer-core/viewer/cube"
	blockgeometry "neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/geometry/material"
	"neomega-viewer-core/viewer/mesher"
	"neomega-viewer-core/viewer/mesher/geometry"
	"neomega-viewer-core/viewer/splitlayer"
)

//go:embed textures/*.png
var files embed.FS

type Handler struct {
	models   *mesher.Mesher[uint32]
	textures map[string]blockactor.Texture
}

// New 加载随处理器交付的陶罐图片；同一张只读图片可由多个方块共享。
func New(tables *blockgeometry.Tables[uint32]) (*Handler, error) {
	images, err := pngtextures.Load(files)
	if err != nil {
		return nil, err
	}
	textures := make(map[string]blockactor.Texture, len(images))
	for name, image := range images {
		name = "minecraft:entity/decorated_pot/" + name
		textures[name] = blockactor.Texture{Name: name, Width: uint32(image.Rect.Dx()), Height: uint32(image.Rect.Dy()), Pixels: image.Pix}
	}
	return &Handler{models: mesher.NewMesher(cube.New(1), tables), textures: textures}, nil
}

func (h *Handler) BlockNames() []string {
	return []string{"minecraft:decorated_pot"}
}

func decorations(root nbt.Compound) [4]string {
	// 目标版本把 sherds 存成按面命名的 compound；1.20 起的早期存档是四项列表，一并接受，格式见 NBT.md。
	var items [4]string
	switch sherds := root["sherds"].(type) {
	case []string:
		copy(items[:], sherds)
	case []any:
		for i := range min(len(sherds), len(items)) {
			items[i], _ = sherds[i].(string)
		}
	case map[string]any:
		for i, side := range []string{"back", "left", "right", "front"} {
			item, _ := sherds[side].(map[string]any)
			items[i], _ = item["id"].(string)
		}
	}
	return items
}

func texture(item string) string {
	name := "decorated_pot_side"
	if sherd, found := strings.CutPrefix(item, "minecraft:"); found && strings.HasSuffix(sherd, "_pottery_sherd") {
		name = strings.TrimSuffix(sherd, "_pottery_sherd") + "_pottery_pattern"
	}
	return "minecraft:entity/decorated_pot/" + name
}

func (h *Handler) Build(input blockactor.Input) (blockactor.Appearance, error) {
	// 表已经按 facing 旋转，复用它可保留 Neck、Head 与顶底面的精确形状和 UV。
	mesh, err := h.models.MeshBlock(splitlayer.SplitBlock(input.State)[0])
	if err != nil {
		return blockactor.Appearance{}, err
	}
	items := decorations(input.NBT)
	facing, _ := input.State.States().Get("facing")
	front := map[string]geometry.Vec3{
		"north": {0, 0, 1}, "east": {-1, 0, 0},
		"south": {0, 0, -1}, "west": {1, 0, 0},
	}[facing.(string)]
	// Java 的原始 front 在 +Z，绕 Y 转 180°-toYRot；north 时左右分别在 -X、+X。
	normals := [4]geometry.Vec3{
		{-front[0], 0, -front[2]},
		{-front[2], 0, front[0]},
		{front[2], 0, -front[0]},
		front,
	}
	for i := range mesh.Quads {
		quad := &mesh.Quads[i]
		// Body 的侧面从 y=0 到 y=1；上方的 Neck 与 Head 不参与纹样选择。
		lo, hi := quad.Positions[0][1], quad.Positions[0][1]
		for _, p := range quad.Positions {
			lo, hi = min(lo, p[1]), max(hi, p[1])
		}
		if math.Abs(lo) > 1e-9 || math.Abs(hi-1) > 1e-9 {
			continue
		}
		for side, normal := range normals {
			if dot(quad.Normal, normal) < .999 {
				continue
			}
			quad.Surface.Texture = texture(items[side])
			// 原版侧面 texOffs(1,0)，在 16×16 纹理中取中央 14×16，保持一像素一模型单位。
			quad.UV = [4]geometry.Vec2{{1.0 / 16, 0}, {1.0 / 16, 1}, {15.0 / 16, 1}, {15.0 / 16, 0}}
		}
	}
	// 只交付当前陶罐实际用到的图片；相同纹样在四面出现时也只传一次。
	appearance := blockactor.Appearance{Mesh: mesh}
	used := make(map[string]bool)
	for i := range appearance.Mesh.Quads {
		surface := &appearance.Mesh.Quads[i].Surface
		image, found := h.textures[surface.Texture]
		if !found {
			return blockactor.Appearance{}, fmt.Errorf("未知陶罐贴图 %q", surface.Texture)
		}
		*surface = material.Surface{Texture: image.Name, Region: [4]float32{0, 0, 1, 1}, Mode: material.Opaque, Opacity: 1, Frames: 1}
		if !used[image.Name] {
			used[image.Name] = true
			appearance.Textures = append(appearance.Textures, image)
		}
	}
	return appearance, nil
}

func dot(a, b geometry.Vec3) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}
