# Java 方块属性修复

fixer 根据已有 Java 状态和周围方块，补全默认属性并重算邻接相关属性。
它只返回需要修改的位置与状态，不读写存档，也不要求输入采用调色板或固定子区块。
是否需要修复由调用方决定；渲染器不会自动调用它。

为了让规则读取中心与邻居，调用方提供 `Input`：中心格子的筛选遍历、任意坐标查询，以及中心的水平范围。
遍历函数只返回筛选条件命中的格子，可以先按调色板筛选，从而跳过完全无关的子区块。
查询函数读取原始快照，缺数据时返回空气和 false。
水平范围按 X、Z 使用左闭右开坐标；树叶规则用这个边界判断是否可能由区域外的原木维持。

`Fix` 按顺序运行规则，每轮收齐改动后才供下一轮读取。
整个调用期间，输入和外部邻居保持只读。
返回值是中心格子的最终修改，坐标与输入一致；调用方决定如何复制、写回或保留这些修改。

## 使用

下面直接以坐标表提供方块，不需要子区块。
在 viewer-core 模块内保存程序，执行 `go run <程序路径>`：

```go
package main

import (
    "fmt"
    "iter"
    "log"

    "neomega-blocks/ir"
    fixer "neomega-viewer-core/fixer/java-postprocess"
)

func main() {
    f, err := fixer.New()
    if err != nil { log.Fatal(err) }
    blocks := map[[3]int]ir.IRBlock{
        {0, 0, 0}: "minecraft:oak_fence []",
        {1, 0, 0}: "minecraft:oak_fence []",
    }
    changes := f.Fix(fixer.Input{
        Bounds: fixer.HorizontalBounds{Max: [2]int{2, 1}},
        Blocks: func(accept func(ir.IRBlock) bool) iter.Seq2[[3]int, ir.IRBlock] {
            return func(yield func([3]int, ir.IRBlock) bool) {
                for pos, state := range blocks {
                    if accept(state) && !yield(pos, state) { return }
                }
            }
        },
        Get: func(x, y, z int) (ir.IRBlock, bool) {
            state, found := blocks[[3]int{x, y, z}]
            if !found { return "minecraft:air []", false }
            return state, true
        },
    })
    fmt.Println(changes)
}
```

已有固定 16³ 的 `celler/layer_storage.Palette` 时，使用应用层的 [fixpalette](../../../apps/neomega-converter/fixpalette/fix.go)。
它提供整列 `FixColumn` 与六面邻居的 `FixSection`，并负责只复制发生变化的旧 Palette。
转换导出与存档浏览共用这个适配，core 不依赖旧存储类型。

修复规则及近似限制见 [FIXER.md](FIXER.md)。
实心表由 [统一建表器](../../viewer-assets/cmd/build-tables/README.md) 生成。
存档遍历、写回、实体与诊断保留由 [neomega-converter](../../../apps/neomega-converter/README.md) 完成。
