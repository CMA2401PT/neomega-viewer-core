# 普通静态物品

`item_render` 把物品资源编译成可以独立摆放的静态网格。物品 ID 先通过客户端 `items/*.json` 找到真实模型，再与方块共用模型解析器和公共图集。物品名称不决定模型路径，例如一个物品可以直接引用 `block/` 模型。

生成器将模型继承、纹理和 `fixed` 展示变换编译进 `Catalog`。`fixed` 是 Java 资源中用于展示框的缩放、旋转和平移。运行时的 `Renderer.Render` 只按物品 ID 返回几何副本，调用方再把它放进展示框或自己的场景。

## 几何与资源边界

带 `elements` 的模型复用 `java_block_model.Bake`，保留实际元素几何。继承 `builtin/generated` 的模型按各层纹理的透明轮廓生成侧壁，并保留带透明裁剪的正反面。侧壁厚度为一个方块的 1/16，位于模型深度 7.5/16 至 8.5/16。编译后以展示中心为原点，并应用一次继承后的 `fixed`。

目录与 `resources.Tables` 使用同一次构建的图集。纹理的动画条带和帧时保留在材质中，静态网格也可以播放纹理动画。`Render` 返回的四边形可以修改，共享目录在整个使用期间保持只读。

当前只接受没有 tint 的普通 `minecraft:model` 定义。复杂选择树、特殊模型、物品 components 和地图内容需要其他渲染能力，目录或展示框诊断会说明原因。模型或纹理引用损坏会使生成失败，不会当作正常空物品交付。

展示框由独立的 `entity_render` 使用这份目录。它根据实体朝向、物品八向旋转和框体偏移摆放网格，再追加到 `Frame.Mesh`。空框保留框体；物品失败时也保留可用框体，并返回实体诊断。发光框让物品使用自发光材质，框体仍使用普通材质。

## 取得一个物品网格

普通应用使用随库分发的目录，无需在运行时读取 JAR。下面程序可放到 viewer-core 模块内运行，它将钻石剑几何写到标准输出：

```go
package main

import (
    "log"
    "os"

    "neomega-viewer-core/viewer/item_render"
    preset "neomega-viewer-core/viewer/resources"
)

func main() {
    // 预置物品目录和方块几何共享配套图集。
    resources, err := preset.Open(preset.OpenOptions{})
    if err != nil { log.Fatal(err) }
    renderer := item_render.New(resources.Items)
    mesh, err := renderer.Render("minecraft:diamond_sword")
    if err != nil { log.Fatal(err) }
    if err := mesh.Marshal(os.Stdout); err != nil { log.Fatal(err) }
}
```

绘制这份 NVMS 网格时，使用 `preset.Resources()` 提供的 `atlas.png`。替换资源后，按[建表说明](../../viewer-assets/cmd/build-tables/README.md)重新生成方块表、物品目录和图集。
