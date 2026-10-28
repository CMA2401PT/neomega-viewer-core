# 来源与许可说明

`neomega-assets` 是用 Go 编写的资源整理工具。流程组织参考本工作区的 `my-mc-assets`；方块模型、纹理图集的输出命名与兼容约定参考 `zardoy/mc-assets`。

## mc-assets

- 项目：<https://github.com/zardoy/mc-assets>
- 本地参考位置：`../refs/mc-assets`
- 参考提交：`c63d6dacd358a8ea73b453d6958167b49dcc3f6c`
- 上游声明：`Copyright 2024 Vitaly Turovsky, contributors`
- 许可证：MIT，完整原文见 [third_party/mc-assets/LICENSE](third_party/mc-assets/LICENSE)。

主要参考对应关系：

| 上游位置 | 在 Go 实现中对应的职责 |
|---|---|
| `src/datapaths-builder.ts` | `s01_extract` 的资源类别、目录与索引概念 |
| `src/newAssetsBuilder.ts`、`src/consumer/types.ts` | `asset`、`s03_models` 的模型结构和导出约定 |
| `src/genBlocks.ts`、`src/consumer/atlasCreator.ts` | `s04_atlas` 的产物名称、纹理类别与 UV 字段约定 |

下载、JAR 读取、模型图检查、PNG 处理和分行装箱由 Go 标准库实现。`s03_models` 保留 Minecraft 的变体和继承结构；`s04_atlas` 将 PNG 与坐标表一起生成。相关 Go 文件包含 SPDX 标识及来源注释。

## my-mc-assets

本地 `../my-mc-assets` 的 `s01_extract_jar`、`s02_generate_model` 分步实验提供了流程组织参考。它的入口将提取、修正、模型整理分开，这里沿用这种可逐步检查的组织方式。该本地项目的 `package.json` 标注 ISC；其中的模型和纹理仍须按各自原始来源识别许可。

## Minecraft 资源和覆盖包

工具源码的 MIT 许可证只授权工具源码及对应的原创部分。客户端 JAR 中的 Minecraft 模型、纹理和其他资源保留原权利人的权利；提取、整理、缩放或装入 PNG 不会改变它们的许可来源。

生成结果应结合以下原始条款判断使用与分发条件：

- [Minecraft EULA](https://www.minecraft.net/eula)
- [Minecraft Usage Guidelines](https://www.minecraft.net/usage-guidelines)
- 额外覆盖包的作者许可与声明

JAR、提取目录和生成产物默认加入 `.gitignore`。输出 `build-info.json` 记录输入来源；`THIRD_PARTY_NOTICES.txt` 包含来源说明和上游 MIT 文本。覆盖包的额外许可证应由使用者随分发结果一并保留。
