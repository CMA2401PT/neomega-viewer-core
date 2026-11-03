// 在本目录运行 npm ci、npm run build、go run .，再打开 http://127.0.0.1:2413。
package main

import (
	"bytes"
	"flag"
	"log"
	"net/http"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/mesher"
	"neomega-viewer-core/viewer/mesher/geometry"
	preset "neomega-viewer-core/viewer/resources"
)

// 示例中的所有输入共用这一个 16³ 坐标约定。
var layout cube.Layout = cube.New(16)

func main() {
	var listen *string = flag.String("listen", "127.0.0.1:2413", "HTTP 监听地址")
	flag.Parse()
	// 1. 加载随库内嵌的 Java 几何表；Tables 用状态 ID 查找形状和材质。
	var resources *preset.Preset
	var err error
	resources, err = preset.Open(preset.OpenOptions{})
	if err != nil {
		log.Fatal(err)
	}
	// 2. 本例只有单层普通方块，直接构建网格；含水、附魔书和陶罐见 single_block。
	var meshBuilder *mesher.Mesher[uint32] = mesher.NewMesher(layout, resources.Tables)
	// 3. 每个请求顺序完成数据构造、网格生成与编码。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/subchunk", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("subchunk") != "default" {
			http.Error(w, "未知子区块", http.StatusNotFound)
			return
		}
		// 每个请求直接构造方块、剔面、合并，再编码成 NVMS 网格。
		// 空 Options 默认启用剔面与相邻面合并，区域外按空气处理。
		var result *mesher.Result
		var err error
		result, err = meshBuilder.MeshLayer(defaultSubchunk(), mesher.Options{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// NVMS 只包含网格；先编码完再响应，编码失败时仍能返回 HTTP 错误。
		var data bytes.Buffer
		if err := result.Mesh.Marshal(&data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", geometry.ContentType)
		if _, err := w.Write(data.Bytes()); err != nil {
			log.Print(err)
		}
	})
	// 4. 同时提供配套贴图和构建后的网页；运行目录须为当前示例目录。
	mux.Handle("GET /textures/", http.StripPrefix("/textures/", http.FileServerFS(preset.Resources())))
	mux.Handle("GET /", http.FileServer(http.Dir("dist")))
	log.Printf("http://%s", *listen)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

// defaultSubchunk 构造石头平台、木屋、玻璃窗与阶梯形屋顶，返回 16³ 个状态 ID。
func defaultSubchunk() []uint32 {
	// 几何表使用规范化 Java 状态的 Hash；这些无属性状态已经规范化。
	var air uint32 = ir.IRBlock("minecraft:air []").Hash()
	var stone uint32 = ir.IRBlock("minecraft:stone []").Hash()
	var oak uint32 = ir.IRBlock("minecraft:oak_planks []").Hash()
	var spruce uint32 = ir.IRBlock("minecraft:spruce_planks []").Hash()
	var glass uint32 = ir.IRBlock("minecraft:glass []").Hash()
	var blocks []uint32 = make([]uint32, layout.Len())
	// 空气也有自己的状态 ID，不能用 uint32 的零值代替。
	for i := range blocks {
		blocks[i] = air
	}
	// 所有写入用同一个布局寻址：Y 最快，Z 次之，X 最慢。
	for x := 2; x < 14; x++ {
		for z := 2; z < 14; z++ {
			blocks[layout.Index(x, 0, z)] = stone
		}
	}
	for x := 5; x <= 10; x++ {
		for z := 5; z <= 10; z++ {
			blocks[layout.Index(x, 1, z)] = oak
			for y := 2; y <= 5; y++ {
				if x == 5 || x == 10 || z == 5 || z == 10 {
					blocks[layout.Index(x, y, z)] = oak
				}
			}
		}
	}
	// 朝 +Z 开门洞；两侧和后墙留窗。
	for x := 7; x <= 8; x++ {
		blocks[layout.Index(x, 2, 10)] = air
		blocks[layout.Index(x, 3, 10)] = air
		blocks[layout.Index(x, 3, 5)] = glass
	}
	for z := 7; z <= 8; z++ {
		blocks[layout.Index(5, 3, z)] = glass
		blocks[layout.Index(10, 3, z)] = glass
	}
	for y := 6; y <= 9; y++ {
		inset := y - 6
		for x := 4 + inset; x <= 11-inset; x++ {
			for z := 4; z <= 11; z++ {
				blocks[layout.Index(x, y, z)] = spruce
			}
		}
	}
	return blocks
}
