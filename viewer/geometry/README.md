# 方块的静态几何

`geometry` 描述一个方块状态在自身格子内具有的外形。模型选择、继承与旋转可以提前算好，保存为共享的形状和查询表。viewer 使用这些形状拼装场景，converter 使用同一套几何算出的实心表判断连接条件。

通用几何代码与预置资源分开。
这里定义几何和算法，`viewer/resources` 分发已生成几何，`fixer/solidtable/preset` 分发实心表。
Java 状态仍由 [blocks 的 preset](../../../neomega-blocks/presets/README.md) 定义。
Java 模型解析与烘焙在 [java_block_model](java_block_model/README.md)，资源生产和建表在同模块的 viewer-assets 中完成。

## 从状态得到局部形状

具体方块统一用 `ir.IRBlock` 表示。Java 模型规则描述“哪些属性命中哪些模型”，不是另一种方块状态。建表时把 IR 属性交给规则解释器，再展开模型继承、选择纹理并烘焙为四边形。

| 包 | 作用 |
| --- | --- |
| `geometry/java_block_model` | 定义和读取模型规则、模型与图集；匹配 IR 属性、解析继承与纹理、执行旋转和 UV 烘焙。 |
| `geometry/material` | 保存烘焙后的表面属性，提供完全不透明的判断。 |
| `fixer/solidtable` | 保存和读写六方向实心位。 |

模型规则返回全部命中的候选组，选择哪个候选由调用方决定。固定取首项、玻璃透明规则、固定草木染色、干燥默认属性和静态流体模型属于[生成器策略](../../viewer-assets/cmd/build-tables/README.md)。烘焙只接收模型、变换和已经绑定到当前方块的材质查询，无需重新表达方块身份。

## 为什么需要边界覆盖

一个模型可能由多个局部矩形共同盖住整面，也可能只盖住下半部分。`BoundaryCoverage` 保存实际位于格子边界的轴对齐矩形，供 viewer 按邻居覆盖裁切。`OpaqueBoundary` 将全部部件合在一起检查，判断某个方向是否被完全不透明的矩形完整覆盖。

覆盖检查使用真实顶点与绕序，不依赖模型包围盒或 `CullFace`。斜面和不规则四边形不会被当作完整的轴对齐覆盖。`InspectRectangle` 与 `Subtract` 同时服务于静态建表和 viewer 的场景裁切，使用相同的旋转误差容限。

六方向顺序统一为 west、east、down、up、north、south。几何坐标以一个方块为一单位，Y 向上，北向为负 Z。局部计算保留 float64；viewer 转成绘制缓冲时再降低到 float32。

## 保存与使用查询表

大量状态会共享同一个模型部件，因此 `Tables[B]` 分别保存数值状态键到部件编号的索引、共享的 `Shape`，以及流体满高度形状的映射。`B` 可以是 uint32、uint64 或对应的自定义类型。表和查询返回的切片按只读数据使用。

`Tables.Encode` 与 `DecodeTables` 读写 NVGT v2，保存烘焙精度、材质、边界覆盖和消面信息，具体布局见 [FORMAT.md](FORMAT.md)。`fixer/solidtable` 读写 NVST v1，每个状态保存六个位，记录哪些面完全不透明。

使用现成 Java 表时无需加载模型或 PNG：

```go
package main

import (
    "fmt"

    "neomega-blocks/ir"
    javaGeometry "neomega-viewer-core/viewer/resources"
)

func main() {
    resolver, err := javaGeometry.Open(javaGeometry.OpenOptions{})
    if err != nil {
        panic(err)
    }
    javaHash := ir.IRBlock("minecraft:stone []").Tidy().Hash()
    ids, err := resolver.Tables.ModelIDs(javaHash)
    if err != nil {
        panic(err)
    }
    fmt.Println(ids)
}
```

这里的 `javaHash` 必须来自已整理的 Java 状态。Java 身份直接通过 `presets/java/v1.HashToBlock` 查询，返回已有的 IR 表示；需要处理文本时，先遵守 [preset 的 Tidy 约定](../../../neomega-blocks/presets/README.md)。

viewer 的 `mesher` 接收这里的表，结合位置和邻居生成场景网格。绘制缓冲、NVMS 网格传输及 NVFR 场景快照由 viewer 提供。
