# viewer-assets

`viewer-assets` 把 Minecraft Java 客户端 JAR 中分散的方块和物品资源，整理成渲染器可以加载的模型、渲染方式说明和纹理图集。它用 Go 实现；从解包到输出分成五步，每一步都留下可以独立检查和复用的中间文件。

理解整个流程，只需要先记住：**方块状态选择模型，模型引用纹理，图集坐标表告诉渲染器去 PNG 的哪里取这张纹理。** 但箱子、旗帜等方块的外形由游戏代码绘制，JAR 中没有完整方块模型。本工具为它们补充静态预览，并用另一张表说明完整渲染还需要什么数据。

一次构建处理一个版本。默认目标来自 `neomega-blocks/presets/java/v1.JavaTargetVersion`（当前为 **Minecraft Java 26.3**），使模型和图集与 Java 状态知识保持一致。客户端下载信息来自 [Mojang 官方版本清单](https://piston-meta.mojang.com/mc/game/version_manifest_v2.json)。版本信息记录在 `build-info.json` 中，数据表直接保存这个版本的内容。

## 先得到一份输出

需要 Go 1.25.5 或以上。需要相邻的 `neomega-blocks` 与 `neomega-nbt` 模块。共享模型定义和解析来自本模块的 `viewer/geometry/java_block_model`，提取与图集处理使用 Go 标准库。JAR 当作 ZIP 读取，因此运行工具不需要 Java、Node.js 或原生 canvas。

在 `neomega-viewer-core/viewer-assets` 目录执行：

```sh
go run ./cmd/neomega-assets build
```

命令按默认目标版本下载客户端到 `.cache/<版本>/client.jar`，然后依次运行五步。已经有 JAR 时，可以直接指定：

```sh
go run ./cmd/neomega-assets build -jar /path/to/client.jar
```

可以用 `-version` 显式选择其他版本，或用 `-version latest` 跟随最新正式版。与 `neomega-blocks` 配套时，应使用 `presets/java/v1.JavaTargetVersion` 对应版本。需要复现结果时，指定版本或固定的本地 JAR。

完成后，`dist/final` 中有以下配套产物：

| 文件 | 它回答的问题 |
|---|---|
| `blockstates.json` | 方块状态选择哪些模型？ |
| `items.json` | 普通物品 ID 引用哪个模型，哪些定义需要其他渲染能力？ |
| `models.json` | 模型的几何、继承和纹理引用是什么？ |
| `blockRenderers.json` | 可以使用静态模型吗？完整外观还依赖什么渲染方式和运行时数据？ |
| `blocksAtlases.json` | 每张纹理的位置、尺寸和动画帧信息是什么？ |
| `blocksAtlasLatest.png` | 坐标表指向的实际像素在哪里？ |

这些文件应整组使用。`build-info.json` 记录版本、来源和数量；`THIRD_PARTY_NOTICES.txt` 保存来源与许可声明。下载缓存在 `.cache`，中间生成文件在 `dist/works`，交付文件在 `dist/final`。`.cache` 与 `dist` 已加入 `.gitignore`。

## 读取交付文件

拿到输出后，消费者只需要 `dist/final`。`blockstates.json` 与 `models.json` 的根对象分别是资源名到定义的映射；`blocksAtlases.json` 的根对象直接包含 `textures`、尺寸和 UV 比例。这些文件没有 `latest` 层，也没有额外的合并文件。交付索引使用完整资源 ID，例如 `minecraft:stone`、`minecraft:block/stone` 和 `minecraft:entity/chest/normal`。

可以先运行独立的解析示例，确认应用会读到什么：

```sh
go run ./cmd/inspect-resources -dir dist/final
go run ./cmd/inspect-resources -dir dist/final \
  -block minecraft:chest \
  -model minecraft:block/stone \
  -texture minecraft:entity/chest/normal
```

示例读取全部表格和 PNG，再输出数量、指定方块的模型选择与渲染需求、模型定义和纹理像素范围。在 Go 应用中使用 `java_block_model.Load`，或者只读取需要的单张表。示例代码和模型解释流程见 [模型包说明](../viewer/geometry/java_block_model/README.md)。

## 从资源得到方块几何

`dist/final` 保存模型、状态、渲染类别和图集。消费者根据完整 Java 状态选择模型，再解析继承与旋转，得到局部几何。

模型定义、共享解析与烘焙由同模块的 `viewer/geometry/java_block_model` 提供，覆盖计算由 `viewer/geometry` 提供。
具体方块由 `neomega-blocks/ir.IRBlock` 表示。
资源准备完成后，在 viewer-core 模块根目录执行 `go generate ./viewer/resources`。
建表器直接读取 `viewer-assets/dist/final`，生成几何、元数据和实心表，并交付配套贴图。
输入、输出和验证方式见 [建表说明](cmd/build-tables/README.md)。

## 为什么分成五步

加载失败时，首先需要判断问题出在原始资源、模型关系、特殊渲染，还是图集坐标。五步分别处理这些边界，让问题可以在进入下一步之前被发现：

```text
客户端 JAR
  │
  ├─ s01_extract：提取资源，建立索引
  │      → dist/works/01-extracted
  │
  ├─ s02_patch：应用显式资源覆盖，加入诊断方块
  │      → dist/works/02-patched
  │
  ├─ s03_models：整理模型关系，找出没有几何的方块
  │      → dist/works/03-models
  │
  ├─ s04_special：补充特殊方块预览，说明渲染需求
  │      → dist/works/04-renderers
  │
  └─ s05_atlas：收集所需纹理，生成坐标和 PNG
         → dist/final
```

中间文件是步骤之间的接口。修改覆盖包后，可以从第二步继续。修改特殊方块处理后，可以从第四步继续。

## 第一步：知道 JAR 中有哪些资源

整理模型前，需要先知道资源在哪里。`s01_extract` 提取相关文件，并建立 `snapshot.json` 资源索引：

```sh
go run ./cmd/neomega-assets fetch
go run ./cmd/neomega-assets extract \
  -jar .cache/26.3/client.jar \
  -out dist/works/01-extracted
```

范围包括 `assets/<namespace>/blockstates`、`items`、`models`、纹理 PNG 和旁边的 `.png.mcmeta` 元数据。目录层级和命名空间会保留。

索引用 `minecraft:block/stone` 这样的**资源 ID** 作为键。它是资源的名字，与磁盘路径、游戏中的运行时方块 ID 是不同概念。索引中的文件路径相对于 `snapshot.json` 所在目录；来源字段记录对应的 JAR 条目。

实现入口：[s01_extract/extract.go](s01_extract/extract.go)。

## 第二步：应用需要的资源修正

有了原始索引，才能明确指定哪些文件需要替换。`s02_patch` 建立一份可加工的资源集，按顺序应用资源包目录：

```sh
go run ./cmd/neomega-assets patch \
  -in dist/works/01-extracted \
  -out dist/works/02-patched
```

没有覆盖包时，它复制原始资源，并加入 `unknown`、`errored` 两个诊断方块及程序生成的纹理。

需要修改资源时，准备标准资源包目录，例如：

```text
my-pack/
└─ assets/minecraft/
   ├─ models/block/stone.json
   └─ textures/block/stone.png
```

然后指定覆盖包：

```sh
go run ./cmd/neomega-assets patch \
  -in dist/works/01-extracted \
  -out dist/works/02-patched \
  -overlay ./my-pack
```

`-overlay` 可以重复，相同资源 ID 由最后一个覆盖包替换。替换单位是完整文件。索引记录最终来源。完整构建也接受相同的 `-overlay` 参数。

覆盖可以直接提供特殊方块的完整静态模型。第四步保留已有几何，只为仍然缺少几何的方块应用模板。

实现入口：[s02_patch/patch.go](s02_patch/patch.go)。

## 第三步：确认模型关系能够连起来

模型可能继承父模型的几何，再替换纹理变量。方块状态还可能选择多个加权候选，或通过 multipart 条件拼接多个模型。因此，收集纹理前需要解析这些关系。

```sh
go run ./cmd/neomega-assets models \
  -in dist/works/02-patched \
  -out dist/works/03-models
```

这一步生成 `blockstates.json`、`models.json`，以及独立的检查报告 `model-index.json`。它检查方块实际引用的模型、父模型和纹理变量，遇到缺失模型或循环时报告资源名。报告列出需要的纹理，并在 `geometrylessBlockStates` 中列出没有任何面的方块。

**没有缺失引用，不代表方块已经能显示。** 原始资源中仍有一些方块没有几何。箱子可能只有粒子纹理；空气本来就不可见；水则需要根据邻居生成表面。仅统计 `builtin/...` 父模型识别不了这些情况，这也是第四步存在的原因。

输出保留继承、变体、权重和逻辑条件。解析用于检查关系和收集纹理，具体选择仍由消费者根据方块状态完成。

模型类型集中在 [java_block_model](../viewer/geometry/java_block_model/README.md)，assets 的 `asset` 包保留提取快照、构建报告和源纹理动画元数据。其中用明确类型区分单个模型与加权候选、属性比较与 `OR`/`AND`、继承几何与显式清空几何，以及字符串纹理与现代 `sprite` 材质对象。未解释的模型顶层扩展字段也会保留。

实现入口：[s03_models/models.go](s03_models/models.go)；继承和纹理变量解析见 [共享 Resolver](../viewer/geometry/java_block_model/resolve.go)。

## 第四步：让特殊方块有可用的外形，并说明边界

箱子的静态外形可以用普通模型表达，但开盖角度来自游戏运行时。旗帜也有类似区别：基础外形可以预先生成，图案和飘动还需要额外数据。把这些情况全部当成“空模型”，会让合法方块消失；只补一个模型，又会掩盖完整渲染的要求。

因此，`s04_special` 同时生成静态预览和 `blockRenderers.json`：

```sh
go run ./cmd/neomega-assets special \
  -in dist/works/02-patched \
  -models dist/works/03-models \
  -out dist/works/04-renderers
```

这一步使用随工具附带的 `mc-assets` 社区模板，为箱子、潜影盒、旗帜、头颅、床、告示牌、饰纹陶罐、潮涌核心和末地传送门等补充静态模型。模板及其依赖按需导入 `neomega:` 命名空间，避免替换 JAR 的原生模型。已有原生几何直接保留。染色潜影盒选择对应颜色的实际皮肤。

这些模板、纹理和无法从 JAR 推导的渲染规则集中在 [resources](resources/README.md)。它们是需要手工维护的输入资料。s04 负责应用规则、导入依赖和检查结果；仅 `minecraft:air` 的处理直接留在步骤代码中。

模板的来源提交、文件映射和作者声明见 [resources/third_party/mc-assets/README.md](resources/third_party/mc-assets/README.md)。生成结果会记录每个模板的来源，并保留模型里的原始署名。

消费者需要知道如何使用这些结果，因此新表按方块资源 ID 给出以下分类：

| `kind` | 消费者如何理解 |
|---|---|
| `model` | 原生或覆盖模型提供静态几何 |
| `special` | 社区模板提供静态预览；完整外观可能还需要专用渲染 |
| `fluid` | 根据液位、邻居等信息生成流体表面 |
| `invisible` | 空气、屏障等，在普通世界视图中不绘制表面 |
| `dynamic` | 例如移动中的活塞，需要运行时内容和进度 |

先看 `staticPreview` 决定是否能直接使用模型表。需要更完整外观时，再看 `family` 和 `dynamicFields`，例如箱子的开盖角度、旗帜图案、玩家皮肤、告示牌文字。`stateProperties` 则是模型选择依赖的方块状态属性。新表说明这些需求，消费者负责实现对应渲染。

为让专用渲染器也有素材可用，`familyTextures` 保留相应纹理族，包括静态预览未直接引用的箱子皮肤、旗帜图案等。它们也会进入第五步的图集。

渲染说明、模型规则、模型与图集的键统一使用完整资源 ID。消费者直接按完整名称查询，不再为不同表裁剪命名空间或纹理路径。

遇到未识别的无几何方块时，这一步报错，并给出资源 ID。新增游戏方块后，应先判断它属于静态模型、流体还是其他渲染方式，再增加相应支持。

实现入口：[s04_special/special.go](s04_special/special.go)；分类、运行时需求与模板适配规则见 [resources/special.go](resources/special.go)，共享类别与描述见 [java_block_model/render.go](../viewer/geometry/java_block_model/render.go)，来源与统计报告见 [asset/render.go](asset/render.go)。

## 第五步：把所需纹理放进图集

完整模型和渲染需求已经明确，才能决定哪些纹理必须交付。`s05_atlas` 收录全部方块纹理、模型引用的其他纹理、普通物品纹理，以及第四步声明的纹理族：

```sh
go run ./cmd/neomega-assets atlas \
  -in dist/works/04-renderers \
  -models dist/works/04-renderers \
  -out dist/final
```

普通物品从 `items/*.json` 取得模型 ID，不根据物品名称猜测模型路径。除直接的 `minecraft:model` 外，还解析可确定的 `display_context=fixed` 和未使用、未装填等缺省状态分支，复用原有模型继承与 `fixed` 展示变换。常量 tint 按物品记录到 `items.json` 并在建表时应用；未知动态属性、特殊模型、动态 tint 等定义在 `items.json` 中保留不支持原因。普通模型的缺失引用使生成失败。展示框的两种框体模型也参与依赖收集。

普通方块纹理缩放到默认的 16×16 格子。动画方块纹理按 `.mcmeta` 的 `frames` 顺序逐帧缩放，再从上到下拼成一条；重复索引也保留。没有显式列表时按帧索引顺序收录全部帧。非块纹理保留原尺寸；带动画元数据时仍只取首帧。

为让消费者按时间切换采样，坐标表的 `frames` 给出条带中的帧数，`frameTime` 给出统一帧时，单位为游戏 tick。动画省略默认帧时时使用 1 tick；静态输出为 `frames=1`、`frameTime=0`。逐帧 `time` 暂不支持，统一使用 `.mcmeta` 的 `frametime`。

装箱先放大纹理，再填入其留下的格子空隙。宽、高分别取二次幂，按面积从小到大尝试；同面积优先接近正方形，再优先横向布局。相同输入产生稳定布局。坐标 `u/su` 除以 PNG 宽度，`v/sv` 除以高度；例如宽 1024 像素的图集中，横坐标 32、宽 16 的纹理对应 `u=32/1024`、`su=16/1024`。每张纹理都有完整的 `su/sv`，输出不再使用假定横纵比例相同的 `suSv` 字段。

动画的坐标覆盖整条区域。第 `i` 帧（从 0 开始）的纵向起点为 `v + i * sv / frames`，高度为 `sv / frames`；横向范围不变。消费者应保留整条区域及帧信息，再选择当前帧。

这一步同时导出方块状态表、物品模型引用、模型表、渲染说明、坐标表和 PNG，防止消费者把不同构建的文件混用。

实现入口：[s05_atlas/atlas.go](s05_atlas/atlas.go)。

## 怎样确认没有漏掉资源

图集大小受排列方式和纹理尺寸影响，不能单凭 PNG 外观判断完整性。需要核对 JAR 纹理、模型引用，以及旧版可见方块是否仍有外形。

需要比较历史资源覆盖时，先准备旧 `mc-viewer-svelte` 的 mc-assets 对照目录。该目录不是本仓库的生成产物；下面命令中的路径应指向实际取得的对照文件：

```sh
go run ./cmd/neomega-assets audit \
  -baseline ../mc-viewer-svelte/src/lib/Viewer/generate_res/mc-assets \
  -current dist/final \
  -snapshot dist/works/01-extracted \
  -raw-models dist/works/03-models \
  -completed dist/works/04-renderers \
  -out resources/audits/target-vs-viewer.json
```

这条命令检查原始 JAR 的方块纹理是否全部收录、所需纹理是否存在、PNG 尺寸与坐标是否一致，并比较补全前后的几何覆盖。全部差异名称保存在 JSON 中。**它检查资源覆盖，不等同于运行游戏渲染器逐像素比对外观。**

下表来自目标版本 Java 26.3 客户端的构建，可在本目录执行以下命令复现。原始数量取自 `dist/works/01-extracted/snapshot.json`，输出数量取自 `dist/final/build-info.json`，尺寸取自 `dist/final/blocksAtlasLatest.png`：

```sh
go run ./cmd/neomega-assets build \
  -jar .cache/26.3/client.jar \
  -work dist/works \
  -out dist/final
```

| 项目 | 结果 |
|---|---:|
| JAR 方块状态 / 模型 / PNG | 1,288 / 4,192 / 3,964 |
| 输出方块状态 / 模型 | 1,290 / 4,357 |
| 原生模型 / 特殊静态预览 | 1,194 / 86 |
| 流体 / 不可见 / 运行时动态 | 3 / 6 / 1 |
| 最终图集纹理数 / 尺寸 | 2,080 / 2048×1024 |

模型统计包含 block/item 模型及导入的模板依赖；PNG 统计包含实体与其他类别，因此这些数量不应相等。输出状态额外包含两个诊断方块。

旧 viewer 的历史名称和纹理布局可能与目标 JAR 不同，覆盖核对应比较具体资源及其用途。历史输入和核对结果见 [resources/audits/README.md](resources/audits/README.md)。

## 与 mc-viewer-svelte 衔接

资源生成完整之后，还需要消费者理解相应格式。资源命名、图集文件名和 UV 字段参考 `mc-assets`，但本工具交付拆开的单版本表。旧 viewer 需要按完整资源 ID 查询模型与纹理，并将合并文件和 `latest` 包装层改为读取 `blockstates.json`、`models.json` 与展开的 `blocksAtlases.json`。`blockRenderers.json` 补充特殊渲染需求。

历史外部项目 mc-viewer-svelte 的方块状态数据基于 Java `1.20.2`；这不是本工作区 viewer-core 的版本。要显示目标版本资源，需要同步更新状态数据与模型消费者，并接入渲染说明表。

消费者完成适配后，整组替换产物，再在 viewer 运行 `pnpm generate`。viewer 的 `rtids.json` 和渲染数据仍由它自己的生成流程负责。

## 在 Go 中调用与扩展

需要把某一步接入其他工具时，可以直接使用对应包。CLI 只负责组合这些步骤：

```go
import (
    "context"

    "neomega-viewer-core/viewer-assets/s04_special"
)

func complete(ctx context.Context) error {
    _, err := s04_special.Run(ctx, s04_special.Options{
        InputDir:  "dist/works/02-patched",
        ModelDir:  "dist/works/03-models",
        OutputDir: "dist/works/04-renderers",
    })
    return err
}
```

其他步骤同样提供 `Options` 和 `Run`。较长的文件处理接收 `context.Context`，便于取消。

扩展前先判断变化属于哪个问题：资源替换放在第二步的覆盖包；新模型字段放在 `viewer/geometry/java_block_model`，第三步直接使用共享解析；新特殊渲染需求和模板适配放在 `resources/special.go`，附带模板放在 `resources/third_party`，由第四步应用；纹理采样和排列放在第五步。更新附带模板时，应同步更新来源索引、作者声明与许可文件，并重跑资源核对。

## 验证实现

不准备 Minecraft 安装也能验证流程：

```sh
go test -race ./...
go vet ./...
```

测试位于 `utils/integration` 与各步骤目录。测试现场创建小型 JAR 和 PNG，覆盖最终文件解析、五步产物、箱子占位模型补全、未知空模型报错、命名空间、资源覆盖、模型继承、现代纹理对象、加权候选、逻辑条件、动画多帧顺序和帧时、图集空隙利用和像素坐标。测试不访问网络。

## 来源与许可

分步思路参考本工作区的 `my-mc-assets`。模型表与图集约定、特殊方块模板参考或取自 [zardoy/mc-assets](https://github.com/zardoy/mc-assets)，固定在提交 `c63d6dacd358a8ea73b453d6958167b49dcc3f6c`。模板包含 150 份方块状态、234 份依赖模型和 17 张自定义纹理，原始模型署名保留在 JSON 中。对应关系和修改说明见 [NOTICE.md](NOTICE.md) 与 [模板说明](resources/third_party/mc-assets/README.md)。

工具源码按 [MIT](LICENSE) 提供。上游 MIT 原文保存在 [resources/third_party/mc-assets/LICENSE](resources/third_party/mc-assets/LICENSE)。Minecraft 原始素材及其衍生纹理保留原权利人的权利；工具和上游代码的 MIT 声明不等于这些素材的再发布授权。分发生成结果时，应核对 [Minecraft EULA](https://www.minecraft.net/eula)、[Usage Guidelines](https://www.minecraft.net/usage-guidelines) 和覆盖包作者条款，并保留对应声明。
