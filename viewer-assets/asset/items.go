package asset

import model "neomega-viewer-core/viewer/geometry/java_block_model"

// ItemModels 保留 items 定义中的真实模型引用；不能从物品名称推测模型路径。
type ItemModels struct {
	Models      map[string]model.ResourceID `json:"models"`
	Tints       map[string][]uint32         `json:"tints,omitempty"`
	Unsupported map[string]string           `json:"unsupported"`
}
