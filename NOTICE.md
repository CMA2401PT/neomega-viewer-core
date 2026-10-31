# 来源与许可说明

`neomega-assets` 是用 Go 编写的资源整理工具。流程组织参考本工作区的 `my-mc-assets`。输出约定参考 `zardoy/mc-assets`，特殊方块的静态预览使用该项目的一部分模型与纹理。

## mc-assets 的参考与附带资源

- 项目：[zardoy/mc-assets](https://github.com/zardoy/mc-assets)
- 固定提交：`c63d6dacd358a8ea73b453d6958167b49dcc3f6c`
- 上游声明：`Copyright 2024 Vitaly Turovsky, contributors`
- 上游许可证：MIT，完整原文见 [resources/third_party/mc-assets/LICENSE](resources/third_party/mc-assets/LICENSE)。

参考位置与实现的关系如下：

| 上游位置 | 对应职责 |
|---|---|
| `src/datapaths-builder.ts` | `s01_extract` 的资源类别、目录与索引概念 |
| `src/newAssetsBuilder.ts`、`src/consumer/types.ts` | blocks 的 `geometry/java_block_model` 模型结构与本工具 `s03_models` 的导出约定 |
| `custom/blockentities/1.13`、`1.20`、`latest` 及 `custom/textures` | `s04_special` 使用的社区静态预览素材 |
| `src/genBlocks.ts`、`src/consumer/atlasCreator.ts` | `s05_atlas` 的产物名称、纹理类别与 UV 字段约定 |

下载、JAR 读取、模型图检查、PNG 处理和格子装箱由 Go 标准库实现。相关 Go 文件包含 SPDX 标识及来源注释。

附带的素材保存在 [resources/third_party/mc-assets/templates](resources/third_party/mc-assets/templates)，包含 150 份方块状态模板、234 份依赖模型和 17 张自定义纹理。文件对应关系保存在 `index.json`。这些文件来自上述固定提交，原有模型署名保留，其中包括 `Made with Blockbench by TyBraniff for Bluemaps support.`。

手工维护的分类与适配规则集中在 [resources/special.go](resources/special.go)，由第四步应用。导入时会改写模型命名空间、规范化资源引用、适配旗帜纹理的新路径，并为染色潜影盒选择对应皮肤。完整说明见 [模板来源与适配](resources/third_party/mc-assets/README.md)。这些适配不会改变原素材的权利来源。

## my-mc-assets

本地 `../my-mc-assets` 的 `s01_extract_jar`、`s02_generate_model` 实验提供了分步流程参考。该本地项目的 `package.json` 标注 ISC；其中的模型和纹理仍须按各自原始来源识别许可。

## Minecraft 资源和覆盖包

工具源码的 MIT 许可证授权工具源码及对应原创部分。Minecraft 模型、纹理及其衍生素材保留原权利人的权利。提取、整理、缩放或装入图集不会改变这些素材的许可来源，上游仓库的 MIT 声明也不应被视为 Mojang 素材的独立授权。

使用与分发生成结果时，需要结合 [Minecraft EULA](https://www.minecraft.net/eula)、[Minecraft Usage Guidelines](https://www.minecraft.net/usage-guidelines) 和额外覆盖包的作者条款判断。

JAR、提取目录和生成产物默认加入 `.gitignore`。`build-info.json` 和 `blockRenderers.json` 记录来源；导入模型保留原始署名。`THIRD_PARTY_NOTICES.txt` 包含来源说明和上游 MIT 文本。额外覆盖包的许可证应随使用其素材的分发结果一并保留。
