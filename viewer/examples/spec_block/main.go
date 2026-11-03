package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	jmodel "neomega-viewer-core/viewer/geometry/java_block_model"
	"neomega-viewer-core/viewer/merger"
	preset "neomega-viewer-core/viewer/resources"
)

// 示例中的所有输入共用这一个 16³ 坐标约定。
var layout = cube.New(16)

func main() {
	var listen *string = flag.String("listen", "127.0.0.1:2414", "HTTP 监听地址")
	flag.Parse()
	// 1. 加载随库内嵌的 Java 几何表；Tables 用状态 ID 查找形状和材质。
	var resources *preset.Preset
	var err error
	resources, err = preset.Open(preset.OpenOptions{})
	if err != nil {
		log.Fatal(err)
	}
	// 陶罐 actor 按 NBT 选择纹样，需要与几何表配套的图集区域。
	var atlasJSON []byte
	atlasJSON, err = fs.ReadFile(preset.Resources(), "blocksAtlases.json")
	if err != nil {
		log.Fatal(err)
	}
	var atlas jmodel.Atlas
	if err := json.Unmarshal(atlasJSON, &atlas); err != nil {
		log.Fatal(err)
	}
	// 2. 组装完整渲染入口：普通网格、让网页创建书本的消息（effect）、按 NBT 生成的陶罐网格（actor）。
	// 最后一个 nil 表示没有缺失模型的替代形状，遇到未知模型会返回错误。
	pipeline := dispatcher.NewJavaPipeline(layout, resources.Tables, &atlas, nil)
	// 3. 每个请求顺序完成数据构造、网格生成与编码。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/subchunk", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("subchunk") != "default" {
			http.Error(w, "未知子区块", http.StatusNotFound)
			return
		}
		// 在请求内分发静态网格、effect 和 block actor，再编码为完整快照。
		var result *dispatcher.Result
		var err error
		result, err = pipeline.Build(
			defaultSubchunk(),
			dispatcher.Neighbours{}, // 六向邻居留空，区域外按空气处理。
			dispatcher.Options{},    // 默认启用剔面与相邻面合并。
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, diagnostic := range result.Diagnostics {
			log.Printf("%s at %v: %s (%v)", diagnostic.Kind, diagnostic.Pos, diagnostic.State, diagnostic.Err)
		}
		// NVFR 是完整快照，包含静态网格、actor 网格和 effect 消息。
		// 先在内存中编码完，失败时仍能返回 HTTP 错误，不会发出半份快照。
		var data bytes.Buffer
		if err := result.Frame.Marshal(&data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", merger.ContentType)
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

// defaultSubchunk 构造木屋、附魔台和陶罐；Section 的尺寸由 layout 约定，不存入数据。
func defaultSubchunk() dispatcher.Section {
	air := ir.IRBlock("minecraft:air []")
	stone := ir.IRBlock("minecraft:stone []")
	oak := ir.IRBlock("minecraft:oak_planks []")
	spruce := ir.IRBlock("minecraft:spruce_planks []")
	glass := ir.IRBlock("minecraft:glass []")
	// 保留完整 Java 状态，Pipeline 才能识别含水、effect 与 actor；Indices 每格一个索引。
	section := dispatcher.Section{
		Blocks:   blockdata.Uniform(layout.Len(), air),
		Entities: map[dispatcher.CellPos]nbt.Compound{},
	}
	set := func(x, y, z int, state ir.IRBlock) {
		// 线性索引中 Y 最快、Z 次之、X 最慢；实体 NBT 则直接用局部 (x,y,z)。
		section.Blocks.Set(layout.Index(x, y, z), state)
	}
	for x := 2; x < 14; x++ {
		for z := 2; z < 14; z++ {
			set(x, 0, z, stone)
		}
	}
	for x := 5; x <= 10; x++ {
		for z := 5; z <= 10; z++ {
			set(x, 1, z, oak)
			for y := 2; y <= 5; y++ {
				if x == 5 || x == 10 || z == 5 || z == 10 {
					set(x, y, z, oak)
				}
			}
		}
	}
	// 朝 +Z 开门洞；两侧和后墙留窗。
	for x := 7; x <= 8; x++ {
		set(x, 2, 10, air)
		set(x, 3, 10, air)
		set(x, 3, 5, glass)
	}
	for z := 7; z <= 8; z++ {
		set(5, 3, z, glass)
		set(10, 3, z, glass)
	}
	for y := 6; y <= 9; y++ {
		inset := y - 6
		for x := 4 + inset; x <= 11-inset; x++ {
			for z := 4; z <= 11; z++ {
				set(x, y, z, spruce)
			}
		}
	}
	for _, x := range []int{4, 7, 10} {
		set(x, 1, 12, ir.IRBlock("minecraft:enchanting_table []"))
	}
	for i, facing := range []string{"north", "east", "west"} {
		pos := dispatcher.CellPos{12, 1, uint8(4 + 4*i)}
		// Tidy 将属性按规范排序，让状态与几何表的 ID 一致。
		var state ir.IRBlock = ir.IRBlock(`minecraft:decorated_pot ["facing"="` + facing + `","cracked"="false","waterlogged"="false"]`).Tidy()
		set(int(pos[0]), int(pos[1]), int(pos[2]), state)
		// 四面顺序：back、left、right、front；纹样来自 NBT，而非方块状态。
		section.Entities[pos] = nbt.Compound{
			"sherds": []string{
				"minecraft:angler_pottery_sherd",
				"minecraft:archer_pottery_sherd",
				"minecraft:blade_pottery_sherd",
				"minecraft:arms_up_pottery_sherd",
			},
		}
	}
	return section
}
