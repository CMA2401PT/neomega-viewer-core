# 单方块渲染示例

这个示例一次只显示一个方块。场景包括含水树叶、动画火焰、附魔台、纹样陶罐、立式旗帜、挂墙旗帜与四种告示牌。页面只有场景链接和可拖动旋转的画布，Go 负责生成渲染结果，网页负责解码和绘制。

普通方块的两种变化分别由含水树叶和火焰展示。含水树叶拆成树叶和水两层几何。火焰的几何保持固定，由网页随时间切换纹理帧。附魔台展示 effect，即在指定位置创建动态书本的消息。陶罐展示 block actor，即读取方块实体数据来生成外观的处理器；四面的纹样由 NBT（Minecraft 的结构化数据格式）决定。旗帜也是 block actor，但需要先在 Go 按图案次序合成旗面，随网格发送私有贴图。立式和挂墙场景使用固定姿态。告示牌同样由 Go 按 NBT 排版文字，支持双面内容、格式码和发光描边；四种形态使用同一处理器。

## 单方块怎样使用完整渲染能力

`dispatcher.Pipeline` 是连接网格、特效和实体外观处理器的统一入口。
示例创建它时传入 `cube.New(1)`，确定每次只处理 1×1×1 个格子。
`blockdata.Uniform(layout.Len(), state)` 创建只有一项调色板、一个索引的输入，输入不携带尺寸，也不填充空气子区块。
这里的 `Section` 只是约定尺寸的区域，名称不要求 16³。
相同入口使用 `cube.New(16)` 时可以处理世界子区块，含水、特效和陶罐规则保持相同。

方块放在局部原点 `(0,0,0)`，外部邻居按空气处理。实体 NBT 也以这个局部坐标关联到方块；陶罐、旗帜和告示牌场景提供它。
尺寸约定的是格子数量，模型和书本动画可以超出单位格子，相机会根据实际范围取景。

## 从请求到绘制

为了指定要显示的方块，[main.go](main.go) 提供各场景的完整 Java 方块状态，以及纹样和文字的 NBT。请求形式是 `GET /api/block?scene=fire`。可选名称为 `waterlogged_leaves`、`fire`、`enchanting_table`、`decorated_pot`、`banner`、`wall_banner`、`sign`、`wall_sign`、`hanging_sign` 和 `wall_hanging_sign`。

`javapreset.NewPipeline` 在启动时组装携带各自贴图的具体处理器，通用分发器只负责调用它们。handler 构造只有一格的完整状态输入，再调用 `Pipeline.Build`。分发器处理含水层的相互遮挡和缩进，为附魔台生成书本消息，并根据陶罐 NBT 替换四面纹样。结果用 `Frame.Marshal` 编码成 NVFR，即包含静态网格、effect 和 actor 网格的完整快照格式。几何表与公共图集来自 `resources`；陶罐、旗帜、告示牌和附魔书的图片各自随处理器代码交付。

为了显示这个快照，[main.ts](main.ts) 使用 `decodeFrame` 解码，通过 `createFrameObject` 创建绘制对象。书本特效由 `createEffectRegistry` 注册。每帧先把回调的毫秒换成秒，用 `frame.updateTime` 更新时间，再绘制整个场景，水、火焰和书本因而共享同一个时钟。纹样与文字的私有贴图由同一个 `createFrameObject` 绑定，无需增加告示牌专用前端逻辑。切换场景直接导航到另一条页面链接。

`sign` 正面展示普通文字，背面展示蓝色发光文字，拖动即可观察两面。
`wall_sign` 展示直接 NBT 文本组件，`hanging_sign` 展示黑色文字的浅色发光描边，`wall_hanging_sign` 展示 `§` 格式码与固定省略号。
字体来源、排版规则和近似范围见 [告示牌说明](../../blockactor/sign/NBT.md)。

## 运行

在工作区根目录执行：

```sh
cd neomega-viewer-core/viewer/examples/single_block
npm ci
npm run build
go run .
```

打开 <http://127.0.0.1:2418>，默认显示含水树叶。顶部链接切换场景。拖动旋转，滚轮缩放，右键拖动平移。前端源码变化后重新运行 `npm run build`。需要其他端口时运行 `go run . -listen 127.0.0.1:2419`。

贴图来源与许可见服务提供的 [THIRD_PARTY_NOTICES.txt](http://127.0.0.1:2418/textures/THIRD_PARTY_NOTICES.txt) 和核心模块的 [NOTICE.md](../../../NOTICE.md)。
