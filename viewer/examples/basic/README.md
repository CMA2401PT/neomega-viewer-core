# 最小子区块示例

浏览器请求 `GET /api/subchunk?subchunk=default`。Go 在请求内构造一个 16×16×16 的子区块，内容是石头平台上的木屋，带门洞、玻璃窗和阶梯形屋顶。`Mesher.MeshLayer` 根据内嵌几何表生成网格，默认剔除遮挡面并合并相邻面，再用 `Mesh.Marshal` 编码为 NVMS（viewer 的网格二进制格式）返回。

网页通过共享绘制包的 `decodeMesh` 解码，用 `createMeshObject` 把网格放入 Three.js 场景。方块贴图来自与几何表配套的内嵌图集。画面只在相机改变或窗口缩放时重绘。

这个入口直接接收一层状态 ID，适合本例中的普通方块。需要自动拆分含水层、生成附魔书或读取陶罐 NBT 时，使用 [single_block](../single_block/README.md) 展示的完整渲染入口。

## 阅读代码

从 [main.go](main.go) 的 HTTP handler 开始：`defaultSubchunk` 生成 4096 个 Java 方块状态 ID，随后调用网格生成与编码。ID 是规范化状态的 `Hash()`；本例中的状态没有属性，已经是规范形式。构建器通过 `cube.New(16)` 确定尺寸，数组按 `layout.Index(x,y,z)` 定位格子，未放方块的位置填空气 ID，不能以数值 0 代替。尺寸在构建器创建时约定，不写进每份方块数据或网格响应。

接着读 [main.ts](main.ts)：加载图集、请求网格、解码、创建绘制对象，最后设置相机和鼠标控制。网页的 `await` 用于等待网络结果；Go handler 内的构造、网格生成与编码按顺序执行。

## 运行

在工作区根目录执行以下命令。前端只需在源码变化后重新构建，运行时由 Go 同时提供网页、贴图和网格接口。

```sh
cd neomega-viewer-core/viewer/examples/basic
npm ci
npm run build
go run .
```

打开 <http://127.0.0.1:2413>。拖动鼠标旋转，滚轮缩放，右键拖动平移。需要更换端口时运行 `go run . -listen 127.0.0.1:2414`。

贴图的来源和许可见服务提供的 [THIRD_PARTY_NOTICES.txt](http://127.0.0.1:2413/textures/THIRD_PARTY_NOTICES.txt) 与模块的 [NOTICE.md](../../../NOTICE.md)。
