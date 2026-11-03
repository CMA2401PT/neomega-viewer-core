// 本例生成一个石头方块的 NVMS 网格；在 neomega-viewer-core 目录运行 go run ./examples/mesh。
package main

import (
	"bytes"
	"fmt"
	"log"

	"neomega-blocks/ir"
	"neomega-viewer-core/viewer/cube"
	"neomega-viewer-core/viewer/mesher"
	"neomega-viewer-core/viewer/mesher/geometry"
	preset "neomega-viewer-core/viewer/resources"
)

func main() {
	// 1. 预置表随库内嵌；无需先下载模型或运行资源生成器。
	var resources *preset.Preset
	var err error
	resources, err = preset.Open(preset.OpenOptions{})
	if err != nil {
		log.Fatal(err)
	}

	// 2. 表以规范化 Java 状态的 Hash 为键；石头没有属性，已经是规范形式。
	// MeshBlock 只生成一个局部方块；边长 1 表示无需建立更大的区域布局。
	// 此入口接收单层静态形状；含水状态、NBT 外观与特效应使用 javapreset.NewPipeline 创建的完整渲染管线。
	var stoneID uint32 = ir.IRBlock("minecraft:stone []").Hash()
	var layout cube.Layout = cube.New(1)
	var meshBuilder *mesher.Mesher[uint32] = mesher.NewMesher(layout, resources.Tables)
	var mesh geometry.Mesh
	mesh, err = meshBuilder.MeshBlock(stoneID)
	if err != nil {
		log.Fatal(err)
	}

	// 3. NVMS 是网格二进制格式；同一 Marshal 调用也可写入文件或 HTTP 响应。
	// 这里保存在内存并打印大小，让命令行运行者直接看到结果。
	var binary bytes.Buffer
	if err := mesh.Marshal(&binary); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("石头网格：%d 个四边形，NVMS 编码 %d 字节\n", len(mesh.Quads), binary.Len())
}
