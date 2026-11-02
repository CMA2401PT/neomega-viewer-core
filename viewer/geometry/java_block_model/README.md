# Java 方块模型的定义与解释

`java_block_model` 把 Java 资源模型解释为方块格子内的静态几何。它定义模型、模型选择规则和纹理图集，提供文件读取、继承解析、属性匹配和烘焙。`viewer-assets` 使用同一套定义提取、加工资源并生成预置表。

一个具体方块由 `ir.IRBlock` 表示，例如 `minecraft:oak_slab ["type"="bottom"]`。`ModelRules` 则描述资源包中的 variants 和 multipart：前者按属性选择模型，后者把多个满足条件的模型部件拼起来。规则的输入是 IR 属性，不需要额外的方块状态结构。

## 输入与职责

为了把形状与纹理连起来，`Load(dir)` 读取同一次构建的六份文件：

| 文件 | 在解释过程中的用途 |
| --- | --- |
| `blockstates.json` | 方块名称到 `ModelRules` 的映射。 |
| `models.json` | 模型定义、父引用、元素和纹理槽。 |
| `blocksAtlases.json` | 完整纹理资源 ID 对应的图集区域和动画信息。 |
| `blocksAtlasLatest.png` | 供材质库采样透明度的像素。 |
| `blockRenderers.json` | 普通模型、特殊预览、流体、不可见和动态类别。 |
| `build-info.json` | 资源版本。读取端只消费版本，不承接生产端的构建统计。 |

资源 ID 是模型或纹理的名称，例如 `minecraft:block/stone`，与 IR 方块及数值状态 hash 不同。内存索引和交付图集使用完整资源 ID，保留 `block/`、`entity/` 等路径。读取 Java JSON 时，省略的命名空间按 `minecraft:` 补齐；直接在 Go 中构造资源表时，由调用方提供完整 ID。

模型定义保留显式空元素与未提供元素的区别：前者清空几何，后者继承父模型。未解释的顶层扩展字段、显示变换和模型署名仍参与读写，防止 assets 加工文件时丢失原始信息。

提取快照、JAR 下载信息、纹理源文件的 `.mcmeta`、打包统计和素材来源记录属于 assets 的生产过程。这里读取的是生产过程交付的模型与图集。来源约定见 [NOTICE](../../../third_party/mc-assets-notices/NOTICE.md)。

## 从 IR 到四边形

`Select(rules, states)` 接收 Java 字符串属性，返回全部命中的 `Choice`。每组保留所有候选的权重和变换，所有命中的 multipart 部件也保留。静态表生成器固定选每组首项；其他调用方可以实现自己的选择策略。

`IRStates` 只统一属性表示，不自动转换版本。Java 的 `"true"` 和 `"8"` 是字符串，不能用 Bedrock 的布尔值、整数属性直接替代。`Select` 不要求属性已经排序；对方块直接取 `Hash()` 时仍必须先 Tidy。Java preset 的 `JavaFullStates` 和 `HashToBlock` 返回值已经 tidy。

`Resolver` 展开父模型与纹理槽引用，缓存只读结果。创建后不要修改模型表或返回的模型。`builtin/*` 表示需要游戏专用绘制的模型，解析结果会保留该信息；显式元素可以替代它。`Bake` 对仍依赖 builtin 的模型返回错误，避免把它当成空几何。

纹理透明度与方块的外观规则共同决定表面材质。`NewMaterials(atlas, pixels, policy)` 建立可共享的材质库，`ForBlock(block)` 将材质策略绑定到一个 IR 方块。`Bake(resolved, application, materials)` 只通过 `FaceMaterials` 查询表面和染色，然后处理元素旋转、模型旋转、UV 锁定及法线。

玻璃透明规则、固定群系颜色、红石颜色和流体预览由调用方的 `Policy` 提供。生成器中的默认实现位于 `viewer-assets/cmd/build-tables/internal/resources`。临时补充干燥属性或计算满高度液面时，只修改用于烘焙的 IR，表索引仍来自原始状态。

## 读取并检查一个方块的模型

下面的程序可以保存到 `neomega-viewer-core/examples/inspect-model/main.go`。从模块根目录运行：

```sh
go run ./examples/inspect-model -dir viewer-assets/dist/final
```

```go
package main

import (
    "flag"
    "fmt"

    model "neomega-viewer-core/viewer/geometry/java_block_model"
    "neomega-blocks/ir"
)

func main() {
    dir := flag.String("dir", "", "directory containing the six resource files")
    flag.Parse()
    bundle, err := model.Load(*dir)
    if err != nil { panic(err) }
    block := ir.IRBlock(`minecraft:oak_slab ["type"="bottom"]`).Tidy()
    rules, ok := bundle.Rules[model.ResourceID(block.NameWithNameSpace())]
    if !ok { panic("model rules missing") }
    choices, err := model.Select(rules, block.States())
    if err != nil { panic(err) }
    resolver := model.NewResolver(bundle.Models)
    for _, choice := range choices {
        // 检查时明确选择每组的第一个模型候选。
        application := choice.Options[0]
        resolved, err := resolver.Resolve(application.Model)
        if err != nil { panic(err) }
        fmt.Println(application.Model, len(resolved.Elements), resolved.Builtin)
    }
}
```

只检查单张表时，可以调用 `ReadRules`、`ReadModels`、`ReadAtlas` 或 `ReadRenderers`。需要完整的预置几何与实心表时，使用[生成入口](../../../viewer-assets/cmd/build-tables/README.md)，无需自行组合这些步骤。
