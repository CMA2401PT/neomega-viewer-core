package geometry

import (
	"embed"
	"io/fs"
)

// 从配套资源构建几何、元数据、实心表和贴图。
//go:generate go run ../../viewer-assets/cmd/build-tables

//go:embed geometry.nvtables geometry.nvtables.java.json geometry.nvtables.items.json.gz
var Files embed.FS

//go:embed textures
var textures embed.FS

// Resources 返回与预置几何匹配的公共图集和来源声明。
func Resources() fs.FS {
	files, _ := fs.Sub(textures, "textures")
	return files
}
