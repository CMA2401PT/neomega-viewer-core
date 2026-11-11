# neomega-viewer-core

本模块提供 Java 方块与普通静态物品渲染、配套资源生成和独立的 Java 属性修复。
运行时提供网格、俯视图和静态正交图片三种入口。
资源工具预先准备模型、图集和查询表，fixer 根据邻居重算方块属性。
三者共享方块与几何定义，运行时渲染入口不调用 fixer。

需要读取存档并浏览世界时，使用 [neomega-viewer](../apps/neomega-viewer/README.md)。需要把存档转换与属性修复组合起来时，使用 [neomega-converter](../apps/neomega-converter/README.md)。

## 根据目的选择入口

- [viewer](viewer/README.md) 提供 Go 网格构建与 Web 绘制，浏览器示例在 `viewer/examples`。
- [static_orthogonal](viewer/static_orthogonal/README.md) 以预绘制的颜色立方体和逐像素深度生成静态正交图片瓦片。
- [viewer-assets](viewer-assets/README.md) 从 Java 客户端资源生成模型和图集，再由建表器生成预置资源。
- [fixer/java-postprocess](fixer/java-postprocess/README.md) 通过遍历和邻居查询读取 Java 状态，返回需要写回的方块修改。
- `fixer/solidtable` 保存实心表格式，`fixer/solidtable/preset` 分发预置数据。

## 运行示例

只想在 Go 中生成一个石头方块的二进制网格，可从工作区根目录运行：

```sh
cd neomega-viewer-core
go run ./examples/mesh
```

命令打印四边形数量与编码字节数，不需要浏览器或外部素材。完整输入约定见 [最小网格示例](examples/mesh/main.go)。

需要看到渲染画面时，从工作区根目录进入静态子区块示例：

```sh
cd neomega-viewer-core/viewer/examples/basic
npm ci
npm run build
go run .
```

浏览器打开 http://127.0.0.1:2413 。附魔书与陶罐、无限世界的入口见各示例 README。
普通 Go 构建直接使用入库的资源；更换模型或图集的方法见 [开发说明](AGENTS.md)。

## 来源与开发

素材和第三方代码的来源见 [NOTICE.md](NOTICE.md) 与 [资源声明](viewer-assets/NOTICE.md)。修改本模块、重新生成资源和运行验收的方法见 [AGENTS.md](AGENTS.md)。
