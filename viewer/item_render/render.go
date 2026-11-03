// Package item_render 把普通静态物品编译为公共图集上的固定展示几何。
package item_render

import (
	"fmt"

	"neomega-viewer-core/viewer/geometry"
	"slices"
)

// Catalog 与公共图集配套交付；Items 使用物品 ID，Models 使用直接引用的模型资源 ID。
// 目录只读，Unsupported 保存资源定义无法用静态模型表达的原因。
type Catalog struct {
	Items       map[string]geometry.Mesh
	Models      map[string]geometry.Mesh
	Unsupported map[string]string
}

// Renderer 共享只读目录，不持有世界、实体或 NBT。
type Renderer struct{ catalog *Catalog }

func New(catalog *Catalog) *Renderer {
	// 调用者在 Renderer 生命周期内保持目录不变。
	return &Renderer{catalog: catalog}
}

func (r *Renderer) Render(id string) (geometry.Mesh, error) {
	// 返回以展示中心为原点、已应用 fixed 变换的副本，调用者可以自由摆放。
	mesh, ok := r.catalog.Items[id]
	if !ok {
		if reason, found := r.catalog.Unsupported[id]; found {
			return geometry.Mesh{}, fmt.Errorf("unsupported item %s: %s", id, reason)
		}
		return geometry.Mesh{}, fmt.Errorf("static item %s missing", id)
	}
	return geometry.Mesh{Quads: slices.Clone(mesh.Quads)}, nil
}
