package main

import (
	"bytes"
	"flag"
	"log"
	"net/http"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/blockdata"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/javapreset"
	"neomega-viewer-core/viewer/merger"
	preset "neomega-viewer-core/viewer/resources"
)

func main() {
	var listen *string = flag.String("listen", "127.0.0.1:2418", "HTTP 监听地址")
	flag.Parse()
	// 1. 加载随库内嵌的 Java 几何表；Tables 用状态 ID 查找形状和材质。
	var resources *preset.Preset
	var err error
	resources, err = preset.Open(preset.OpenOptions{})
	if err != nil {
		log.Fatal(err)
	}
	// 2. 固定处理 1³；Java 预设加载配套资源并组装方块实体和展示框处理器。
	layout := cube.New(1)
	// 最后一个 nil 表示没有缺失模型的替代形状，遇到未知模型会返回错误。
	// Items 提供展示框所用的普通物品几何，必须与 Tables 使用同一套图集。
	pipeline, err := javapreset.NewPipeline(layout, resources.Tables, resources.Items, nil)
	if err != nil {
		log.Fatal(err)
	}
	// 3. 每个请求顺序完成数据构造、网格生成与编码。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/block", func(w http.ResponseWriter, r *http.Request) {
		scene, found := scenes[r.URL.Query().Get("scene")]
		if !found {
			http.Error(w, "未知场景", http.StatusNotFound)
			return
		}
		// 构建器约定 N=1，因此输入只分配一格；尺寸不随每份数据重复传递。
		// Tidy 规范化完整状态；含水状态由 Pipeline 拆出水层，无需手工叠加网格。
		var blocks *blockdata.Blocks = blockdata.Uniform(layout.Len(), scene.State.Tidy())
		var result *dispatcher.Result
		var err error
		result, err = pipeline.Build(
			// Section 表示约定的 N³ 区域，这里只有一格；方块实体 NBT 按局部坐标关联。
			dispatcher.Section{
				Blocks:        blocks,
				BlockEntities: map[dispatcher.CellPos]nbt.Compound{{0, 0, 0}: scene.NBT},
			},
			dispatcher.Neighbours{}, // 六向邻居留空，区域外按空气处理。
			dispatcher.Options{},    // 默认启用剔面与相邻面合并。
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, diagnostic := range result.Diagnostics {
			log.Printf("%s: %s (%v)", diagnostic.Kind, diagnostic.State, diagnostic.Err)
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

// 每个场景只有一个完整 Java 状态；方块实体 NBT 指定纹样或文字，按局部坐标绑定到这一格。
var scenes = map[string]struct {
	State ir.IRBlock
	NBT   nbt.Compound
}{
	"waterlogged_leaves": {
		State: ir.IRBlock(`minecraft:oak_leaves ["distance"="1","persistent"="true","waterlogged"="true"]`),
	},
	"fire": {
		State: ir.IRBlock(`minecraft:fire ["age"="0","east"="false","north"="false","south"="false","up"="false","west"="false"]`),
	},
	"enchanting_table": {
		State: ir.IRBlock("minecraft:enchanting_table []"),
	},
	"banner": {
		State: ir.IRBlock(`minecraft:white_banner ["rotation"="0"]`),
		NBT: nbt.Compound{"patterns": []any{
			map[string]any{"pattern": "minecraft:stripe_bottom", "color": "blue"},
			map[string]any{"pattern": "minecraft:cross", "color": "red"},
			map[string]any{"pattern": "minecraft:border", "color": "black"},
		}},
	},
	"wall_banner": {
		State: ir.IRBlock(`minecraft:yellow_wall_banner ["facing"="east"]`),
		NBT: nbt.Compound{"patterns": []any{
			map[string]any{"pattern": "minecraft:gradient", "color": "red"},
			map[string]any{"pattern": "minecraft:flower", "color": "white"},
		}},
	},
	"decorated_pot": {
		State: ir.IRBlock(`minecraft:decorated_pot ["cracked"="false","facing"="north","waterlogged"="false"]`),
		NBT: nbt.Compound{
			"sherds": []string{
				"minecraft:angler_pottery_sherd",
				"minecraft:archer_pottery_sherd",
				"minecraft:blade_pottery_sherd",
				"minecraft:arms_up_pottery_sherd",
			},
		},
	},
	"sign": {
		State: ir.IRBlock(`minecraft:oak_sign ["rotation"="0","waterlogged"="false"]`),
		// 正反面各有最多四行 messages。这里使用旧 Java 存档常见的 JSON 字符串组件。
		// color 是整面的染料名；has_glowing_text 用 NBT Byte 表示。拖动相机可看背面。
		NBT: nbt.Compound{
			"front_text": nbt.Compound{
				"messages": []string{`{"text":"你好，世界"}`, `{"text":"Java Sign"}`, `{"text":"红色","color":"red","extra":[{"text":" + 蓝色","color":"blue"}]}`, `{"text":"粗体","bold":true}`},
				"color":    "black",
			},
			"back_text": nbt.Compound{
				"messages": []string{`{"text":"背面发光"}`, `{"text":"Glow text"}`, `{"text":"斜体","italic":true}`, `{"text":"下划线","underlined":true}`},
				"color":    "blue", "has_glowing_text": int8(1),
			},
		},
	},
	"wall_sign": {
		State: ir.IRBlock(`minecraft:cherry_wall_sign ["facing"="south","waterlogged"="false"]`),
		// 新 Java 也能直接把文本组件写成 NBT compound，Go 端使用相同的排版入口。
		NBT: nbt.Compound{"front_text": nbt.Compound{
			"messages": []nbt.Compound{
				{"text": "挂墙告示牌"},
				{"text": "ABC", "color": "#0066FF"},
				{"text": "删除线", "strikethrough": int8(1)},
				{"text": "é à ü 中"},
			},
			"color": "black",
		}},
	},
	"hanging_sign": {
		State: ir.IRBlock(`minecraft:spruce_hanging_sign ["attached"="false","rotation"="0","waterlogged"="false"]`),
		// 悬挂牌的文字更窄；黑色发光文字使用浅色描边，因此在深色木板上仍可辨认。
		NBT: nbt.Compound{"front_text": nbt.Compound{
			"messages": []string{"悬挂告示牌", "Black glow", "§n下划线", "§l粗体"},
			"color":    "black", "has_glowing_text": int8(1),
		}},
	},
	"wall_hanging_sign": {
		State: ir.IRBlock(`minecraft:birch_wall_hanging_sign ["facing"="south","waterlogged"="false"]`),
		// § 格式码兼容 Bedrock 转换结果；§k 的连续随机字符区域固定显示一个省略号。
		NBT: nbt.Compound{"front_text": nbt.Compound{
			"messages": []string{"墙上悬挂牌", "§a绿色§r复原", "§o斜体", "§k随机字符§r结束"},
			"color":    "black",
		}},
	},
}
