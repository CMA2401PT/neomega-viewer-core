# 26.3 图集与旧 viewer 资源核对

PNG 看起来更空，可能来自排列、纹理尺寸或不同版本的资源名，也可能确实漏了特殊方块。因此核对分为纹理覆盖和可见几何两部分，逐项名称保存在 [26.3-vs-viewer.json](26.3-vs-viewer.json)。

## 输入与复现

当前资源来自 Mojang 的 Java 26.3 客户端 JAR。对照资源是工作区 `mc-viewer-svelte/src/lib/Viewer/generate_res/mc-assets` 中的模型表和图集坐标表。旧资源中的 `latest` 还包含历史名称，不能当作单版本 JAR 的目录清单。

在 `neomega-assets` 目录执行：

```sh
go run ./cmd/neomega-assets build -jar .cache/26.3/client.jar
go run ./cmd/neomega-assets audit \
  -baseline ../mc-viewer-svelte/src/lib/Viewer/generate_res/mc-assets \
  -current dist/final \
  -snapshot dist/works/01-extracted \
  -raw-models dist/works/03-models \
  -completed dist/works/04-renderers \
  -out resources/audits/26.3-vs-viewer.json
```

JAR 可先通过 `go run ./cmd/neomega-assets fetch -version 26.3` 获得。报告中的“补全前”读取第三步原始模型，“补全后”读取最终产物。

## 纹理是否有遗漏

| 检查项 | 结果 |
|---|---:|
| 旧图集纹理名 | 1,408 |
| 新图集纹理名 | 1,464 |
| 共有名称 | 1,029 |
| 仅旧表存在 | 379 |
| 仅新表存在 | 435 |
| 旧表独有名称中，仍存在于 26.3 JAR 的名称 | 0 |
| 26.3 JAR 方块 PNG | 1,300 |
| 未收录的 JAR 方块 PNG | 0 |
| 未收录的模型及渲染器所需纹理 | 0 |

379 个旧名称在当前 JAR 中不存在。例如旧版 `wool_colored_red`、`grass`、`chain` 与当前的 `red_wool`、`short_grass`、`iron_chain` 使用不同命名；床和告示牌还有纹理布局变化。名称比较只说明目录差异，不证明每张新旧纹理的像素等价。

未补充特殊渲染需求时，18 个旧表已有、当前 JAR 仍存在的实体纹理名没有进入图集。它们涉及箱子、头颅、潜影盒和饰纹陶罐。第四步收集对应纹理族后，这 18 个名称全部恢复。这 18 个名称见报告的 `beforeMissingBaselineTextures`；该清单根据原始模型的纹理引用和 JAR 索引重建补全前的选择。

最终 PNG 为 1024×1024。核对命令确认坐标表尺寸与实际 PNG 一致，并检查所有纹理矩形位于图内。

## 模型是否仍然可见

纹理存在还不够：箱子等方块的 JAR 模型没有表面。旧表中可见、原始模型中没有几何的方块有 67 种：

| 类别 | 数量 |
|---|---:|
| 站立与墙上旗帜 | 32 |
| 潜影盒 | 17 |
| 站立与墙上头颅、骷髅头 | 14 |
| 普通、陷阱、末影箱 | 3 |
| 饰纹陶罐 | 1 |

第四步补充后，上述 67 种方块全部具有静态几何。另为铜箱子、铜傀儡雕像、潮涌核心和末地传送门等提供预览，特殊预览总计 86 种。

原始模型中共 96 种方块没有几何。最终仍无静态几何的 10 种都有明确分类：6 种普通视图不可见方块、3 种流体、1 种移动活塞。对应信息见生成的 `blockRenderers.json`。

本报告检查资源名、引用、几何面的存在和图集边界。动画、玩家皮肤、旗帜图案、流体表面等还需要消费者实现；静态预览的视觉准确性也需要渲染验证。
