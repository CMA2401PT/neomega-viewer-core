# 生成配套几何、实心表、颜色表与贴图

建表器从同一次 Java 资源构建的模型、图集和渲染说明生成方块几何、普通静态物品目录、元数据、六面实心表及俯视图颜色表。
几何与贴图由渲染器使用，实心表由 fixer 判断邻居边界覆盖。
共同生成可以保证它们使用相同的状态集合、模型选择和材质规则。

## 输入与职责

`neomega-blocks/presets/java/v1` 提供 Java 目标状态全集和历史状态兼容映射。
目标版本由该包的 `JavaTargetVersion` 定义（当前为 26.3），资源构建与几何建表共用这一来源。
`viewer/geometry/java_block_model` 解析模型并烘焙局部形状，`viewer/geometry` 计算覆盖并保存共享形状。
普通物品由 `viewer/item_render` 编译，应用继承后的 `fixed` 展示变换；generated 模型按纹理透明轮廓挤出侧壁。
`internal/tablegen` 组合这些能力，`internal/resources` 补充固定染色、流体预览和透明材质策略。

默认输入是 `viewer-assets/dist/final` 中的 `blockstates.json`、`models.json`、`blocksAtlases.json`、`blocksAtlasLatest.png`、`blockRenderers.json`、`items.json` 和 `build-info.json`。
状态版本与资源版本必须一致。
贴图交付只复制该目录的公共图集和 `THIRD_PARTY_NOTICES.txt`。图集坐标已经烘焙进几何表，无需另外交付坐标 JSON。
旗帜、陶罐和附魔书的固定图片随各自处理器源码维护，不属于此生成器的输出。

模型选择固定使用每组首个候选，满足条件的 multipart 部件全部保留。
同一状态的部件先合并，再判断各面是否被不透明矩形完整覆盖。
这是渲染边界的覆盖含义，不等同于游戏的碰撞或红石导电规则。

## 生成

在 `neomega-viewer-core/viewer-assets` 准备资源：

```sh
go run ./cmd/neomega-assets build
```

`build -version` 默认取 `JavaTargetVersion`，命令会下载对应客户端。已有本地 JAR 时，可用 `build -jar /path/to/client.jar -work dist/works -out dist/final` 准备同样的输入。
固定图片随处理器维护，取自同一目标客户端的对应资源路径。
然后在 `neomega-viewer-core` 模块根目录执行：

```sh
go generate ./viewer/resources
```

默认路径以生成器源码位置为准，无需手动复制输入。
输出分别是 `viewer/resources/geometry.nvtables`、同名 `.java.json` 元数据和 `.items.json.gz` 物品目录、`fixer/solidtable/preset/geometry.nvsolid`、`viewer/topview/colors.nvcolor`，以及 `viewer/resources/textures` 下的贴图与来源声明。
颜色表让俯视图按状态哈希取色，渲染时不再采样贴图。
这些产物随源码提交，使普通构建不需要 Minecraft 安装。

需要使用其他资源或输出位置时，在模块根目录执行：

```sh
go run ./viewer-assets/cmd/build-tables \
  -assets /path/to/dist/final \
  -output /path/to/geometry.nvtables \
  -solid /path/to/geometry.nvsolid \
  -colors /path/to/colors.nvcolor \
  -textures /path/to/textures
```

几何表、实心表与颜色表的目标父目录需要预先存在。
资源生产流程与来源声明见 [viewer-assets](../../README.md)。

## 验证

日常测试使用内嵌表，无需生成输入。
需要核对真实输入与提交的产物时，在模块根目录运行：

```sh
NEOMEGA_ASSETS_DIR="$PWD/viewer-assets/dist/final" \
  go test -count=1 -v ./viewer-assets/cmd/build-tables/internal/tablegen
```

这项检查重新建表，比较状态索引、四边形、覆盖、流体满高度映射、元数据、物品目录、实心表和颜色表，并验证编码、磁盘加载与嵌入加载一致。
