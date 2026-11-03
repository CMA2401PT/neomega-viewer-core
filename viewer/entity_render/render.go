// Package entity_render 把普通实体列表转换为区域局部几何，与方块实体输入分开。
package entity_render

import (
	"fmt"
	"math"
	"slices"

	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/item_render"
)

// Entity 的 Pos 是区域局部坐标，NBT 保留来源数据并保持只读。
type Entity struct {
	ID  string
	Pos geometry.Vec3
	NBT nbt.Compound
}

// Input.List 决定本次输出；可选 Query 返回同一坐标系内的候选，允许多返回且不会自动参与输出。
type Input struct {
	List  []Entity
	Query func(geometry.Bounds) []Entity
}

// Issue 保留失败实体的局部位置和名称，不借用方块状态诊断。
type Issue struct {
	Index int
	ID    string
	Pos   geometry.Vec3
	Err   error
}

// Renderer 将普通实体组合为公共图集上的静态几何，不持有世界查询。
type Renderer struct {
	catalog *item_render.Catalog
	items   *item_render.Renderer
}

func New(catalog *item_render.Catalog, items *item_render.Renderer) *Renderer {
	// 两种渲染共享同一份公共图集目录，框体按模型资源 ID 读取。
	return &Renderer{catalog: catalog, items: items}
}

func (r *Renderer) Build(input Input) (geometry.Mesh, []Issue) {
	// 只认领展示框；物品局部失败时仍保留可用的框体。
	var result geometry.Mesh
	var issues []Issue
	for index, entity := range input.List {
		if entity.ID != "minecraft:item_frame" && entity.ID != "minecraft:glow_item_frame" {
			continue
		}
		mesh, err := r.frame(entity)
		result.Quads = append(result.Quads, mesh.Quads...)
		if err != nil {
			issues = append(issues, Issue{Index: index, ID: entity.ID, Pos: entity.Pos, Err: err})
		}
	}
	return result, issues
}

func (r *Renderer) frame(entity Entity) (geometry.Mesh, error) {
	// 使用原版框模型基底，先放框和物品，再统一按 Facing 转到区域坐标。
	facing, ok := entity.NBT["Facing"].(int8)
	if !ok || facing < 0 || facing > 5 {
		return geometry.Mesh{}, fmt.Errorf("item frame Facing must be a byte in 0..5")
	}
	invisible, _ := entity.NBT["Invisible"].(int8)
	glow := entity.ID == "minecraft:glow_item_frame"
	var result geometry.Mesh
	if invisible == 0 {
		id := "minecraft:block/item_frame"
		if glow {
			id = "minecraft:block/glow_item_frame"
		}
		body, ok := r.catalog.Models[id]
		if !ok {
			return result, fmt.Errorf("frame model %s missing", id)
		}
		result.Quads = slices.Clone(body.Quads)
	}
	var itemErr error
	if value, present := entity.NBT["Item"]; present {
		item, ok := value.(map[string]any)
		if !ok {
			itemErr = fmt.Errorf("item frame Item must be a compound")
		} else {
			id, ok := item["id"].(string)
			if !ok {
				itemErr = fmt.Errorf("item frame Item.id must be a string")
			} else if components, exists := item["components"]; exists && components != nil {
				itemErr = fmt.Errorf("unsupported item components for %s", id)
			} else {
				var mesh geometry.Mesh
				mesh, itemErr = r.items.Render(id)
				if itemErr == nil {
					rotation := int8(0)
					if value, present := entity.NBT["ItemRotation"]; present {
						var valid bool
						rotation, valid = value.(int8)
						if !valid {
							itemErr = fmt.Errorf("item frame ItemRotation must be a byte")
						}
					}
					if itemErr == nil {
						depth := 7.0 / 16
						if invisible != 0 {
							depth = .5
						}
						a := float64(rotation%8) * math.Pi / 4
						c, s := math.Cos(a), math.Sin(a)
						rotate := func(p geometry.Vec3) geometry.Vec3 { return geometry.Vec3{c*p[0] - s*p[1], s*p[0] + c*p[1], p[2]} }
						for i := range mesh.Quads {
							q := &mesh.Quads[i]
							for j, p := range q.Positions {
								p = rotate(p)
								p[0] *= .5
								p[1] *= .5
								p[2] = p[2]*.5 + depth
								q.Positions[j] = p
							}
							q.Normal = rotate(q.Normal)
							q.ShadeNormal = rotate(q.ShadeNormal)
							q.Surface.Unlit = q.Surface.Unlit || glow
						}
						result.Quads = append(result.Quads, mesh.Quads...)
					}
				}
			}
		}
	}
	orient := func(p geometry.Vec3) geometry.Vec3 {
		switch facing {
		case 0:
			return geometry.Vec3{-p[0], p[2], p[1]}
		case 1:
			return geometry.Vec3{-p[0], -p[2], -p[1]}
		case 2:
			return p
		case 3:
			return geometry.Vec3{-p[0], p[1], -p[2]}
		case 4:
			return geometry.Vec3{p[2], p[1], -p[0]}
		default:
			return geometry.Vec3{-p[2], p[1], p[0]}
		}
	}
	// 按原版朝向旋转后，沿法线偏移 15/32，使框体厚度围绕实体 Pos。
	normal := [6]geometry.Vec3{{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}}[facing]
	for i := range result.Quads {
		q := &result.Quads[i]
		for j, p := range q.Positions {
			p = orient(p)
			for axis := range p {
				p[axis] += entity.Pos[axis] + normal[axis]*15/32
			}
			q.Positions[j] = p
		}
		q.Normal = orient(q.Normal)
		q.ShadeNormal = orient(q.ShadeNormal)
	}
	return result, itemErr
}
