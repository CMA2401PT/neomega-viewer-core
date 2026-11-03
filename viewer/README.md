# Java 渲染核心

渲染器把完整 Java 方块状态、方块实体 NBT 和普通实体转成浏览器可绘制的几何与特效。
Go 负责网格构建及二进制编码，Web 包负责解码、材质、场景与动画。
方块实体 NBT 是附着于方块的结构化数据，例如陶罐四面的纹样。

`geometry` 表达单个方块形状并解析 Java 模型，`resources` 分发预置几何和配套图集。
公共几何与图集的生成由同模块的 `viewer-assets` 负责；特殊处理器的固定图片与其代码放在一起。
普通静态物品由 [item_render](item_render/README.md) 按真实物品定义编译，展示框由 `entity_render` 组合框体与物品。
`mesher` 按邻居覆盖剔面和合并，`dispatcher` 将普通方块、特效和依赖 NBT 的外观分别处理，再由 `merger` 编码成完整快照。
渲染入口使用已经确定的 Java 状态，是否先调用 fixer 由外层应用决定。

## 最简示例

从 [single_block](examples/single_block/README.md) 可以分别查看含水树叶、动画火焰、附魔台、纹样陶罐、旗帜与双面告示牌，每个场景只显示一个方块。
需要多个方块时，[basic](examples/basic/README.md) 展示静态子区块，[spec_block](examples/spec_block/README.md) 展示附魔书、陶罐与旗帜，[infinity](examples/infinity/README.md) 展示按需生成的无限世界。
它们直接连接 Go 与 Web 核心，HTTP 接口由各示例自行提供。
存档浏览、来源缓存和复杂演示见 [neomega-viewer](../../apps/neomega-viewer/README.md)。

## 直接使用 Go 库

几何使用者先加载配套表，再创建 Mesher，把状态 ID 转成网格。可直接运行的 [最小 Go 示例](../examples/mesh/main.go) 使用内嵌表生成石头方块，并将结果编码成 NVMS；在本模块根目录运行：

```sh
go run ./examples/mesh
```

示例中的 `resources.Open` 取得内嵌表，`MeshBlock` 返回独立网格，`Mesh.Marshal` 把网格写入任意 `io.Writer`。文件、内存缓冲和 HTTP 响应都使用同一个编码入口。

自己的数值 ID 系统可以直接构建 `geometry.Tables[B]{Index, Shapes, FullHeight}`，用 `tables.Encode(writer)` 保存。`ModelIDs(id)` 和 `Shape(modelID)` 返回共享只读数据；`FullHeight` 把原模型编号映射到满高度编号。`Shape`、`ModelID` 和 `Tables` 都由 `neomega-viewer-core/viewer/geometry` 定义。自定义 64 位 ID 可写成 `type BlockID uint64`，并使用 `Tables[BlockID]`、`DecodeTables[BlockID]` 和 `[]BlockID`；ModelID 始终是 uint32 的 Shape 下标。

多个方块使用一个固定尺寸的 Mesher。`cube.New(N)` 创建只读的立方体布局，构建器及其所有邻居遵守同一个 N³ 约定。
尺寸不放进每份方块数据，也不随每次构建重复传递。
布局使用 `(x*N+z)*N+y` 顺序，即 Y 最快、Z 次之、X 最慢；用 `layout.Index(x,y,z)` 定位格子。
局部格坐标沿用 NVFR 的 uint8，当前支持边长 1…256。

下面构建一个完整石头子区块；将布局改为 `cube.New(1)`，同一流程就只处理一格：

```go
layout := cube.New(16)
renderer := mesher.NewMesher(layout, tables)
blocks := make([]uint32, layout.Len())
for i := range blocks { blocks[i] = stoneID }
result, err := renderer.MeshLayer(blocks, mesher.Options{})
if err != nil { return err }
if err := result.Mesh.Marshal(writer); err != nil { return err }
```

`MeshLayer` 接收一层 N³ 个单层状态 ID，`MeshLayers` 接收多层，二者共用邻域构建流程。
输入长度必须符合构建器约定，尺寸不匹配返回错误。
固定 16³ 的 `celler/layer_storage.Palette` 属于世界存储，渲染 core 不依赖它。
已有这种存储时，应用负责展开逻辑数据；完整状态的适配见应用层的 [source.BlocksFromPalette](../../apps/neomega-viewer/source/palette.go)。

几何传输入口在 `neomega-viewer-core/viewer/mesher/geometry`。`Mesh.Marshal(w io.Writer)` 写出 NVMS，`geometry.UnmarshalMesh(r io.Reader)` 读回 Mesh；文件、HTTP 响应和内存缓冲使用同一接口。`geometry.MarshalQuad(w, quad)` 和 `geometry.UnmarshalQuad(r)` 用于只有一个四边形的 NVMS。`Mesh.Buffers()` 保留为扁平 GPU 属性入口，单方块 JSON 和直接绘制的 Go 调用者仍使用它。

NVMS 保存 float32 绘制数据，按材质分批后的顺序恢复四边形。它不保留 CullFace，也不保留“零 ShadeNormal 表示使用 Normal”的原始标记；需要保留烘焙精度和剔面信息时，用 NVGT 保存 Tables。

构建期间调用者应保持输入切片和 Tables 不变。同一个 Mesher 可以并发处理不同输入，输出网格的修改不会改变共享表。Mesh 方法只返回几何和统计，Java 状态由调用者从 `neomega-blocks/presets/java/v1.HashToBlock` 查询。

演示程序用 `neomega-viewer-core/viewer/resources.Open` 加载内嵌表，或通过 `OpenOptions{TablesPath: "/path/to/geometry.nvtables"}` 加载文件及配套的 `.java.json`、`.items.json.gz` 旁文件。
需要同时返回 Java 状态和渲染类别时，应用层的 [singleblock](../../apps/neomega-viewer/demo/singleblock/engine.go) 在核心网格之外补充这些元数据。
应用层的 [HTTP 接口](../../apps/neomega-viewer/demo/httpapi/handler.go) 将状态展开为网页使用的名称和属性对象。

## 渲染含水方块：拆层后多层渲染

几何表的键是不带 `waterlogged` 的单层状态，称为 JavaBlockHashRSL（Render Single Layer），一个 RSL 对应一组模型。完整 Java 状态的 hash 可能带 `waterlogged="true"`，它需要台阶和水两组模型，不能直接查表。

`neomega-viewer-core/viewer/splitlayer` 负责把完整状态拆成若干 RSL，下标即层号。规则只看状态：有 `waterlogged` 就去掉它放进第 0 层，值为 `true` 时第 1 层放一格 `minecraft:water ["level"="0"]`；seagrass、tall_seagrass、kelp、kelp_plant 总在水里，同样补一层水；其余方块原样放第 0 层。

- `splitlayer.SplitBlock(block ir.IRBlock)` 直接按规则拆一个状态，适合手里是 blockstate 的调用方。
- `splitlayer.New()` 把规则对 `presets/java/v1.JavaCompatibleStates()` 的目标及兼容状态展开成 hash 表，多个实例共享。`SplitHash(hash)` 拆一个 hash；`SplitBlocks(blocks)` 按输入顺序拆层，返回 `[][]uint32`，层数取各格最大值，空位填空气。表里没有的 hash 原样当作 RSL。

拆层只能做一次：kelp 的 RSL 再拆一次会多出一层水。输入若已丢了 `waterlogged`，拆层也无从得知它原本含水。

`Mesher.MeshLayers(layerBlocks [][]B, options)` 接收多层，每层 N³ 个 RSL，索引与 `MeshLayer` 相同。`MeshLayer` 就是只有一层的特例。接续前面的 `blocks` 和 `renderer`：

```go
splitter := splitlayer.New()
layers := splitter.SplitBlocks(blocks)
result, err := renderer.MeshLayers(layers, mesher.Options{})
if err != nil {
    return err
}
if err := result.Mesh.Marshal(writer); err != nil {
    return err
}
```

演示服务的子区块接口就按这个顺序处理输入。为什么按位置合并各层、层序在同格里意味着什么、镂空前景下水面为何缩进，见 [多层渲染方案.md](多层渲染方案.md)。

## 渲染带 NBT 的方块：分发与快照

这里的 `Section` 表示构建器约定的一个 N³ 区域，数据自身不携带尺寸。需要同时生成静态几何、特效和 NBT 外观时，使用 `dispatcher.Pipeline`。它接收完整 Java 状态调色板及按格内坐标组织的 `nbt.Compound`，并输出一个 NVFR 快照：

- **mesher 段**：普通方块、尚未支持的 NBT 方块和 actor 生成失败时保留的静态几何。
- **effector 段**：特效消息，例如 `{pos: [3,1,3], effect: "enchanting_table_book"}`。本体照常在 mesher 段里，特效由前端程序驱动。
- **block actor 段**：按 NBT 生成的几何，坐标已是子区块局部坐标。成功构建的 actor 替换该格的静态本体。

Java 的默认组合由 `javapreset.NewPipeline` 提供，启用附魔书、陶罐、旗帜、告示牌和静态展示框。各方块处理器在构造函数中读取内嵌资源，展示框使用与方块表配套的物品目录。方块处理器由调用方组合，普通实体交给 `entity_render`。`blocks` 是 `*blockdata.Blocks`，包含状态调色板和 N³ 个未压缩索引，实体来自已解码的 Java NBT：

```go
layout := cube.New(1)
blocks := blockdata.Uniform(layout.Len(), ir.IRBlock("minecraft:enchanting_table []"))
resources, err := preset.Open(preset.OpenOptions{})
if err != nil { return err }
pipeline, err := javapreset.NewPipeline(layout, resources.Tables, resources.Items, nil)
if err != nil { return err }
result, err := pipeline.Build(
    dispatcher.Section{Blocks: blocks},
    dispatcher.Neighbours{},
    dispatcher.Options{},
)
if err != nil {
    return err
}
if err := result.Frame.Marshal(writer); err != nil {
    return err
}
```

`Build` 按调色板项调用 `splitlayer.SplitBlock`，再展开到 N³ 格，因此预置状态集以外的含水状态也能拆开。路由来自已启用的 handler 和 effect 的方块名。actor 逐格构建，成功才移除本体层，水层保留；失败时保留静态本体并产生 `ActorError` 诊断，其他格继续构建。诊断携带拆层前的完整状态和格内坐标。NVFR 的字节布局见 [merger/FORMAT.md](merger/FORMAT.md)。

方块实体放在 `Section.BlockEntities`，按格内整数坐标索引。普通实体放在可选的 `Section.Entities`，包含局部浮点位置和原始 NBT。它的 `List` 决定本次输出归属；可选的 `Query` 只提供同一局部坐标系内的候选，允许多返回，不能增加输出实体。展示框不需要 Query。`Blocks == nil` 的区域仍然可以输出实体。

展示框几何追加到 `Frame.Mesh`，不占用 block actor 通道，也不改变 NVFR。物品失败时保留框体，错误通过独立的 `Result.EntityDiagnostics` 返回。普通物品与 `fixed` 变换的边界见 [item_render](item_render/README.md)。

中心、六向邻居及批量来源都必须遵守同一构建器的尺寸。需要跨 section 剔面时，`Neighbours` 按西、东、下、上、北、南传入邻居，nil 代表空空间。邻居只提供拆层后的覆盖信息；actor 候选格的本体不参与遮挡，水层仍提供覆盖。这种处理可能保留额外的面，避免用静态本体误遮挡 actor 周围的可见表面。

批量构建使用 `BuildSections(ctx, source, positions, options)`。调用方实现 `SectionSource`，按区域网格坐标提供数据，相邻坐标相差一个 N³ 区域。Pipeline 在一次调用内缓存请求位置与六向邻居的读取和拆层结果，按请求顺序返回各位置的快照，不存在的位置返回空快照。存档来源的解码与实体分组由外层应用完成。

世界存储的 hash 输入由应用层 `source.PaletteFromHashes` 反查成完整状态，再由 `source.BlocksFromPalette` 接入渲染；未知 hash 返回错误。
应用层的 [frame 接口](../../apps/neomega-viewer/demo/httpapi/frame.go) 使用这条路径，场景配方中的 SNBT（NBT 的文本表示）由应用层解析成 Compound。
纯 mesh 接口仍按前文的多层入口构建。

需要缺失模型占位时，将独立的 `*blockgeometry.Shape` 传给 `javapreset.NewPipeline`。mesher 只在几何表没有对应键时引用它，有键但模型列表为空的状态仍视为有意无几何。中心与邻居使用相同的查表规则，只有中心格产生缺失记录。Pipeline 将多层缺失合并为每格一条 `MissingModel` 诊断。占位为 nil 时缺失 ID 返回错误。[存档浏览器](../../apps/neomega-viewer/cmd/world-viewer/mesh.go) 使用石头形状的洋红副本作为占位，最简示例使用 nil。

新增一种 NBT 方块时，在 `blockactor/` 下新建子包，实现 `blockactor.Handler`，按完整状态和 Compound 返回 `Appearance`，即格内网格与可选私有 RGBA 图片。`blockactor.Dispatcher` 统一复制和平移网格，`javapreset` 负责注册具体处理器。陶罐、旗帜和告示牌自行持有固定资源，并随网格交付所需像素；旗面和文字先在 Go 合成。接口边界与具体接入方式见 [blockactor 说明](blockactor/README.md)。

NVFR v2 携带 actor 的私有图片，Go/TS 解码器仍可读取不含图片的 v1。升级服务时应同时更新前端绘制包。旗帜 NBT、支持范围和示例见 [旗帜说明](blockactor/banner/NBT.md)，双面文字和发光规则见 [告示牌说明](blockactor/sign/NBT.md)。

## 更新配套几何资源

模型、图集或静态材质策略改变后，在 viewer-core 模块根目录运行：

```sh
go generate ./viewer/resources
```

入口直接读取 `viewer-assets/dist/final`，同时更新方块几何、物品目录、元数据、实心表与公共图集。
准备资源与验证建表的命令见 [生成器说明](../viewer-assets/cmd/build-tables/README.md)。
几何中的 UV 与图集布局配套，更新后应重新构建使用这些资源的程序。

## 一个方块怎样变成几何

单个状态如何选择模型、解析父模型、烘焙旋转和材质，由 [neomega-viewer-core/viewer/geometry](geometry/README.md) 负责。模型部件按完整四边形共享，多个状态可以引用同一根栅栏柱或同一段栏杆。viewer 读取这些已经固定的局部形状，再按位置与邻居生成场景网格。

运行时通过共享查表函数查询 `Tables.Index`，再用 `Tables.Shape(modelID)` 取得部件。单方块将部件四边形复制到独立网格，子区块保留共享引用供邻域剔面使用。

几何使用一个方块为一单位的局部坐标，Y 向上，北向为 −Z。四边形保存几何法线、UV、颜色和材质。模型要求覆盖明暗方向时，另外保存光照法线，真实几何法线仍保持原义。`CullFace` 保存模型允许被邻居遮挡的方向；单方块入口保留这些面，子区块入口检查其实际覆盖情况。

## 形状、透明度与静态外观

半砖的半高来自模型几何，草的外形来自旋转的交叉平面。几何形状和贴图透明度需要分别处理：

| 表面类型 | 示例 | 绘制方式 |
|---|---|---|
| 不透明 `opaque` | 石头、半砖、栅栏 | 写入深度，同材质面合批 |
| 镂空 `cutout` | 树叶、草 | 丢弃透明像素，其余像素写入深度 |
| 半透明 `blend` | 玻璃、水 | 混合颜色，不写入深度；默认逐面排序，地图模式按材质批次排序 |

`neomega-viewer-core/viewer/geometry/java_block_model.MaterialLibrary` 根据图集像素分类。[建表材质策略](../viewer-assets/cmd/build-tables/internal/resources/preview.go) 补充玻璃等语义规则，以及草木、水和红石颜色。红石颜色在建表时按 `power` 烘入 `Quad.Color`。形状去重比较完整四边形，包括颜色。有厚度的镂空元素使用正面绘制，避免多画盒体的背面。独立薄片可以双面显示；草已有明确前后面时，分别绘制对应一侧，避免重复绘制。前端在每张纹理自己的图集区域内采样；即使一个面合并到跨越多个方块，也会重复纹理。

箱子、床和告示牌等方块的完整外观由游戏代码绘制，JAR 未必包含完整模型。`viewer-assets` 的 `blockRenderers.json` 说明类别并补入社区静态模板，viewer-assets 的建表器通过同一模型接口解析它们。图集 PNG 单独加载，不随每个网格重复传输。

这里的静态指**给定完整状态后，模型姿态固定**。箱子使用固定姿态；纯 mesher 入口中的告示牌展示牌体，启用告示牌处理器的 Pipeline 还会添加文字。水、岩浆、火等纹理由浏览器按图集帧信息循环播放，几何保持不变。库不接收方块实体的实时外观或动画状态。子区块中的同类流体上方还有流体时，补满当前格的液面高度。这不是游戏的流体曲面模拟。

动画纹理的全部帧在图集中竖排，Surface 的 Region 覆盖整条区域。生成器直接搬运图集的 Frames 和 FrameTime；后者以 tick 为单位，每 tick 为 1/20 秒。shader 先按时间选择一帧，再在这一帧内重复局部 UV，并按单帧尺寸内缩半像素以免采到相邻帧。静态纹理使用 Frames=1、FrameTime=0。

状态全集暂不包含 `waterlogged`，这是涉及前景、背景方块表示的历史约定。当模型条件需要它时，建表补为 `false`。网页仅提供状态全集中已有的属性选项。`air` 等不可见类别返回空几何。`moving_piston` 缺少决定移动内容的数据，会返回无法确定静态外形的错误。

水和岩浆的状态包含 `level=0…15`，资源的空 variant 不按 level 选择模型。流体预览使用 level 决定高度。建表时另存满高度几何，并建立原模型编号到满高度编号的映射。子区块只比较上方流体的数值消面组，再切换到满高度形状。

## 多个方块放在一起时

直接叠加区域内每格的网格，会保留大量完全看不见的内面。区域入口只遍历约定的 N³ 个位置，再查询各格的共享 Shape 引用。N=1 时不会补齐或遍历 4096 格。默认填满的格子同样参与渲染。

栅栏、玻璃板和墙的连接方向已经包含在输入 ID 对应的 Java 状态中。调用者写入确定的连接状态，渲染时直接查表取得已经选好的部件。

若输入的栅栏、玻璃板或铁栏杆连接属性不可靠，调用者应先用 独立的 Java fixer 修复，再把结果交给渲染入口。

多层输入时，各层的几何按位置并进同一格，再一起剔面与合并，所以含水台阶的水和旁边的普通水之间照常剔掉内面。同一格内，前层的不透明面会裁掉后层与它共面的部分；前层是树叶这类满格镂空方块时，后层改为缩进到 0.99 倍并跳过剔面，避免共面闪烁。上方格任意一层有同类流体，本格流体就切换成满高度。

`prepareNeighborhood` 准备各格的共享几何引用，并根据上方邻居切换流体高度。独立子区块的外部按空覆盖处理。组装地图时，`MeshWithNeighbours(layers, neighbours, options)` 接收西、东、下、上、北、南六个邻居的完整多层数组，每层同样为 N³ 格；缺失邻居传 nil。入口只提取接触面的覆盖，也用上方邻居决定顶层流体是否满高。剔面和合并放在同一个渲染入口内，因为剔面需要直接访问邻居几何，并可能留下半个面这样的局部片段。这样可以直接把剩余面片交给合并步骤，省去另行传递一份裁剪结果。确定形状后，`meshing` 分两步减少面：

1. **剔除实际被盖住的部分。** 模型声明可剔除、且位于格子边界的轴对齐矩形，才会检查邻居。邻居覆盖直接来自 Shape.Boundary，无须逐格重扫几何。完整不透明面可以遮挡；多个部件的矩形并集也可以形成覆盖。石头紧贴底半砖时，只裁掉下半部分，并为保留部分重新插值 UV。斜面和非矩形保守保留。
2. **合并相邻矩形。** 在同一平面上按实际矩形边界建立二维网格，贪婪地扩展连续区域。只有材质、颜色、法线、光照方向、UV 变化率和重复相位兼容时才合并。合并后的 UV 延伸到多个纹理周期，保持每个方块原有的纹理尺寸。

透明面有额外规则。建表时将玻璃名称和流体家族转成 Shape 的数值消面组，避免渲染时解析名称。模型的 CullFace 也在烘焙时转为数值方向。相同玻璃、同类流体可以移除实际重叠的内面；不同玻璃保留界面。玻璃和树叶不会作为不透明遮挡物盖掉后面的石头。半透明面不参与大矩形合并，前端默认按四边形排序；地图页面传入 `sortTransparentQuads: false`，按材质合批绘制以限制调用数量。逐面排序不保证任意相交半透明表面的严格顺序。

所有格子复用建表时固定的模型候选。即使名称相同，UV 不兼容的面也不会强行合并。剔面与摆放由 `cullAndPlace` 处理，合并由 `merge` 处理，得到 `geometry.Mesh`。传输时调用 `Mesh.Marshal()`；需要扁平属性或 JSON 时调用 `Mesh.Buffers()`。

为避免重复分析同一个形状，剔面过程在一次构建内复用矩形、UV 变化率和满格覆盖的分析结果。裁切时复用临时缓冲，保留的面直接写入输出。分析缓存和临时缓冲都由当前调用独占，因此并发构建不会互相改写，后一次调用也不会沿用旧输入的分析结果。局部裁切仍使用 geometry 的共享矩形相减规则。

独立贪婪矩形可能形成 T 形接缝，即较短边接在另一条长边的中间；算法不额外缝合这些边。未提供邻居时外边界按暴露面处理；存档浏览入口通过 `MeshWithNeighbours` 提供相邻子区块。子区块解析不创建地图或 worker，地图加载与卸载属于另一个入口的调度职责。

## 几何怎样传给绘制端

单方块的 JSON 便于查看状态和几何。子区块则有更多顶点，数值文本解析和临时数组会产生额外开销。因此子区块通过 `Mesh.Marshal` 输出 **NVMS v1 二进制**：材质表使用一小段 JSON，顶点属性和索引使用对齐的小端数值数组。

浏览器的 `decodeMesh` 在常见的小端平台上为响应 `ArrayBuffer` 建立 `Float32Array`、`Uint32Array` 视图。Three.js 适配层直接保留这些视图；GPU 上传仍由 Three.js 完成。协议不依赖 HTTP，同一份数据也可以写入 `.nvmesh` 文件。布局与解析接口见 [NVMS v1 格式说明](mesher/geometry/FORMAT.md)。

## 复用前端绘制

外部页面需要把已有几何放进自己的 Three.js 场景时，可以使用独立前端包 `@neomega/viewer`。包根目录是 `web/`，绘制入口为 `@neomega/viewer/render`。examples 使用本地依赖 `file:../../web`，应用层也通过 `file:` 路径引用同一个包。

在宿主前端工程中安装共享包及匹配的 Three.js。将下面的路径替换为本机的仓库路径：

```sh
npm install /path/to/neomega-viewer-core/viewer/web three@0.186.1
```

包导出 TypeScript 源码，宿主需要支持 TypeScript 的构建工具。Three.js 以 `^0.186.1` 声明为 peer dependency，由宿主提供。本地链接的共享包也可能安装开发用的 Three.js。为让场景与绘制模块使用同一个实例，Vite 宿主应加入 `resolve: { dedupe: ["three"] }`。

拿到包含静态几何、block actor 和特效的完整 NVFR 快照时，使用 `decodeFrame` 解码为 `FrameData`，再交给 `createFrameObject` 统一绘制。block actor 是按方块实体数据生成的几何。三部分共享场景的深度缓冲和透明排序。特效名通过注册表映射到工厂。`createEffectRegistry` 提供附魔书工厂，书的贴图由宿主加载。下面使用宿主已创建的 `scene`、`camera` 和 `renderer`：

```ts
import {
  createEffectRegistry,
  enchantingTableBookTextureURL,
  createFrameObject,
  decodeFrame,
  loadAtlas,
} from "@neomega/viewer/render";

const [atlas, book] = await Promise.all([
  loadAtlas("/assets/atlas.png"),
  loadAtlas(enchantingTableBookTextureURL),
]);
const registry = createEffectRegistry(book);
const response = await fetch("/api/subchunks/block_entities/frame");
if (!response.ok) throw new Error(await response.text());
const data = decodeFrame(await response.arrayBuffer());
const frame = createFrameObject(data, atlas, registry);
scene.add(frame.object);

// 在宿主已有的每帧回调中，先更新时间，再绘制场景。
function render(timestamp: number): void {
  frame.updateTime(timestamp / 1000);
  renderer.render(scene, camera);
}
```

对象退出场景时，先移除 `frame.object`，再调用 `frame.dispose()` 释放它的缓冲、材质及 actor 私有贴图。图集与书本贴图由宿主在所有使用者释放后统一销毁。需要支持其他特效时，在 `registry` 中注册对应工厂。

希望一并复用相机、视口和动画驱动时，使用 `new MeshViewport(host, atlas, { effects: registry })` 创建视口，再调用其 `setFrame(data)` 显示快照。换一个快照会整体替换三部分内容。视口只在存在动画材质、特效或相机仍在移动时连续请求帧。

只有 NVMS 网格或单方块 JSON 时，使用 `createMeshObject`。它接受解码后的网格，也接受 JSON 的数值数组。沿用前面的图集和宿主场景：

```ts
import { createMeshObject, decodeMesh } from "@neomega/viewer/render";

const response = await fetch("/api/subchunks/solid/mesh");
if (!response.ok) throw new Error(await response.text());
const meshData = decodeMesh(await response.arrayBuffer());
const mesh = createMeshObject(meshData, atlas);
scene.add(mesh.object);
```

网格使用相同的时间更新和释放方式。需要视口时调用其 `set(meshData)`。`BlockViewport` 是 `MeshViewport` 的兼容导出。

需要只绘制筛选后的材质批次，或单独挂载特效时，使用 `createBatchObject` 和 `createEffectsObject`。例如，从前面的快照中取出透明批次并单独创建特效：

```ts
import { createBatchObject, createEffectsObject } from "@neomega/viewer/render";

// 这里只筛选使用公共图集的几何；私有贴图 actor 应交给 createActorObject。
const batches = [data.mesh, ...data.actors.filter(actor => !actor.textures?.length).map(actor => actor.mesh)]
  .flatMap((mesh) => mesh.batches)
  .filter((batch) => batch.surface.mode === "blend");
const transparent = createBatchObject(batches, atlas);
const effects = createEffectsObject(data.effects, registry);
scene.add(transparent.object, effects.object);
```

`createActorObject(actor, atlas)` 会同时绑定 actor 的私有图片。不能把这种 actor 的 mesh 单独取出后绑定公共图集。

这些入口返回统一的 `RenderObject`，可以像整帧对象一样更新时间和释放资源。批次对象从实际几何计算边界，特效对象的边界覆盖整个动画范围。宿主每帧向各对象传入同一个秒数。`animated` 可用于决定是否继续请求动画帧。

需要同时显示一批 section 时，逐个创建整帧对象会增加绘制提交。`SectionScene` 按空间区域合批，并统一管理共享材质、两级可见性和遮挡查询。使用公共图集的 actor 参与合批，私有贴图 actor 保持独立材质并随区域卸载。每个实例使用固定边长，构造函数第五个参数 `sectionSize` 默认为 16；使用其他 N 时由宿主在创建时指定。共享坐标类型 `SectionPos` 使用 section 坐标。帧内几何使用该 section 的局部坐标。

宿主先准备一组位置与 `FrameData`。下面的 `sections` 是宿主提供的位置与快照列表，可以一次挂载 Java 结构文件中的所有 section，也可以随着加载结果逐个加入：

```ts
import { SectionScene } from "@neomega/viewer/render";

const sectionScene = new SectionScene(
  scene,
  atlas,
  registry,
  renderer,
);
for (const { pos, frame } of sections) {
  const section = sectionScene.prepare(pos, frame);
  sectionScene.add(section);
}

function renderSections(timestamp: number): void {
  sectionScene.update(camera, timestamp / 1000);
  sectionScene.render(renderer, camera);
}
```

传入 `renderer` 后，场景可以用 GPU 遮挡查询减少绘制。`render` 统一安排不透明表面、查询和其余表面的绘制顺序。移除单个 section 时调用其 `dispose()`。关闭整组场景时调用 `sectionScene.dispose()` 释放已挂载对象和共享材质。数据读取、加载调度和缺失模型计数由宿主管理。

## 接口与扩展位置

仓库分发的 Java 几何表覆盖 `neomega-blocks/presets/java/v1.JavaCompatibleStates()` 枚举的目标及兼容状态，其状态不带 `waterlogged`。单方块身份查询与几何接口接受该状态集中的 hash。Pipeline 接受完整 Java 状态调色板，包括含水状态，经拆层得到 RSL 后查询几何表。应用演示服务的 hash 输入先反查成完整状态。mesher 本身可以使用调用者提供的其他数值 ID 与表。需要改变固定颜色和材质语义时，在 viewer-assets 建表侧通过 `tablegen.OpenOptions.Policy` 提供 `java_block_model.Policy`，并重新生成表。resources 加载器通过 `OpenOptions.TablesPath` 选择外部资源。

其他变化可以按职责定位：

| 要解决的问题 | 模块 |
|---|---|
| ID 对应哪个 Java 状态？ | `neomega-blocks/presets/java/v1.HashToBlock` |
| 如何修复转换丢失的连接、拐角与门状态？ | [fixer/java-postprocess](../fixer/java-postprocess/README.md)；实心表在 `neomega-viewer-core/fixer/solidtable/preset` |
| 含水方块怎样拆成多层？ | `splitlayer` |
| 带 NBT 的方块、特效怎样分流和打包？ | `dispatcher.Pipeline.Build`；批量 section 用 `BuildSections`，由 `merger` 编码 |
| 怎样接入另一种场景输入？ | 实现 `dispatcher.SectionSource`；浏览服务由外层应用组装 |
| 状态命中哪些部件，模型怎样继承？ | `neomega-viewer-core/viewer/geometry/java_block_model` |
| 一个状态怎样烘焙成局部几何？ | viewer-assets 的建表器，以及 `geometry/java_block_model` |
| 怎样编码与查询共享几何？ | `neomega-viewer-core/viewer/geometry.Tables[B]`；Java 预置表由 `resources.Open` 加载 |
| 如何构建单个方块？ | `Mesher.MeshBlock`；完整渲染流程见 [single_block](examples/single_block/README.md) |
| 如何从 N³ 个 ID 准备邻域几何？ | `Mesher.MeshLayer`；多层用 `Mesher.MeshLayers` |
| 哪些表面被覆盖，哪些矩形能够合并？ | `mesher/meshing` |
| 几何怎样编码和解码？ | `mesher/geometry` 的 `Mesh.Marshal`、`UnmarshalMesh`；前端 `web/render/binary.ts` |
| 哪些代码只是演示？ | 本目录的 [examples](examples/) 和应用层的 [demo](../../apps/neomega-viewer/demo/) |

## 验证

在 viewer-core 模块根目录运行 `go test ./...`，并在 `viewer/web` 运行 `npm ci`、`npm test` 和 `npm run typecheck`。
完整验收从工作区根目录运行 `./check.sh neomega-viewer-core`。
几何编解码使用固定样本验证字节兼容，网格测试覆盖裁切、合并、含水拆层和跨 section 邻居。

素材来源见模块的 [NOTICE.md](../NOTICE.md)。
