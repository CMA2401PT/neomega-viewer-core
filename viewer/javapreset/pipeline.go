// Package javapreset 组装内置 Java 方块处理器，具体资源由各处理器持有。
package javapreset

import (
	"neomega-viewer-core/viewer/blockactor"
	"neomega-viewer-core/viewer/blockactor/banner"
	"neomega-viewer-core/viewer/blockactor/decoratedpot"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/effector"
	core "neomega-viewer-core/viewer/geometry"
)

// NewPipeline 组装附魔书、陶罐和旗帜；处理器自行加载内嵌图片，不需要外部资源目录。
func NewPipeline(layout cube.Layout, tables *core.Tables[uint32], missing *core.Shape) (*dispatcher.Pipeline, error) {
	pot, err := decoratedpot.New(tables)
	if err != nil {
		return nil, err
	}
	flag, err := banner.New(tables)
	if err != nil {
		return nil, err
	}
	return dispatcher.NewPipeline(
		layout,
		tables,
		effector.New(map[string]string{"minecraft:enchanting_table": "enchanting_table_book"}),
		blockactor.New(pot, flag),
		missing,
	), nil
}
