# 读取交付资源

应用加载方块时，需要将状态、模型和纹理连起来。`parse` 读取 `dist/final` 中的单版本文件，返回明确的 Go 类型，并提供按资源 ID 查询和将 UV 换算成像素范围的方法。它不依赖生成步骤，也不需要 JAR 或 `dist/works`。

模型中的继承、变体与加权候选会原样保留。解析完成后，应用根据具体方块状态选择模型，再按模型的父引用处理继承。特殊方块的完整外观需求来自 `blockRenderers.json`。

## 先用 CLI 看数据

在仓库根目录生成资源后，可以运行 [cli/main.go](cli/main.go)：

```sh
go run ./parse/cli -dir dist/final
```

默认输出版本、方块状态数、模型数、纹理数和实际 PNG 尺寸。要理解某个具体资源，再增加查询：

```sh
go run ./parse/cli -dir dist/final \
  -block minecraft:chest \
  -model minecraft:block/stone \
  -texture minecraft:entity/chest/normal
```

三个查询互相独立，可以只保留需要的参数。方块查询返回模型选择和渲染需求；模型查询返回原始定义；纹理查询返回 UV 和 PNG 中的像素矩形。默认命名空间可以省略，例如 `-block chest`。

## 在 Go 中加载

需要把资源交给自己的渲染器时，直接使用同一个读取接口：

```go
package example

import (
    "fmt"

    "neomega-assets/parse"
)

func inspect() error {
    assets, err := parse.Load("dist/final")
    if err != nil {
        return err
    }
    state, ok := assets.BlockState("minecraft:chest")
    if !ok {
        return fmt.Errorf("chest is missing")
    }
    fmt.Println(state.Variants)
    fmt.Println(assets.Renderers.Blocks["minecraft:chest"])

    pixels, ok := assets.TextureRect("minecraft:entity/chest/normal")
    if ok {
        fmt.Println(pixels, assets.AtlasImage.At(pixels.Min.X, pixels.Min.Y))
    }
    return nil
}
```

这些方法接收 `asset.ID`。从用户输入构造 ID 时，先调用 `asset.ParseID`。查询方法处理默认命名空间与导出名之间的转换；模组命名空间会保留。纹理 ID 使用原始路径，例如 `minecraft:block/stone` 或 `minecraft:entity/chest/normal`。

## 只读取需要的文件

并非每个应用都需要解码 PNG。`ReadBlockStates`、`ReadModels`、`ReadAtlas`、`ReadRenderers`、`ReadBuildInfo` 和 `ReadAtlasImage` 可以分别读取对应文件。例如：

```go
models, err := parse.ReadModels("dist/final/models.json")
```

`blockstates.json` 和 `models.json` 直接保存资源名到定义的映射。`blocksAtlases.json` 直接保存单张图集的描述，没有 `latest` 包装层。版本与来源放在 `build-info.json` 中；`THIRD_PARTY_NOTICES.txt` 是需要随交付保留的文本声明。
