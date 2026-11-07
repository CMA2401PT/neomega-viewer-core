# 告示牌文字与外观

`sign.Handler` 把 Java 告示牌状态和实体 NBT 转成牌体、文字面与私有 RGBA 图片。
Go 解析两面的文本组件，使用内嵌的 Java 默认字体生成文字图片；浏览器只绘制收到的网格和图片。
它复用 `blockactor.Appearance`，无需修改分发协议或增加前端告示牌处理器。

牌体来自几何表，木纹与字体固定资源放在本包的 `textures/` 和 `font/`。
`sign.New(tables)` 在创建处理器时加载资源，之后构建不同告示牌只生成各自的文字图片。
默认的 `javapreset.NewPipeline` 已注册这个处理器。
自行组装时，把 `sign.New` 返回的处理器传给 `blockactor.New` 即可。

## 输入与双面文字

支持 `neomega-blocks/presets/java/v1.JavaFullStates()` 中的所有告示牌，包括立式、挂墙、悬挂、墙上悬挂四种形态。
木材由状态名称推导；当前目标 Java 26.3 包含 13 种木材（含杨木 poplar）。
完整状态决定木材、朝向、链条形态和是否含水。
含水层仍由 Pipeline 拆分，处理器只生成牌体与文字。

`front_text` 与 `back_text` 分别描述正反两面，各有最多四行 `messages`。
每行可以是 JSON 字符串组件，也可以是新版 Java 的直接 NBT 组件。
`color` 使用染料名，省略时为黑色；`has_glowing_text` 是 NBT Byte 或 Go bool，省略时不发光。
缺少某一面时显示空白；旧版 `Text1`…`Text4`、`Color` 和 `GlowingText` 在没有 `front_text` 时解释为正面。

例如，把下面的 NBT 绑定到 `minecraft:oak_sign ["rotation"="0","waterlogged"="false"]` 所在格：

```go
entity := nbt.Compound{
    "front_text": nbt.Compound{
        "messages": []string{
            `{"text":"你好，世界"}`,
            `{"text":"Java Sign","bold":true}`,
            `{"text":"红色","color":"red"}`,
            `{"text":"下划线","underlined":true}`,
        },
        "color": "black",
    },
    "back_text": nbt.Compound{
        "messages": []nbt.Compound{{"text": "背面发光"}},
        "color": "blue",
        "has_glowing_text": int8(1),
    },
}
```

这里的 `nbt` 指 `neomega-nbt/presets/v1`。
完整请求、实体坐标绑定和快照输出见 [单方块示例](../../examples/single_block/main.go)。

## 排版与发光

字体资源取自目标版本客户端的默认位图字体，以及官方资源索引提供的 Unifont 16.0.01。
字形选择依次使用空格定义、原版位图字符表和 Unifont，保留对应字宽、重音偏移、中文宽度覆盖与粗体偏移。
Unicode 码点直接作为索引，因此非 BMP 字符不会拆成两个字。
文字以半个 Java 字体单位为一个像素烘焙，固定图片不依赖宿主系统字体。

普通告示牌的文字宽度为 90 字体单位，行距为 10，世界缩放为 1/96。
悬挂牌的宽度为 60，行距为 9，世界缩放为 0.9/64。
四行各自居中，过长时只显示首行；优先在空格处分行，无空格时按字符截断。
正反面独立排版，背面不会镜像。
斜体在半单位像素网格上取整，因此斜边和 GPU 直接绘制相比会有细微差别。

普通染料文字使用染料文字颜色的 40% RGB；文本组件中显式指定的颜色保持原值。
发光文字使用完整颜色，并通过已有的 `Surface.Unlit` 跳过前端方向光压暗。
发光轮廓使用压暗后的染料色，黑色染料例外，使用 `#F0EBCC` 浅色。
描边按字形自身的阴影偏移绘制八个方向：位图字体偏移 1 单位，Unifont 偏移 0.5 单位。
下划线与删除线不参与描边。
贴图在 Go 生成，不随相机距离重建，所以发光文字始终保留轮廓，没有复制客户端按距离开关轮廓的规则。
这里的发光是文字自亮与描边，不向周围方块投光，也没有额外光晕后处理。

## 文本支持范围

支持 `text`、`extra` 和组件数组的样式继承，支持命名颜色、`#RRGGBB`、粗体、斜体、下划线和删除线。
子组件可以用 `false` 或 NBT 的 `0b` 取消父组件样式。
`§0`…`§f` 与 `§l/o/n/m/r` 按 Java 含义解释，兼容 Bedrock 告示牌转换后的文本。
`§k` 和 `obfuscated` 的连续区段固定替换为一个 `…`。
每个消息遇到换行符后只保留第一行。

`translate` 使用随组件提供的 `fallback`，否则显示翻译键；支持 `%s`、`%1$s` 与 `%%`。
`keybind` 显示键名，`score.value` 显示已解析的值。
没有服务端上下文的选择器、动态 NBT 和未解析分数显示省略号。
点击和悬停事件不参与静态外观。
未实现自定义字体包、语言翻译表、日文替代字形、双向排版与复杂文字塑形。
缺字显示方框。

无效 NBT 返回错误，由 Pipeline 记录 `ActorError` 并保留静态牌体，其他方块继续构建。

## 依据与资源来源

排版、文字变换和发光规则依据目标版本客户端的 `AbstractSignRenderer`、`StandingSignRenderer`、`HangingSignRenderer`、`BitmapProvider` 与 `UnihexProvider`。
牌体纹理取自目标版本客户端的 `assets/minecraft/textures/block/{wood}_sign.png` 和 `{wood}_hanging_sign.png`。
本包 `textures/` 保留对应文件名，处理器以 `minecraft:block/<文件名去掉 .png>` 绑定几何材质。
位图字体取自同一客户端的 `assets/minecraft/textures/font/`。
`font/bitmap.json` 保留客户端 `assets/minecraft/font/include/default.json` 的位图提供器配置；`font/unifont.json` 只保存默认非日文字体的宽度覆盖表。
Unifont ZIP 保留原始字体与许可证，包外也附有 [许可证原文](font/UNIFONT-LICENSE.txt)。
素材许可见模块的 [NOTICE.md](../../../NOTICE.md)。

格式码处理参考了工作区的 `research/TellrawR` 与 [minecraft_formatting_code_online](https://github.com/Spectrollay/minecraft_formatting_code_online)。
实际字形与告示牌布局采用客户端资源和规则，没有依赖浏览器 CSS 字体或它们的网页描边策略。
