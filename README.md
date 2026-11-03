# neomega-viewer-core

本模块提供 Java 方块与普通静态物品渲染、配套资源生成和独立的 Java 属性修复。
渲染器接收完整 Java 状态并生成几何；资源工具预先准备模型、图集和查询表；fixer 根据邻居重算方块属性。
三者共享方块与几何定义，运行时渲染入口不调用 fixer。

本模块只依赖 `neomega-blocks` 和 `neomega-nbt`。
存档转换与修复的组合在 `neomega-converter`，存档浏览和复杂演示在 `neomega-viewer`。
两个 core 之间没有依赖。

## 根据目的选择入口

- [viewer](viewer/README.md) 提供 Go 网格构建与 Web 绘制，最简示例在 `viewer/examples`。
- [viewer-assets](viewer-assets/README.md) 从 Java 客户端资源生成模型和图集，再由建表器生成预置资源。
- [fixer/java-postprocess](fixer/java-postprocess/README.md) 通过遍历和邻居查询读取 Java 状态，返回需要写回的方块修改。
- `fixer/solidtable` 保存实心表格式，`fixer/solidtable/preset` 分发预置数据。

## 运行示例

从工作区根目录进入静态子区块示例：

```sh
cd neomega-viewer-core/viewer/examples/basic
npm ci
npm run build
go run .
```

浏览器打开 http://127.0.0.1:2413 。附魔书与陶罐、无限世界的入口见各示例 README。
普通 Go 构建直接使用入库的资源；更换模型或图集时，按 [建表说明](viewer-assets/cmd/build-tables/README.md) 更新。

## 验证与来源

工作区根目录的 `./check.sh neomega-viewer-core` 验证 Go 代码、独立编译与依赖边界。
前端包在 `viewer/web` 运行 `npm ci`、`npm test` 和 `npm run typecheck`。
素材和第三方代码的来源见 [NOTICE.md](NOTICE.md) 与 [资源声明](viewer-assets/NOTICE.md)。
