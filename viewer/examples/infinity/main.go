package main

import (
	"bytes"
	"flag"
	"log"
	"net/http"
	"strconv"

	"neomega-viewer-core/viewer/dispatcher"
	"neomega-viewer-core/viewer/javapreset"
	"neomega-viewer-core/viewer/merger"
	preset "neomega-viewer-core/viewer/resources"
)

func main() {
	var listen *string = flag.String("listen", "127.0.0.1:2416", "HTTP 监听地址")
	flag.Parse()
	// 1. 加载随库内嵌的 Java 几何表；Tables 用状态 ID 查找形状和材质。
	var resources *preset.Preset
	var err error
	resources, err = preset.Open(preset.OpenOptions{})
	if err != nil {
		log.Fatal(err)
	}
	// 2. Java 预设组装方块实体和展示框处理器，处理器自行加载各自内嵌的图片。
	// layout 在 world.go 中固定为 cube.New(16)，数据来源和构建器共用它。
	// 最后一个 nil 表示没有缺失模型的替代形状，遇到未知模型会返回错误。
	// Items 提供展示框所用的普通物品几何，必须与 Tables 使用同一套图集。
	pipeline, err := javapreset.NewPipeline(layout, resources.Tables, resources.Items, nil)
	if err != nil {
		log.Fatal(err)
	}
	// 3. 每个请求顺序完成数据构造、网格生成与编码。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/section", func(w http.ResponseWriter, r *http.Request) {
		// 请求参数是区域网格坐标，世界方块原点为 pos × layout.Side()；此例只生成 Y=0。
		var pos dispatcher.SectionPos
		for name, target := range map[string]*int32{"x": &pos.X, "z": &pos.Z} {
			var value int64
			var err error
			value, err = strconv.ParseInt(r.URL.Query().Get(name), 10, 32)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			*target = int32(value)
		}
		// generatedWorld 在 world.go 按坐标生成方块；BuildSections 额外读取邻域，用于边界剔面与流体高度判断。
		// 返回值只包含请求的位置，顺序与输入一致，所以这里取唯一的 results[0]。
		var results []dispatcher.PositionedResult
		var err error
		results, err = pipeline.BuildSections(
			r.Context(),
			generatedWorld{},
			[]dispatcher.SectionPos{pos},
			dispatcher.Options{}, // 默认启用剔面与相邻面合并。
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var result *dispatcher.Result = results[0].Result
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
