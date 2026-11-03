# 附魔台、陶罐与旗帜示例

这个示例在石头平台的木屋外放置三个附魔台、三个纹样陶罐和两面旗帜。浏览器请求 `GET /api/subchunk?subchunk=default`，Go 在请求内构造方块与方块实体数据，生成完整的子区块渲染快照，再交给网页绘制。

附魔台由静态台体和 effect 组成。effect 是一条带位置和特效名的消息，网页据此创建动态书本。陶罐通过 block actor 生成几何；block actor 是读取方块实体数据来决定外观的处理器。这里用 NBT（Minecraft 的结构化数据格式）记录四面的陶片纹样。三个陶罐分别朝 north、east、west，可以旋转相机观察四面的纹样。

## 数据怎样传到画面

为了把三种内容一起传给网页，[main.go](main.go) 使用 `javapreset.NewPipeline` 创建分发器。它将普通方块交给 mesher 剔面和合并，为附魔台生成书本消息，并按陶罐的 NBT 替换侧面材质。Java 预设只组装处理器；陶罐和旗帜从各自包内加载固定图片，再随网格交付像素。

构建器使用 `cube.New(16)` 固定尺寸，方块输入采用不带尺寸的 `blockdata.Blocks`。其中每个索引对应一格，通过 `layout.Index(x,y,z)` 定位。`Section` 表示这个约定尺寸的区域，并不要求固定为 16³。

输入是 `dispatcher.Section`：完整 Java 方块状态的调色板，以及按格内坐标关联的实体 NBT。陶罐的 `sherds` 使用四项列表，顺序是 back、left、right、front，分别为垂钓、弓箭、利刃和举臂纹样。旗帜的 `patterns` 按列表顺序在 Go 合成，结果作为 actor 私有图片传输；两面旗帜使用同名图片，但底色互不影响。陶罐的格式与朝向说明见 [陶罐 NBT](../../blockactor/decoratedpot/NBT.md)。

`Pipeline.Build` 生成静态网格、effect 消息和 actor 网格。本例不传邻居，区域外按空气处理；空选项默认启用剔面和相邻面合并。`Frame.Marshal` 将它们编码成 NVFR（viewer 的完整快照二进制格式）。处理器诊断会写到 Go 日志，便于发现陶罐处理失败等问题。

[main.ts](main.ts) 加载方块图集和随前端包交付的书本贴图，用 `decodeFrame` 解码响应。`createEffectRegistry` 注册书本特效，`createFrameObject` 将三部分放进同一个绘制对象。每帧把毫秒换成秒，调用 `frame.updateTime` 后绘制，让书本持续起伏、旋转和翻页。

## 运行

在工作区根目录执行：

```sh
cd neomega-viewer-core/viewer/examples/spec_block
npm ci
npm run build
go run .
```

打开 <http://127.0.0.1:2414>。附魔台沿屋前排列，陶罐沿屋子右侧排列。拖动旋转，滚轮缩放，右键拖动平移。需要其他端口时运行 `go run . -listen 127.0.0.1:2415`。前端源码变化后需重新运行 `npm run build`。

贴图来源与许可见服务提供的 [THIRD_PARTY_NOTICES.txt](http://127.0.0.1:2414/textures/THIRD_PARTY_NOTICES.txt) 和模块的 [NOTICE.md](../../../NOTICE.md)。
