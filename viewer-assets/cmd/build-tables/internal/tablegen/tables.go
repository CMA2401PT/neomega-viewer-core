package tablegen

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"neomega-blocks/ir"
	java "neomega-viewer-core/viewer/javastates"
	"neomega-viewer-core/fixer/solidtable"
	"neomega-viewer-core/viewer-assets/cmd/build-tables/internal/resources"
	"neomega-viewer-core/viewer/geometry"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
	preset "neomega-viewer-core/viewer/resources"
)

type shapeBucket struct {
	// 面数和首面只缩小查找范围，完整四边形序列相等才共享 ID。
	// Bake 受状态的材质与颜色影响，仅按模型路径和旋转会误合并。
	Group uint32
	Count int
	First geometry.Quad
}

func intern(tables *geometry.Tables[uint32], mesh geometry.Mesh, group uint32, buckets map[shapeBucket][]geometry.ModelID) geometry.ModelID {
	key := shapeBucket{Group: group, Count: len(mesh.Quads)}
	if len(mesh.Quads) != 0 {
		key.First = mesh.Quads[0]
	}
	for _, id := range buckets[key] {
		if slices.Equal(tables.Shapes[id].Quads, mesh.Quads) {
			return id
		}
	}
	id := geometry.ModelID(len(tables.Shapes))
	tables.Shapes = append(tables.Shapes, geometry.Shape{
		Quads:     mesh.Quads,
		CullGroup: group,
		Boundary:  geometry.BoundaryCoverage(mesh.Quads),
	})
	buckets[key] = append(buckets[key], id)
	return id
}

type Result struct {
	*geometry.Tables[uint32]
	preset.Metadata
	Solid solidtable.Table
}

func (engine *Builder) Build() (*Result, error) {
	states := slices.Collect(java.JavaFullStates())
	// Stable hash ordering makes shape IDs reproducible without a second identity table.
	slices.SortFunc(states, func(a, b ir.IRBlock) int { return cmp.Compare(a.Hash(), b.Hash()) })

	tables := &Result{
		Solid: make(solidtable.Table, len(states)),
		Tables: &geometry.Tables[uint32]{
			Index:      make(map[uint32][]geometry.ModelID, len(states)),
			FullHeight: map[geometry.ModelID]geometry.ModelID{},
		},
		Metadata: preset.Metadata{
			Version:   engine.Bundle.Info.Version,
			Renderers: map[string]preset.Renderer{},
		},
	}
	for name, profile := range engine.Bundle.Renderers.Blocks {
		tables.Renderers[string(name)] = preset.Renderer{Kind: profile.Kind, Family: profile.Family}
	}
	groups := map[string]uint32{}
	buckets := map[shapeBucket][]geometry.ModelID{}
	for _, block := range states {
		javaHash := block.Hash()
		meshes, err := engine.bakeParts(block)
		if err != nil {
			tables.Skipped = append(tables.Skipped, preset.SkippedState{Block: block, Reason: err.Error()})
			continue
		}
		profile := engine.Bundle.Renderers.Blocks[jmodel.ResourceID(block.NameWithNameSpace())]
		group := cullGroup(block, profile.Kind, profile.Family, groups)
		var full []geometry.Mesh
		if profile.Kind == jmodel.RendererFluid {
			full, err = engine.bakeParts(ir.IRBlock(block.NameWithNameSpace() + " " + string(block.States().Set("level", "8"))))
			if err != nil {
				return nil, err
			}
		}
		// 整个状态成功后再编号，失败状态不会留下部分索引或无引用的 Shape。
		ids := make([]geometry.ModelID, 0, len(meshes))
		for _, mesh := range meshes {
			ids = append(ids, intern(tables.Tables, mesh, group, buckets))
		}
		tables.Index[javaHash] = ids
		// 多部件可能共同覆盖整面，不能逐部件判断后简单合并实心位。
		var combined geometry.Mesh
		for _, mesh := range meshes {
			combined.Quads = append(combined.Quads, mesh.Quads...)
		}
		var solid uint8
		for side, name := range [...]string{"west", "east", "down", "up", "north", "south"} {
			if geometry.OpaqueBoundary(combined.Quads, name) {
				solid |= 1 << side
			}
		}
		tables.Solid[javaHash] = solid
		if len(full) != 0 {
			tables.FullHeight[ids[0]] = intern(tables.Tables, full[0], group, buckets)
		}
	}
	// Legacy indexes reuse target shapes, preserving tint, cull groups and fluids.
	for mapping := range java.JavaCompatibility() {
		source, target := mapping.Source.Hash(), mapping.Target.Hash()
		ids, ok := tables.Index[target]
		if !ok {
			tables.Skipped = append(tables.Skipped, preset.SkippedState{Block: mapping.Source, Reason: "compatibility target has no static geometry: " + string(mapping.Target)})
			continue
		}
		tables.Index[source] = ids
		tables.Solid[source] = tables.Solid[target]
		if _, exists := tables.Renderers[mapping.Source.NameWithNameSpace()]; !exists {
			tables.Renderers[mapping.Source.NameWithNameSpace()] = tables.Renderers[mapping.Target.NameWithNameSpace()]
		}
	}
	return tables, nil
}

func (engine *Builder) bakeParts(state ir.IRBlock) ([]geometry.Mesh, error) {
	profile, ok := engine.Bundle.Renderers.Blocks[jmodel.ResourceID(state.NameWithNameSpace())]
	if !ok {
		return nil, fmt.Errorf("no renderer entry for %s", state.NameWithNameSpace())
	}
	switch profile.Kind {
	case jmodel.RendererInvisible:
		return nil, nil
	case jmodel.RendererDynamic:
		return nil, fmt.Errorf("%s has no fixed static shape", state.NameWithNameSpace())
	case jmodel.RendererFluid:
		mesh, err := jmodel.Bake(
			resources.FluidPreview(state, profile.Family),
			jmodel.Apply{},
			engine.materials.ForBlock(state),
		)
		// 只有真正位于格子边界的流体面才允许邻居剔除。
		for side, covers := range geometry.BoundaryCoverage(mesh.Quads) {
			for _, cover := range covers {
				mesh.Quads[cover.Quad].CullFace = geometry.CullFace(side + 1)
			}
		}
		return []geometry.Mesh{mesh}, err
	}
	definition, ok := engine.Bundle.Rules[jmodel.ResourceID(state.NameWithNameSpace())]
	if !ok {
		return nil, fmt.Errorf("blockstate %s missing", state.NameWithNameSpace())
	}
	if definition.UsesProperty("waterlogged") {
		state = resources.PreviewProperties(state)
	}
	choices, err := jmodel.Select(definition, state.States())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", state.NameWithNameSpace(), err)
	}
	meshes := make([]geometry.Mesh, 0, len(choices))
	for _, choice := range choices {
		// Static presets deliberately choose the first candidate; multipart parts remain independent.
		application := choice.Options[0]
		id := application.Model
		resolved, err := engine.models.Resolve(id)
		if err != nil {
			return nil, err
		}
		mesh, err := jmodel.Bake(
			resolved,
			application,
			engine.materials.ForBlock(state),
		)
		if err != nil {
			return nil, fmt.Errorf("model %s: %w", id, err)
		}
		meshes = append(meshes, mesh)
	}
	return meshes, nil
}

func cullGroup(state ir.IRBlock, kind jmodel.RendererKind, family string, groups map[string]uint32) uint32 {
	// 名称规则只在建表时消费，形状去重也必须区分消面语义。
	key := ""
	if kind == jmodel.RendererFluid {
		key = "fluid:" + family
	} else if strings.Contains(state.ShortName(), "glass") {
		key = "glass:" + state.NameWithNameSpace()
	}
	if key == "" {
		return 0
	}
	if groups[key] == 0 {
		groups[key] = uint32(len(groups) + 1)
	}
	return groups[key]
}
