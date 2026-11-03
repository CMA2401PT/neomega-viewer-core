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

func main() {
	listen := flag.String("listen", "127.0.0.1:2418", "HTTP 监听地址")
	flag.Parse()
	resources, err := preset.Open(preset.OpenOptions{})
	if err != nil {
		log.Fatal(err)
	}
	// 陶罐需要配套图集的区域信息，才能按 NBT 选择四面的纹样。
	atlasJSON, err := fs.ReadFile(preset.Resources(), "blocksAtlases.json")
	if err != nil {
		log.Fatal(err)
	}
	var atlas jmodel.Atlas
	if err := json.Unmarshal(atlasJSON, &atlas); err != nil {
		log.Fatal(err)
	}
	// 构建器固定为 1³；四种表达使用相同的拆层、网格、特效与实体流程。
	layout := cube.New(1)
	pipeline := dispatcher.NewJavaPipeline(layout, resources.Tables, &atlas, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/block", func(w http.ResponseWriter, r *http.Request) {
		scene, found := scenes[r.URL.Query().Get("scene")]
		if !found {
			http.Error(w, "未知场景", http.StatusNotFound)
			return
		}
		// 构建器约定 N=1，因此输入只分配一格；尺寸不随每份数据重复传递。
		blocks := blockdata.Uniform(layout.Len(), scene.State.Tidy())
		result, err := pipeline.Build(
			dispatcher.Section{
				Blocks:   blocks,
				Entities: map[dispatcher.CellPos]nbt.Compound{{0, 0, 0}: scene.NBT},
			},
			dispatcher.Neighbours{},
			dispatcher.Options{},
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, diagnostic := range result.Diagnostics {
			log.Printf("%s: %s (%v)", diagnostic.Kind, diagnostic.State, diagnostic.Err)
		}
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
	mux.Handle("GET /textures/", http.StripPrefix("/textures/", http.FileServerFS(preset.Resources())))
	mux.Handle("GET /", http.FileServer(http.Dir("dist")))
	log.Printf("http://%s", *listen)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

// 每个场景只有一个完整 Java 状态；只有陶罐需要额外的实体 NBT。
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
}
