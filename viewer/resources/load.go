package geometry

import (
	"encoding/json"
	"os"
	"sync"

	core "neomega-viewer-core/viewer/geometry"
)

type OpenOptions struct {
	// 留空使用随库分发的表，外部表的 Java 元数据来自同名 .java.json 文件。
	TablesPath string
}

// Preset contains shared read-only geometry and its Java resource metadata.
type Preset struct {
	Tables   *core.Tables[uint32]
	Metadata Metadata
}

var bundledTables = sync.OnceValues(func() (*core.Tables[uint32], error) {
	// 多个演示入口共享同一份解码结果。
	file, err := Files.Open("geometry.nvtables")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return core.DecodeTables[uint32](file)
})

func Open(options OpenOptions) (*Preset, error) {
	var tables *core.Tables[uint32]
	var metadata Metadata
	var err error
	if options.TablesPath == "" {
		tables, err = bundledTables()
	} else {
		file, openErr := os.Open(options.TablesPath)
		if openErr != nil {
			return nil, openErr
		}
		defer file.Close()
		tables, err = core.DecodeTables[uint32](file)
	}
	if err != nil {
		return nil, err
	}
	var data []byte
	if options.TablesPath == "" {
		data, err = Files.ReadFile("geometry.nvtables.java.json")
	} else {
		data, err = os.ReadFile(options.TablesPath + ".java.json")
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &Preset{Tables: tables, Metadata: metadata}, nil
}
