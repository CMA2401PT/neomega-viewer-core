# Java 方块属性修复

fixer 根据中心柱子及周围八根柱子的 Java 状态，补全默认属性并重算邻接相关属性。
它可以处理转换生成的状态或其他来源的 Java 方块数据。
是否需要修复由调用方决定；渲染器不会自动调用它。

一根柱子用 `Column` 表示，即 section 高度到 Java 方块调色板的映射。
section 是 16×16×16 的方块区域。
`Fix` 返回新的高度映射，只复制改变的调色板，输入和邻居保持只读。
八邻居按西北、北、东北、西、东、西南、南、东南提供；缺失用 nil 表示。
邻居必须始终取修复前的数据，否则结果会受访问顺序影响。

## 使用

在 viewer-core 模块内保存下面程序，执行 `go run <程序路径>`：

```go
package main

import (
    "fmt"
    "log"

    "celler/layer_storage"
    "neomega-blocks/ir"
    fixer "neomega-viewer-core/fixer/java-postprocess"
)

func main() {
    f, err := fixer.New()
    if err != nil { log.Fatal(err) }
    blocks := layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:air []"))
    blocks.Set(layer_storage.At(1, 2, 3), "minecraft:oak_fence []")
    fixed := f.Fix(fixer.Column{0: blocks}, fixer.ChunkNeighbours{})
    fmt.Println(fixed[0].Get(layer_storage.At(1, 2, 3)))
}
```

单个 section 预览使用 `FixSection`，六面邻居顺序是西、东、下、上、北、南。
修复规则及近似限制见 [FIXER.md](FIXER.md)。
实心表由 [统一建表器](../../viewer-assets/cmd/build-tables/README.md) 生成。
存档遍历、写回、实体与诊断保留由 [neomega-converter](../../../apps/neomega-converter/README.md) 完成。
