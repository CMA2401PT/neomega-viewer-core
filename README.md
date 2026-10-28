# neomega-assets

`neomega-assets` 把一个 Minecraft Java 客户端 JAR 中分散的方块资源整理成三份配套产物：**方块状态和模型表、纹理坐标表、纹理图集 PNG**。它用 Go 实现，每一步都可以单独运行，并把输入、输出和来源写成可检查的文件。

要理解这个流程，只需要先记住一条关系：**方块状态选择模型，模型引用纹理，图集坐标表告诉渲染器去 PNG 的哪里取这张纹理。** JAR 里保存的是分散资源；这三个产物把它们组织成应用便于加载的形式。

本模块先处理一个版本。本文以 **Minecraft Java 26.3** 为例，版本来自 [Mojang 官方版本清单](https://piston-meta.mojang.com/mc/game/version_manifest_v2.json)。输出里的 `latest` 表示这次选择的版本。

## 先得到一份可用的输出

需要 Go 1.25.5 或以上。实现只使用 Go 标准库。JAR 当作 ZIP 读取，因此运行工具不需要 Java、Node.js 或原生 canvas。

在 `neomega-assets` 目录执行：

```sh
go run ./cmd/neomega-assets build -version 26.3
```

命令从 Mojang 的清单找到客户端 JAR，下载到 `.cache/26.3/client.jar`，再依次执行四个步骤。已经有 JAR 时，也可以直接指定：

```sh
go run ./cmd/neomega-assets build -jar /path/to/client.jar
```

希望跟随最新正式版时，用 `-version latest`。需要复现一次生成结果时，使用明确版本或固定的本地 JAR。

完成后，`dist` 中最关键的是：

| 文件 | 它回答的问题 |
|---|---|
| `blockStatesModels.json` | 某个方块状态使用哪些模型？模型的几何、父模型、纹理引用是什么？ |
| `blocksAtlases.json` | 某张纹理在图集里的位置和尺寸是什么？ |
| `blocksAtlasLatest.png` | 坐标表引用的实际像素在哪里？ |

图集和坐标表必须配套使用。`dist/build-info.json` 记录输入版本、来源和数量；`dist/THIRD_PARTY_NOTICES.txt` 保存来源与许可声明。`.cache`、`work`、`dist` 都已加入 `.gitignore`。

## 为什么分成四步

如果把解包、补模型、处理继承和绘图放进一个函数，出现错误时很难判断是哪种数据出了问题。因此，每一步只处理一个已经明确的问题：

```text
客户端 JAR
  │
  ├─ s01_extract：找到资源，建立索引
  │      → work/01-extracted/snapshot.json + 原始资源
  │
  ├─ s02_patch：应用明确指定的资源覆盖
  │      → work/02-patched/snapshot.json + 处理后的资源
  │
  ├─ s03_models：整理模型，检查引用关系
  │      → work/03-models/blockStatesModels.json + models.json
  │
  └─ s04_atlas：读取所需纹理，生成一对坐标表和 PNG
         → dist 中的三份配套产物与来源说明
```

中间文件是步骤之间的接口。修改一个模型或覆盖包后，可以从对应步骤重新开始。

## 第一步：从 JAR 建立资源索引

在整理模型之前，需要先知道资源实际有哪些、分别存在哪里。`s01_extract` 只提取相关资源，并建立 `snapshot.json`：

```sh
go run ./cmd/neomega-assets fetch -version 26.3
go run ./cmd/neomega-assets extract \
  -jar .cache/26.3/client.jar \
  -out work/01-extracted
```

提取范围包括 `assets/<namespace>/blockstates`、`models`、纹理 PNG，以及纹理旁边的 `.png.mcmeta` 元数据。目录层级和命名空间会保留。

索引使用 `minecraft:block/stone` 这样的**资源 ID**作为键。资源 ID 是资源的名字，与磁盘路径、游戏运行时方块 ID 是不同概念。索引中的 `File.Path` 相对于 `snapshot.json` 所在目录；`File.Origin` 说明资源来自 JAR 的哪个条目。

实现入口：[s01_extract/extract.go](s01_extract/extract.go)。

## 第二步：明确应用哪些覆盖

知道原始资源后，才有条件处理自定义模型或修正。`s02_patch` 建立一份可加工的资源集，并按顺序应用调用者提供的资源包目录：

```sh
go run ./cmd/neomega-assets patch \
  -in work/01-extracted \
  -out work/02-patched
```

没有覆盖包时，它复制原始资源，并加入供渲染器诊断使用的 `unknown`、`errored` 方块及程序生成的纹理。

需要替换某份资源时，准备标准资源包目录：

```text
my-pack/
└─ assets/
   └─ minecraft/
      ├─ models/block/stone.json
      └─ textures/block/stone.png
```

再指定覆盖包：

```sh
go run ./cmd/neomega-assets patch \
  -in work/01-extracted \
  -out work/02-patched \
  -overlay ./my-pack
```

`-overlay` 可以重复。相同资源 ID 由最后一个覆盖包替换，替换单位是一份完整文件。这个规则让覆盖结果可预测；修改少量字段时，应先准备好完整 JSON。索引会记录最终文件的来源。

26.3 的原始 JAR 已提供床、告示牌等方块模型。旧版 `mc-assets` 的自定义模型可能引用旧纹理路径，因此应当按当前版本准备覆盖包。

实现入口：[s02_patch/patch.go](s02_patch/patch.go)。

## 第三步：整理模型，并检查引用关系

有了最终资源集，接下来要确认模型之间能否连起来。一个子模型可以继承父模型的几何，再替换纹理变量；方块状态还可能选择多个加权模型，或按多个条件拼接模型。

```sh
go run ./cmd/neomega-assets models \
  -in work/02-patched \
  -out work/03-models
```

这一步会检查实际被方块状态使用的父模型和纹理引用，发现缺失引用或循环时返回带资源名的错误。`models.json` 记录需要纳入图集的纹理，以及仍依赖游戏内置渲染的模型。

最终模型表保留父模型、变体和 multipart 条件。这里的继承解析用于检查关系和收集纹理；渲染器仍然可以根据具体方块状态选择变体，而不是在生成时随机选掉其中一个。

类型集中在 [asset/model.go](asset/model.go)，几个容易混淆的情况有明确表示：

- `Choice` 区分单个模型与一组加权候选，候选的权重和旋转会保留。
- `Condition` 区分属性匹配与 `OR`、`AND` 组合。
- `Model.Elements` 用指针区分“继承父几何”和“显式空数组，清除父几何”。
- `TextureBinding` 区分传统字符串引用与 26.3 的 `sprite` 对象，保留 `force_translucent`。
- 模型顶层尚未解释的扩展字段保存在 `Model.Extra`，供消费者继续处理。

实现入口：[s03_models/models.go](s03_models/models.go)；继承与纹理变量解析在 [s03_models/resolve.go](s03_models/resolve.go)。

## 第四步：把纹理放进图集

模型已经明确引用哪些纹理后，才能生成与它们匹配的坐标。`s04_atlas` 收录方块纹理和模型实际引用的其他纹理，一起生成坐标表与 PNG：

```sh
go run ./cmd/neomega-assets atlas \
  -in work/02-patched \
  -models work/03-models \
  -out dist
```

普通方块纹理缩放到 16×16 的格子。实体类纹理保留原始尺寸。带动画元数据的纹理取 `frames` 指定的首帧；没有显式帧列表时取第 0 帧。PNG 表达的是静态采样结果。

装箱按尺寸和名称排序后分行放置，放不下时扩大图集，输出为正方形的二次幂尺寸。相同输入会得到稳定的布局。坐标 `u/v/su/sv` 都是相对于整个 PNG 的归一化值。

例如，图集宽 1024 像素，某纹理位于横坐标 32、宽 16，则 `u=32/1024`、`su=16/1024`。消费者应同时读取这次生成的坐标表与 PNG。

实现入口：[s04_atlas/atlas.go](s04_atlas/atlas.go)。

## 与 mc-viewer-svelte 如何衔接

文件名、`latest` 层、默认命名空间省略方式和 UV 字段沿用 `mc-assets` 的约定。这三份产物对应 viewer 的 `generate_res/mc-assets` 所需的上游数据。

**最新资源格式与旧 viewer 的完整渲染支持需要分别处理。** 当前 viewer 的方块状态表仍基于 Java `1.20.2`，旧模型解析器也只理解字符串纹理引用。26.3 输出会保留 `sprite` 对象、材质标记、新的旋转字段和 `shade_direction_override` 光照方向。要让 viewer 完整展示最新版本，还需要同步升级它的方块状态数据和模型消费者；单独替换三份文件不能完成这些工作。

更新消费者后，应整组替换三份资源，再在 viewer 运行 `pnpm generate`。viewer 自己的 `rtids.json` 和渲染数据仍由 viewer 的生成流程负责。

## 在 Go 中调用与扩展

CLI 只是几个包的组合。需要在其他工具中处理资源时，可以从某一步开始调用，而不用执行整个构建：

```go
report, err := s03_models.Run(s03_models.Options{
    InputDir:  "work/02-patched",
    OutputDir: "work/03-models",
})
if err != nil {
    return err
}
fmt.Println(report.Version, report.ModelCount)
```

导入路径是 `neomega-assets/s03_models`。其他步骤同样使用各自的 `Options` 和 `Run`。下载与较长的文件处理接收 `context.Context`，便于取消。

添加资源修正时，优先准备覆盖包，让修正作为显式输入。需要理解新的模型字段时，扩展 `asset` 中的对应类型；需要改变纹理采样或布局时，修改 `s04_atlas`。这些职责分别对应已有步骤，便于定位结果变化。

## 示例数据与验证

以下结果由上述四步命令产生，输入为 Mojang 清单中的 `26.3` 客户端 JAR，没有额外覆盖包。第二步加入两个程序生成的诊断方块及对应模型、纹理。

| 检查项 | 数量 |
|---|---:|
| JAR 中的方块状态 JSON | 1,288 |
| JAR 中的模型 JSON | 4,192 |
| JAR 中的纹理 PNG | 3,964 |
| 输出方块状态 | 1,290 |
| 输出模型 | 4,194 |
| 图集中收录的纹理 | 1,302 |
| 图集尺寸 | 1024×1024 |
| 尚需内置渲染的已引用模型 | 0 |

“模型”统计包含 JAR 中的 block/item 模型；图集只收录方块纹理及方块模型实际需要的其他纹理，所以数量不同。

运行验证：

```sh
go test -race ./...
go vet ./...
```

测试现场创建小型 JAR 和 PNG，覆盖从解包到最终表的流程，以及命名空间、覆盖包、继承、空几何、引用循环、26.3 纹理对象、加权模型、逻辑条件、动画首帧和图集坐标。测试不需要网络或本地 Minecraft 安装。

## 来源与许可

流程分步方式参考本工作区的 `my-mc-assets`；模型表与图集格式参考 [zardoy/mc-assets](https://github.com/zardoy/mc-assets)。源码中的相关位置有来源说明，完整说明见 [NOTICE.md](NOTICE.md)，上游 MIT 原文保存在 [third_party/mc-assets/LICENSE](third_party/mc-assets/LICENSE)。

工具源码按 [MIT](LICENSE) 提供。Minecraft JAR、提取的模型和纹理，以及包含它们的生成图集，仍适用原始素材的条款，不能把工具的 MIT 许可证当作素材再发布授权。发布生成结果前，应核对 [Minecraft EULA](https://www.minecraft.net/eula)、[Usage Guidelines](https://www.minecraft.net/usage-guidelines) 和覆盖包作者提供的条款。输出目录会附带 `THIRD_PARTY_NOTICES.txt`，但它不替代素材本身的授权。
