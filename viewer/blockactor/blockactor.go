package blockactor

import (
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/viewer/mesher/geometry"
)

type Input struct {
	Pos         [3]uint8
	State       ir.IRBlock
	NBT         nbt.Compound
	Waterlogged bool
}

// Texture 是处理器生成的私有贴图：从左上角逐行排列的非预乘 RGBA8、sRGB 像素。
// Name 与本 actor 的 Surface.Texture 对应，Region 相对这张图片；不会覆盖其他 actor 的同名贴图。
type Texture struct {
	Name          string
	Width, Height uint32
	Pixels        []byte
}

// Appearance 是格内完整外观；处理器负责状态旋转，通用分发器负责格子平移。
// 没有私有贴图的材质继续使用共享图集，输出在使用期间视为只读。
type Appearance struct {
	Mesh     geometry.Mesh
	Textures []Texture
}

type Actor struct {
	// Pos 只标记归属，Mesh 顶点已经位于子区块局部坐标。
	Pos      [3]uint8
	Mesh     geometry.Mesh
	Textures []Texture
}

// Handler 把支持的方块状态与 NBT 烘焙为完整外观，具体方块知识止于此接口。
type Handler interface {
	BlockNames() []string
	// Build 返回按状态旋转的格内网格与可选图片；格子平移统一由 Dispatcher 执行。
	Build(input Input) (Appearance, error)
}

type Dispatcher struct {
	handlers map[string]Handler
}

func New(handlers ...Handler) *Dispatcher {
	d := &Dispatcher{handlers: map[string]Handler{}}
	for _, handler := range handlers {
		for _, name := range handler.BlockNames() {
			d.handlers[name] = handler
		}
	}
	return d
}

func (d *Dispatcher) BlockNames() []string {
	names := make([]string, 0, len(d.handlers))
	for name := range d.handlers {
		names = append(names, name)
	}
	return names
}

func (d *Dispatcher) Build(inputs []Input) ([]Actor, error) {
	actors := make([]Actor, 0, len(inputs))
	for _, input := range inputs {
		actor, err := d.BuildOne(input)
		if err != nil {
			return nil, err
		}
		actors = append(actors, actor)
	}
	return actors, nil
}

func (d *Dispatcher) BuildOne(input Input) (Actor, error) {
	// 复制后平移一次，处理器可以返回共享的格内模型。
	appearance, err := d.handlers[input.State.NameWithNameSpace()].Build(input)
	if err != nil {
		return Actor{}, err
	}
	placed := geometry.Mesh{Quads: append([]geometry.Quad(nil), appearance.Mesh.Quads...)}
	for i := range placed.Quads {
		for j := range placed.Quads[i].Positions {
			for axis, offset := range input.Pos {
				placed.Quads[i].Positions[j][axis] += float64(offset)
			}
		}
	}
	return Actor{Pos: input.Pos, Mesh: placed, Textures: appearance.Textures}, nil
}
