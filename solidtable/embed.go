package solidtable

import (
	"bytes"
	_ "embed"
	"sync"
)

// 实心表由 neomega-viewer 的几何生成器写入本目录，与几何表使用同一套资源产出。
//
//go:embed geometry.nvsolid
var bundled []byte

// Bundled 返回随包分发的实心表；多个调用方共享同一份只读解码结果。
var Bundled = sync.OnceValues(func() (Table, error) {
	return Decode(bytes.NewReader(bundled))
})
