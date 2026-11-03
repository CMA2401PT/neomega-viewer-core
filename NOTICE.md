# 来源与许可

本项目原创的 Go 几何与修复、TypeScript 绘制适配和示例采用 [MIT](LICENSE)。下列依赖和输入素材的许可独立保留。

## 绘制代码

浏览器使用 [Three.js](https://github.com/mrdoob/three.js)，版本由 `viewer/web/package-lock.json` 固定为 0.186.1。

`Copyright © 2010-2026 three.js authors`

Three.js 的 MIT 原文保存在 [viewer/web/THREE-LICENSE.txt](viewer/web/THREE-LICENSE.txt)。Vite 和 TypeScript 是构建依赖，各自保留包内许可。

## 状态与资源

应用层的存档适配使用相邻 `neomega-celler` 中的固定子区块存储接口，渲染核心使用独立的方块输入。

Java 状态全集和方块哈希使用相邻的 `neomega-blocks` preset。转换数据的准备依赖 Amulet Team 的 PyMCTranslate。本项目使用现有 Java 1.21.4 目标表；本地转换库记录的 build 317 对应上游 1.2.30。该版本采用 Amulet Team License 1.0.0，包含非商业、教育和非竞争等使用条款；它不是 MIT 许可。完整条款和来源见 [neomega-blocks 的声明](../neomega-blocks/third_party/pymctranslate-notices/NOTICE.md)。

Required Notice: Copyright Amulet Team. (https://www.amuletmc.com/)

模型、公共图集和特殊方块的静态外形来自 `viewer-assets` 交付文件。
`viewer/blockactor/banner/textures`、`viewer/blockactor/decoratedpot/textures`、`viewer/blockactor/sign/textures` 与 `viewer/web/render/effects/enchanting_table_book.png` 保存同一 Java 1.21.4 资源来源的原始 PNG，作为固定源码资源维护。其中的社区模板来自 `zardoy/mc-assets` 固定提交 `c63d6dacd358a8ea73b453d6958167b49dcc3f6c`，声明为 `Copyright 2024 Vitaly Turovsky, contributors`，采用 MIT。原始模型署名由资源生成器保留。具体来源和适配关系见 [neomega-viewer-core/viewer-assets/NOTICE.md](viewer-assets/NOTICE.md)。

告示牌的 `viewer/blockactor/sign/font` 同时保存 Java 1.21.4 默认位图字体与配置，以及官方资源索引提供的 GNU Unifont 16.0.01。
`bitmap.json` 与三张 PNG 来自客户端 JAR；`unifont.json` 提取默认非日文字体的宽度覆盖表。
`unifont.zip` 保持官方分发包原样，包含原始 HEX 字体和 `LICENSE.txt`；另附 [UNIFONT-LICENSE.txt](viewer/blockactor/sign/font/UNIFONT-LICENSE.txt) 便于阅读。
Unifont 字体按原文提供 SIL Open Font License 1.1 与带字体嵌入例外的 GPL 条款，不能把本项目的 MIT 许可套用于这些字体。

Minecraft 模型、纹理及其衍生素材保留原权利人的权利。资源提取、图集打包和顶点生成不产生新的素材授权。应用分发资源时，应保留 `dist/final/THIRD_PARTY_NOTICES.txt`，并遵守输入素材适用的 [Minecraft EULA](https://www.minecraft.net/eula)、[Usage Guidelines](https://www.minecraft.net/usage-guidelines) 和额外资源包条款。

服务端分发时保留本文件、项目许可证、转换数据的许可声明与配套资源声明。单独分发网页时保留 Three.js 的许可文本；外部提供的模型和纹理仍按其自身来源附带声明。
