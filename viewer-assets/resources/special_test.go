// SPDX-License-Identifier: MIT
package resources

import (
	"testing"

	"neomega-viewer-core/viewer-assets/asset"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
)

func TestBannerTexturePathsFollowTheInputResources(t *testing.T) {
	old := jmodel.ResourceID("minecraft:entity/banner_base")
	modern := jmodel.ResourceID("minecraft:entity/banner/banner_base")
	for _, available := range []jmodel.ResourceID{old, modern} {
		got, err := NormalizeTextureReference(jmodel.TextureRef(old), map[jmodel.ResourceID]asset.File{available: {}})
		if err != nil || got != jmodel.TextureRef(available) {
			t.Fatalf("input contains %s; chose %s (%v)", available, got, err)
		}
	}
}
