# 方块资源覆盖核对

图集外观和纹理数量不能直接说明资源是否完整。版本变化会带来重命名和纹理布局调整，而箱子、床等方块还需要 JAR 以外的静态模型。因此核对同时检查纹理收录与可见几何。

默认目标为 Java 1.21.4，与 `neomega-blocks` 的 Java 输出一致。下列对照数量引用随库保存的历史核对记录，逐项名称保存在 [1.21.4-vs-viewer.json](1.21.4-vs-viewer.json)。[26.3-vs-viewer.json](26.3-vs-viewer.json) 保留了先前对 26.3 输入的核对记录。

## 输入与复现

当前资源来自 Mojang 的 Java 1.21.4 客户端 JAR。对照资源是工作区 `mc-viewer-svelte/src/lib/Viewer/generate_res/mc-assets` 中的模型表和图集坐标表。复跑前需要另外准备该历史目录，并将命令中的 `-baseline` 改为实际路径。对照文件中的 `latest` 包含历史名称，因此需要结合目标 JAR 核实差异。

在 `neomega-assets` 目录执行：

```sh
go run ./cmd/neomega-assets build -version 1.21.4
go run ./cmd/neomega-assets audit \
  -baseline ../mc-viewer-svelte/src/lib/Viewer/generate_res/mc-assets \
  -current dist/final \
  -snapshot dist/works/01-extracted \
  -raw-models dist/works/03-models \
  -completed dist/works/04-renderers \
  -out resources/audits/1.21.4-vs-viewer.json
```

报告中的“补全前”读取第三步模型，“补全后”读取最终产物。需要确认转换结果能使用这些资源时，再运行 [blocks 生成工具说明](../../../neomega-blocks/third_party/gen_resource/README.md)中的 `TestJavaTargetAssets`，将真实 palette 的转换输出与资源版本、方块名和 variant 核对。

## 纹理是否有遗漏

| 检查项 | 结果 |
|---|---:|
| 旧图集纹理名 | 1,408 |
| 新图集纹理名 | 1,218 |
| 共有名称 | 1,071 |
| 仅旧表存在 | 337 |
| 仅新表存在 | 147 |
| 旧表独有名称中，仍存在于 1.21.4 JAR 的名称 | 0 |
| 1.21.4 JAR 方块 PNG | 1,039 |
| 未收录的 JAR 方块 PNG | 0 |
| 未收录的模型及渲染器所需纹理 | 0 |

337 个旧名称在当前 JAR 中不存在，包含历史命名和旧版纹理布局。名称比较只说明目录差异，不证明新旧纹理逐像素等价。

补充特殊模型与纹理族之前，旧表中有 57 个仍存在于当前 JAR 的纹理名没有进入原始图集选择。它们的名称见报告的 `beforeMissingBaselineTextures`。最终这些纹理均已收录。

随库历史报告中的 PNG 为 1024×1024。上面的 Java 1.21.4 构建命令使用完整动画条带，输出为 2048×1024；实际尺寸由纹理集合与帧信息决定。核对命令检查坐标表尺寸与 PNG 一致，并检查所有纹理矩形位于图内。

## 模型是否仍然可见

纹理存在还不够：床、告示牌等方块的 JAR 模型没有表面。旧表中可见、原始模型中没有几何的方块有 127 种：

| 类别 | 数量 |
|---|---:|
| 站立与墙上旗帜 | 32 |
| 潜影盒 | 17 |
| 站立与墙上头颅、骷髅头 | 14 |
| 普通、陷阱、末影箱 | 3 |
| 饰纹陶罐 | 1 |
| 床 | 16 |
| 告示牌与悬挂告示牌 | 44 |

第四步补充后，上述 127 种方块全部具有静态几何。1.21.4 最终共有 134 种特殊方块静态预览。

原始模型中共 144 种方块没有几何。最终仍无静态几何的 10 种都有明确分类：6 种普通视图不可见方块、3 种流体、1 种移动活塞。对应信息见 `blockRenderers.json`。

本报告检查资源名、引用、几何面的存在和图集边界。具体外观还需要消费者正确处理模型选择、继承、旋转与材质，并通过渲染验证。
