# 单方块渲染示例

这个示例一次只显示一个方块。四个场景分别是含水树叶、动画火焰、附魔台和纹样陶罐。页面只有场景链接和可拖动旋转的画布，Go 负责生成渲染结果，网页负责解码和绘制。

普通方块的两种变化分别由含水树叶和火焰展示。含水树叶拆成树叶和水两层几何。火焰的几何保持固定，由网页随时间切换纹理帧。附魔台展示 effect，即在指定位置创建动态书本的消息。陶罐展示 block actor，即读取方块实体数据来生成外观的处理器；四面的纹样由 NBT（Minecraft 的结构化数据格式）决定。

## 单方块怎样使用完整渲染能力

`dispatcher.Pipeline` 是连接网格、特效和实体外观处理器的统一入口。
示例创建它时传入 `cube.New(1)`，确定每次只处理 1×1×1 个格子。
`blockdata.Uniform(layout.Len(), state)` 创建只有一项调色板、一个索引的输入，输入不携带尺寸，也不填充空气子区块。
这里的 `Section` 只是约定尺寸的区域，名称不要求 16³。
相同入口使用 `cube.New(16)` 时可以处理世界子区块，含水、特效和陶罐规则保持相同。

方块放在局部原点 `(0,0,0)`，外部邻居按空气处理。实体 NBT 也以这个局部坐标关联到方块；只有陶罐场景提供它。
尺寸约定的是格子数量，模型和书本动画可以超出单位格子，相机会根据实际范围取景。

## 从请求到绘制

为了指定要显示的方块，[main.go](main.go) 提供四组完整 Java 方块状态，以及陶罐的 NBT。请求形式是 `GET /api/block?scene=fire`。可选名称为 `waterlogged_leaves`、`fire`、`enchanting_table` 和 `decorated_pot`。

handler 构造只有一格的完整状态输入，再调用 `Pipeline.Build`。分发器处理含水层的相互遮挡和缩进，为附魔台生成书本消息，并根据陶罐 NBT 替换四面纹样。结果用 `Frame.Marshal` 编码成 NVFR，即包含静态网格、effect 和 actor 网格的完整快照格式。几何表与贴图均来自核心内嵌资源。

为了显示这个快照，[main.ts](main.ts) 使用 `decodeFrame` 解码，通过 `createFrameObject` 创建绘制对象。书本特效由 `createEffectRegistry` 注册。每帧先把回调的毫秒换成秒，用 `frame.updateTime` 更新时间，再绘制整个场景，水、火焰和书本因而共享同一个时钟。切换场景直接导航到另一条页面链接。

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
