// Package javapreset 组装内置 Java 方块处理器，具体资源由各处理器持有。
package javapreset

import (
	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/blockactor/banner"
	"neomega-viewer-core/viewer/blockactor/decoratedpot"
	"neomega-viewer-core/viewer/blockactor/sign"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/effector"
	"neomega-viewer-core/viewer/entity_render"
	core "neomega-viewer-core/viewer/geometry"
	"neomega-viewer-core/viewer/item_render"
)

// NewPipeline 组装方块处理器和展示框；items 必须与 tables 使用同一套公共图集。
func NewPipeline(layout cube.Layout, tables *core.Tables[uint32], items *item_render.Catalog, missing *core.Shape) (*dispatcher.Pipeline, error) {
	pot, err := decoratedpot.New(tables)
	if err != nil {
		return nil, err
	}
	flag, err := banner.New(tables)
	if err != nil {
		return nil, err
	}
	board, err := sign.New(tables)
	if err != nil {
		return nil, err
	}
	return dispatcher.NewPipeline(
		layout,
		tables,
		effector.New(map[string]string{"minecraft:enchanting_table": "enchanting_table_book"}),
		blockactor.New(pot, flag, board),
		entity_render.New(items, item_render.New(items)),
		missing,
	), nil
}
