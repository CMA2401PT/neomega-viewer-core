# 26.3 资源复现

先将配套的 neomega-blocks 26.3 catalog 分支放在本模块旁边。运行时依赖新增的 `presets/java/v26_3`，旧方块转换 preset 保持原有版本。

固定客户端：`https://piston-data.mojang.com/v1/objects/e877b6a07acd633fb3bb475002175cec036e7b87/client.jar`。

- SHA-1：`e877b6a07acd633fb3bb475002175cec036e7b87`
- SHA-256：`4508d006323f24fa02876310c192d739af56516eb259000ac50f0909a68c9a2d`
- Wiki 版本页：`https://minecraft.wiki/w/Java_Edition_26.3`

模型、动画元数据和未匹配纹理来自该客户端。Wiki PNG 的 URL、原始 SHA-256 和目标路径保存在 `viewer/resources/textures/wiki-sources.json` 的 `selected` 中。只有像素与客户端一致的 PNG 才用于覆盖；清单包含 883 个资源路径，最终图集使用其中 852 个，其余纹理保留客户端来源。来源清单不会将游戏资产重新许可为 MIT。

在模块根目录执行以下 Python 3 脚本，下载固定客户端并恢复已锁定的 Wiki 覆盖；下载校验失败会停止，不会使用不同内容。

```python
import hashlib, json, pathlib, urllib.request

root = pathlib.Path("viewer-assets/.cache/26.3")
root.mkdir(parents=True, exist_ok=True)

def download(url, target, digest):
    if target.exists() and hashlib.sha256(target.read_bytes()).hexdigest() == digest:
        return
    request = urllib.request.Request(url, headers={"User-Agent": "neomega-assets-rebuild/1"})
    with urllib.request.urlopen(request, timeout=120) as response:
        data = response.read()
    if hashlib.sha256(data).hexdigest() != digest:
        raise ValueError("SHA-256 mismatch: " + url)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(data)

download("https://piston-data.mojang.com/v1/objects/e877b6a07acd633fb3bb475002175cec036e7b87/client.jar",
         root / "client.jar", "4508d006323f24fa02876310c192d739af56516eb259000ac50f0909a68c9a2d")
manifest = json.loads(pathlib.Path("viewer/resources/textures/wiki-sources.json").read_text(encoding="utf-8"))
for name, record in manifest["selected"].items():
    relative = pathlib.PurePosixPath(name)
    if relative.is_absolute() or ".." in relative.parts or not name.startswith("assets/minecraft/textures/"):
        raise ValueError("invalid texture path: " + name)
    download(record["url"], root / "overlay" / relative, record["sha256"])
```

然后在 `viewer-assets` 目录执行：

```sh
go run ./cmd/neomega-assets build -jar .cache/26.3/client.jar -overlay .cache/26.3/overlay -work dist/works -out dist/final
```

回到模块根目录，生成配套几何、物品、实心表与公共图集：

```sh
go generate ./viewer/resources
```

告示牌处理器的 26 张固定 PNG 也来自同一客户端和覆盖结果。以下脚本在模块根目录同步它们：

```python
import pathlib, zipfile
root = pathlib.Path("viewer-assets/.cache/26.3")
target = pathlib.Path("viewer/blockactor/sign/textures")
woods = "oak spruce birch jungle acacia dark_oak mangrove cherry bamboo crimson warped pale_oak poplar".split()
with zipfile.ZipFile(root / "client.jar") as client:
    for wood in woods:
        for suffix in ("sign", "hanging_sign"):
            name = f"assets/minecraft/textures/block/{wood}_{suffix}.png"
            overlay = root / "overlay" / name
            data = overlay.read_bytes() if overlay.exists() else client.read(name)
            (target / f"block_{wood}_{suffix}.png").write_bytes(data)
```

构建输出为 24,463 个 Java hash、8,449 种共享形状、1,559 项静态物品、2 个框体模型；图集包含 2,080 项纹理。12 个移动活塞状态没有静态模型，99 项复杂物品定义保留不支持原因。

验证命令（模块根目录，POSIX shell）：

```sh
go test ./...
go vet ./...
NEOMEGA_ASSETS_DIR="$PWD/viewer-assets/dist/final" go test -count=1 -v ./viewer-assets/cmd/build-tables/internal/tablegen
```

Windows PowerShell 使用 `$env:NEOMEGA_ASSETS_DIR = "$PWD/viewer-assets/dist/final"` 设置同一变量后运行最后一条 Go 命令。测试检查真实输入与提交、嵌入资源的一致性，不代表 Minecraft 客户端画面对比验证。
