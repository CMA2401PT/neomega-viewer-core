# 旗帜的状态、图案与贴图

旗帜处理器在 Go 中按底色和 NBT 图案合成旗面，复用几何表里的旗杆、横梁、旗面及其朝向。
网页接收普通 actor 网格和私有 RGBA 图片，不需要认识旗帜，也不需要额外请求图片。
当前使用固定姿态，展示立式和挂墙旗帜的图案；旗面飘动没有实现。

## 输入从哪里来

底色来自方块名，例如 `minecraft:white_banner` 或 `minecraft:yellow_wall_banner`。
立式旗帜由 `rotation="0"` 到 `"15"` 选择方向，挂墙旗帜由 `facing` 选择 north、east、south、west。
这些朝向已经烘焙在配套几何表中，处理器只替换旗面材质，保留模型 UV。

现代 Java 的实体 NBT 使用 `patterns`，列表顺序就是从底到顶的绘制顺序：

```snbt
{patterns:[
  {pattern:"minecraft:stripe_bottom",color:"blue"},
  {pattern:"minecraft:cross",color:"red"},
  {pattern:"minecraft:border",color:"black"}
]}
```

Java 1.20.5 将旧字段 `Patterns[].Pattern`、`Patterns[].Color` 改为上述形式，见 [Mojang 版本说明](https://feedback.minecraft.net/hc/en-us/articles/26136167989005-Minecraft-Java-Edition-1-20-5-Armored-Paws)。
为了读取旧 Java 存档，处理器也接受短图案码和 Int 染料编号：

```snbt
{Patterns:[{Pattern:"bs",Color:11},{Pattern:"cr",Color:14},{Pattern:"bo",Color:15}]}
```

这里的编号属于 Java：0 为白、15 为黑，与 Bedrock 的编号相反。
Bedrock 数据应先由转换器转成 Java 状态和 NBT；旗帜处理器不在渲染时猜测源版本。
当前 Bedrock 转换器仅转换旗帜底色，尚未保留图案，因此本示例直接提供 Java NBT。
没有图案列表时仅绘制底色。
未知图案、染料或错误字段类型会返回错误，上层记录 `ActorError` 并保留该格的静态旗帜，其余方块继续构建。

## 图片怎样准备

共享图集适合静态材质，但经过缩放的图案不适合作为合成源。
原始 PNG 保存在本包的 `textures/`，随 Go 程序内嵌，不由资源生成器复制。
`banner.New(tables)` 在初始化时解码这些图片，供后续构建共享。
每次 Build 以 `minecraft:base` 为底图，用 Java 染料调制图案的 RGB，保留布料的明暗，再按 alpha 顺序叠加。

内置 Java 图案的资源 ID 与图片文件同名。
处理器使用随代码维护的固定图片，不加载外部数据包或 `asset_id` 注册表。
图片需要与底图尺寸相同。

合成结果使用当前 actor 内的 `banner:face` 名称，旗面的 Region 为整张图片。
旗杆与横梁使用本包的 `banner_base.png`，同样随网格交付像素。
每面旗帜的全部图案合成在一张图片中，不按图案数量增加几何层，避免共面闪烁。
像素格式、归属和释放约定见 [blockactor 接口说明](../README.md)。

## 查看与验证

在工作区根目录执行：

```sh
cd neomega-viewer-core/viewer/examples/single_block
npm ci
npm run build
go run .
```

打开 <http://127.0.0.1:2418/?scene=banner> 查看立式旗帜，或切换到 `?scene=wall_banner` 查看挂墙旗帜。
[spec_block](../../examples/spec_block/README.md) 同时放置两面不同底色的旗帜、附魔台和陶罐，可以检查多个 actor 的贴图隔离。

在核心模块目录运行 `go test ./viewer/blockactor/banner ./viewer/javapreset ./viewer/merger`，检查图案混合、Java 新旧 NBT、方向、失败降级与快照往返。
在 `viewer/web` 运行 `npm test`，检查 Go 生成快照的解码、贴图绑定和卸载释放。
跨语言样本由 `go run ./viewer/web/tests/fixtures/generate_banner.go` 在核心模块目录生成。
