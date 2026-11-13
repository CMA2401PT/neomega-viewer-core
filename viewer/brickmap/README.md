# brickmap：分层材质体素

这个包用规则网格中的材质编号表示完整立方体，由 WebGL 2 对每个像素沿观察方向遍历空间。
Go 负责准备和序列化数据，浏览器负责正交相机、表面透明合成和绘制。
体素数据既可以一次加载，也可以按区域装入移动窗口；窗口目录与共享砖池分别管理，使多个视图能够共用一份材质和体素显存。

空间分为 8×8×8 的小块。同材质小块只保存一个编号，混合小块每格使用两个字节。
上层目录逐级合并相同材质，射线可以整段跨过空气与均匀介质。
混合小块内部使用逐网格边界的 DDA 遍历，跨界轴由整数坐标决定，避免浮点误差使射线停在原格。

## 准备数据

生成来源可调用 `Build`，按坐标回调提供小块。
流式来源使用 `NewBuilder` 创建目录，调用 `AddBrick` 提交小块，最后调用 `Finish` 构建层次。
同一位置只提交一次。起点按 8 格对齐，每轴尺寸为不小于 8 的二次幂，未提交的位置为空气。
小块数组按 `x + 8*(y + 8*z)` 排列，提交时复制，因此来源可复用缓冲。
已有 16³ section 时，用 `AddSection(origin, cells)` 一次提交八个小块。
它接收按 `x<<8 | z<<4 | y` 排列的 4096 个 uint16 材质编号，由应用负责将来源方块映射成这些编号。

编号 0 表示空气，1…65535 引用共享材质表。
每条材质按 -X、+X、-Y、+Y、-Z、+Z 保存六份非预乘 sRGB 与表面不透明度。
非零 Group 表示可隐藏内部接缝的同一种透明介质。
黑色和零透明度都有独立材质编号，不被转换为空气。

### 从 Java 方块准备材质

Java 方块状态需要先压成六面颜色材质，再分配体素使用的编号。
[`OpenMaterialSource`](materials.go) 加载内嵌几何资源、图集和俯视颜色表，返回的 `MaterialSource` 提取六面颜色、表面透明度与介质组。
[`Palette`](palette.go) 缓存状态对应的编号，并让材质值相同的状态共用编号。

先用 `NewPalette(source)` 创建编号表，对全部方块调用 `ID(block)`。
再把 `Materials()` 返回的材质表传给 `Build` 或 `NewBuilder`，小块中填写相应编号。
逐 section 读取时也可以先用空调色板构建目录，在所有编号分配完成后，将 `Materials()` 赋给返回体积的 `Palette`。
`Unknown()` 返回排序后的未知方块短名，供调用方报告使用回退色的方块。

## 表面透明

同材质或相同非零 Group 的内部接缝不贡献颜色。
在其他边界，先混合旧材质的出口面，再混合新材质的入口面。
射线以从近到远的顺序累计颜色和剩余透光率，遇到不透明表面时结束；离开体积后混合背景。
流式窗口的水平边缘是数据裁切，绘制时使用 `openBoundary: true` 隐去这些截断面。
未加载区域使用独立目录值，射线遇到它便停止并显示斜纹；拾取会返回缺失位置。
`clipY` 是保留范围的上界，表示只显示 `y < clipY` 的格子，切开的实心部分仍显示表面。
颜色混合在线性空间进行，输出转换回 sRGB。

此模型不按穿过距离计算吸收，因此连续透明区域变厚不会因为格子数增加而变浓。
材质分组与六面颜色由调用者提供，渲染器不识别具体方块名称。

## 数据与绘制

`Volume.Write` 输出 `brickmap-2` 格式。
开头为小端序 JSON 头长度，随后为补齐到四字节边界的 JSON。
JSON 记录起点、尺寸、逐层目录尺寸、混合小块字节数与材质数。
后续依次为全部材质的六面 RGBA、全部材质的 uint32 Group、由细到粗的 uint32 目录以及 uint16 小块数据。
全部整数使用小端序，应用可对整个输出压缩。

[`web/volume.ts`](web/volume.ts) 的 `decodeVolume(buffer)` 解析响应，返回的数组借用输入缓冲。
[`web/gpu.ts`](web/gpu.ts) 的 `BrickmapPool` 管理共享砖池和全局材质表。
`pool.upload(volume)` 先按六面 RGBA 和 Group 合并材质，再映射砖内编号并上传，返回一个 `GPURegion`；容量不足时返回 `null`。
返回对象复制并保留区域本地目录，不保留网络响应中的砖数组，因此调用方可以释放响应缓冲。

为了移动数据覆盖范围，`BrickmapWindow` 持有一份独立的三维目录纹理。
流式区域水平尺寸为 128×128，所有区域与窗口共用 Y 原点及补齐后的高度。
窗口水平原点按 128 对齐，水平尺寸为 1024×1024。
`install(region)` 写入区域的 0～4 级目录，重算并上传第 5 级以上的父层。
`rebuild(origin, regions)` 在窗口跳动时用保留区域重新建立全部目录，砖池槽位保持不变。
`remove(region)` 摘除区域及父层引用；同一区域可以安装到多个窗口，全部窗口摘除之后才能调用 `pool.release(region)` 归还槽位。

共享资源和窗口都由宿主在动画帧内更新。
网络回调只准备 CPU 数据，宿主确认区域仍属于当前存档和窗口后，依次完成上传、目录发布和绘制。
以下代码展示已解码的一个区域如何进入窗口；`gl` 是宿主的 WebGL 2 上下文，`minY`、`height` 来自维度描述：

```ts
const pool = new BrickmapPool(gl, 512 * 1024 ** 2);
const window = new BrickmapWindow(gl, [-512, minY, -512], [1024, height, 1024]);
const renderer = new BrickmapRenderer(gl, pool, window);

// 在动画帧中安装完成下载的区域；null 表示需要由宿主显示容量退化状态。
const region = pool.upload(volume);
if (region) window.install(region);
renderer.draw(camera, {
  viewport: [left, bottom, width, heightInPixels],
  openBoundary: true,
  timing: false,
});
threeRenderer.resetState();
```

[`web/renderer.ts`](web/renderer.ts) 的绘制器借用资源与目录，每次提交明确绑定目标、视口、纹理与绘制状态。
视口坐标以实际绘制像素为单位，原点在画布左下角。
共享 three.js 时，在主场景及其遮挡查询完成之后绘制小地图，再调用 `resetState()` 交还上下文。
主场景每帧清屏，因此共享画布上的小地图也每帧绘制。

点击定位使用 `renderer.pick({ origin, direction }, options)`，返回 `hit`、`missing` 或 `miss`。
命中包含世界方块坐标和面编号，透明表面取第一处可见表面；缺失包含需要加载的位置。
面编号与材质面顺序相同。
`cameraRay(camera, width, height, x, y)` 将视口内的屏幕位置转成正交射线，屏幕 Y 从上往下。
它的起点在相机目标平面，向下拾取整个体积时，宿主先沿同一直线将起点移到体积上方。
头顶探测可以直接从相机所在位置向上拾取。

[`web/shaders.ts`](web/shaders.ts) 将射线生成、层级遍历和像素输出分别组织。
遍历把材质边界交给输出部分；颜色绘制和整数拾取共用同一套介质及跨界规则。
颜色输出在线性空间累计，再转换为 sRGB。
相机与深度合成代码可以替换这些部分，而无需把窗口管理混入遍历。

独立离线预览由应用创建 WebGL 2 上下文，按整份体积的砖字节数创建池，再用 `window.load(region)` 安装完整目录。
此时窗口尺寸等于体积尺寸，并使用默认的封闭体积边界。
释放时先调用绘制器和窗口的 `dispose()`，再释放共享池；绘制器自身只拥有程序、拾取目标和计时查询。

每个像素采用一条射线，因此细碎结构可能出现采样闪烁。
遍历次数超出上限时，颜色输出紫色，拾取抛出错误。
应用与离线导出命令见 [neomega-full-view](../../../apps/neomega-full-view/README.md)。
