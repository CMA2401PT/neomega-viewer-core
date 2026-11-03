# neomega-viewer-core 开发说明

本模块把已经确定的 Java 状态转成几何，并提供资源生成与独立属性修复。修改前先按 [README.md](README.md) 选择能力入口；运行时渲染与属性修复由调用方组合。

## 依赖与职责

本模块只依赖 `neomega-blocks` 和 `neomega-nbt`。不要引入 `neomega-converter-core`、世界存储或应用模块。存档转换的组合在 `apps/neomega-converter`，存档浏览与加载调度在 `apps/neomega-viewer`。

`viewer/` 保存运行时渲染、前端绘制包和直接连接两端的示例。`viewer-assets/` 保存模型、图集与查询表的生成工具。`fixer/java-postprocess/` 根据邻居重算属性，`fixer/solidtable/` 保存实心表格式和预置数据。

## 修改资源与示例

普通 Go 构建使用入库资源。模型、图集或静态材质策略变化时，先完整阅读 [建表说明](viewer-assets/cmd/build-tables/README.md)，准备 `viewer-assets/dist/final`，再从本模块根目录运行：

```sh
go generate ./viewer/resources
```

几何中的 UV 与公共图集必须配套更新。特殊处理器的固定资源随其代码维护；新增 NBT 外观处理器前阅读 [处理器说明](viewer/blockactor/README.md)。

示例保持独立可读。最小 Go 用法在 `examples/mesh`，浏览器基础用法在 `viewer/examples/basic`。修改前端示例后，在相应目录执行 `npm ci` 与 `npm run build`，再启动 Go 服务检查网页及二进制接口。

## 验收

从工作区根目录运行：

```sh
./check.sh neomega-viewer-core
```

它检查依赖边界，并执行 Go 静态检查、测试和脱离工作区的独立编译。不要在工作区根目录直接执行 `go build ./...`。

修改共享前端绘制包时，在本模块的 `viewer/web` 目录运行：

```sh
npm ci
npm test
npm run typecheck
```

素材与第三方代码来源记录在 [NOTICE.md](NOTICE.md) 和 [资源声明](viewer-assets/NOTICE.md)。
