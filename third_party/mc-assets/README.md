# 特殊方块模板的来源

JAR 中有一部分方块由游戏代码绘制，普通模型文件只有占位信息。为了让资源消费者能先显示静态外形，这里附带 `mc-assets` 的一部分方块实体模板，以及它们实际依赖的模型和自定义纹理。`s04_special` 按需导入这些文件，运行工具时不需要访问上游或本工作区的 `refs` 目录。

## 原始文件在哪里

来源仓库：[zardoy/mc-assets](https://github.com/zardoy/mc-assets)，固定提交 [`c63d6dacd358a8ea73b453d6958167b49dcc3f6c`](https://github.com/zardoy/mc-assets/tree/c63d6dacd358a8ea73b453d6958167b49dcc3f6c)。

[templates/index.json](templates/index.json) 记录资源 ID 与本地文件的对应关系。文件按以下关系收录：

| 本地位置 | 上游位置 | 文件数 |
|---|---|---:|
| `templates/states` | `custom/blockentities/latest/blockStates` | 86 |
| `templates/models` | `custom/blockentities/latest/blockModels` | 147 |
| `templates/textures` | `custom/textures` | 17 |

选择范围对应 26.3 原始 JAR 中需要补充预览的方块。依赖闭包中的公共父模型也包含在内。17 张 PNG 为 16 色旗帜预览纹理和一张末地传送门预览纹理。原始文件直接保留；生成阶段才应用适配。

## 为什么导入时还需要适配

模板的资源名与最新 JAR 并不完全相同，直接覆盖可能破坏已经存在的原生模型。因此导入器执行以下处理：

- 只对缺少几何的方块使用模板，依赖模型移到 `neomega:` 命名空间。
- 规范化模型和纹理引用，将旧 `entity/banner_base` 路径映射到 `entity/banner/banner_base`。
- 为染色潜影盒选择对应的实体皮肤，并移除额外染色，避免重复着色。
- 仅补充缺少的自定义纹理，调用者明确提供的资源覆盖优先。

模板给出的是静态预览。开盖、旗帜图案、皮肤等动态需求另外记录在 `blockRenderers.json` 中。

新增模板时，应从明确提交收录文件及依赖，更新索引、署名和本页数量，并用目标 JAR 重跑完整构建及 `audit`。不能仅凭模板存在就认定它与新版本资源兼容。

## 署名与许可

上游声明为 `Copyright 2024 Vitaly Turovsky, contributors`，MIT 原文保存在 [LICENSE](LICENSE)。模型 JSON 中的 `credit` 等原始字段会保留到生成结果。已收录的署名包括：

- `Made with Blockbench`
- `Made with Blockbench by TyBraniff for Bluemaps support.`

Minecraft 原始素材及衍生纹理保留原权利人的权利。上述 MIT 声明不能替代 Minecraft 素材本身的使用与分发条款。仓库整体的来源与分发说明见 [NOTICE.md](../../NOTICE.md)。
