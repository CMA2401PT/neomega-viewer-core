package topview

import (
	"bytes"
	_ "embed"
	"sync"
)

// 由 viewer-assets/cmd/build-tables 与几何表一起生成。
// 在模块根目录运行 go generate ./viewer/resources。
//
//go:embed colors.nvcolor
var bundled []byte

var decodeBundled = sync.OnceValues(func() (Table, error) {
	return Decode(bytes.NewReader(bundled))
})

func Bundled() (Table, error) {
	// Bundled 返回随包分发的颜色表。多个调用方共享同一次解码结果，不应修改返回的表。
	return decodeBundled()
}
