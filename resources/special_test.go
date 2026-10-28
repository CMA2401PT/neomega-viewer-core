// SPDX-License-Identifier: MIT
package resources

import (
	"testing"

	"neomega-assets/asset"
)

func TestBannerTexturePathsFollowTheInputResources(t *testing.T) {
	old := asset.ID("minecraft:entity/banner_base")
	modern := asset.ID("minecraft:entity/banner/banner_base")
	for _, available := range []asset.ID{old, modern} {
		got, err := NormalizeTextureReference(asset.TextureRef(old), map[asset.ID]asset.File{available: {}})
		if err != nil || got != asset.TextureRef(available) {
			t.Fatalf("input contains %s; chose %s (%v)", available, got, err)
		}
	}
}
