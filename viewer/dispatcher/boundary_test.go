package dispatcher_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// 分发器只认识通用处理器协议；新增 NBT 方块必须在组装层启用，不能让具体实现重新泄漏进来。
func TestDispatcherDoesNotImportConcreteHandlers(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(name, "neomega-viewer-core/viewer/blockactor/") || name == "neomega-viewer-core/viewer/javapreset" {
				t.Fatalf("通用分发器 %s 依赖了具体处理器组合 %s", path, name)
			}
		}
	}
}
