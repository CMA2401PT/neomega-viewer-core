package tablegen

import (
	"fmt"

	java "neomega-viewer-core/viewer/javastates"
	"neomega-viewer-core/viewer-assets/cmd/build-tables/internal/resources"
	model "neomega-viewer-core/viewer/geometry/java_block_model"
)

type OpenOptions struct {
	AssetsDir string
	Policy    model.Policy
}

type Builder struct {
	Bundle    *model.Bundle
	models    *model.Resolver
	materials *model.MaterialLibrary
}

func Open(options OpenOptions) (*Builder, error) {
	assets, err := model.Load(options.AssetsDir)
	if err != nil {
		return nil, err
	}
	if assets.Info.Version != java.JavaTargetVersion {
		return nil, fmt.Errorf("resources are Java %s; Java states target %s", assets.Info.Version, java.JavaTargetVersion)
	}
	if options.Policy == nil {
		options.Policy = resources.PreviewPolicy{}
	}
	engine := &Builder{
		Bundle:    assets,
		models:    model.NewResolver(assets.Models),
		materials: model.NewMaterials(assets.Atlas, assets.AtlasImage, options.Policy),
	}
	return engine, nil
}
