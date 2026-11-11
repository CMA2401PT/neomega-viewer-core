// Package static_orthogonal 用固定正交投影的方块贴图生成有深度的图片瓦片。
// 调用方负责读取方块、划分瓦片和保存图片；核心不持有世界或调度队列。
package static_orthogonal

import (
	"fmt"
	"image"
	"math"
)

const phases = 16

// Camera 指定相机朝向和每个世界单位的像素数。Yaw=0 从 +Z 看向原点，
// Pitch=90 为正俯视；角度单位为度。Scale=1 时，每个方块只绘制一个中心像素点。
// Scale 大于 1 时绘制立方体贴图，建议为 2 到 16。
type Camera struct{ Yaw, Pitch, Scale float64 }

// Stamp 是方块在整张画布中的位置、深度与不透明颜色。
// U/V 使用 1/16 像素单位，避免每块独立取整造成接缝。
type Stamp struct {
	U, V  int64
	Depth float64
	RGB   uint32
}

type sample struct {
	depth float64
	face  int
}
type sprite struct {
	rect   image.Rectangle
	pixels []sample
}

// Atlas 保存固定相机的覆盖掩码与相对深度；所有颜色共享同一套形状贴图。
// 创建后只读，可以供多个瓦片渲染协程共享。
type Atlas struct {
	point   bool
	shift   uint
	u, v    [3]int64
	depth   [3]float64
	sprites [phases * phases]sprite
	shade   [3]uint32
}

// NewAtlas 为立方体预绘制全部亚像素相位，投影基向量量化到 1/16 像素。
// Scale=1 则只准备高精度点投影，不生成形状贴图。
// 同一世界顶点始终落在同一位置；旋转、相交计算只发生在建表时。
func NewAtlas(camera Camera) (*Atlas, error) {
	if math.IsNaN(camera.Scale+camera.Yaw+camera.Pitch) || math.IsInf(camera.Scale+camera.Yaw+camera.Pitch, 0) || camera.Scale < 1 || camera.Scale > 64 || camera.Pitch <= 0 || camera.Pitch > 90 {
		return nil, fmt.Errorf("比例须为 1…64，俯角须为 (0,90]，相机参数必须有限")
	}
	yaw, pitch := camera.Yaw*math.Pi/180, camera.Pitch*math.Pi/180
	r := [3]float64{math.Cos(yaw), 0, -math.Sin(yaw)}
	d := [3]float64{math.Sin(yaw) * math.Sin(pitch), -math.Cos(pitch), math.Cos(yaw) * math.Sin(pitch)}
	a := &Atlas{point: camera.Scale == 1, shift: 4, shade: [3]uint32{205, 255, 165}}
	// 点模式没有贴图相位表，可以用更高的投影精度，避免低尺度改变观察角度。
	if a.point {
		a.shift = 16
	}
	units := float64(int64(1) << a.shift)
	for i := range 3 {
		a.u[i] = int64(math.Round(r[i] * camera.Scale * units))
		a.v[i] = int64(math.Round(d[i] * camera.Scale * units))
		r[i] = float64(a.u[i]) / units
		d[i] = float64(a.v[i]) / units
	}
	if a.u[0]*a.v[2]-a.u[2]*a.v[0] == 0 {
		return nil, fmt.Errorf("当前比例下俯角太小，请增大 pitch 或 scale")
	}
	// d×r 指向观察者。逆矩阵给出屏幕射线在深度零平面上的起点。
	n := cross(d, r)
	length := math.Sqrt(dot(n, n))
	for i := range 3 {
		n[i] /= length
	}
	a.depth = n
	if a.point {
		return a, nil
	}
	du, dv := cross(d, n), cross(n, r)
	det := dot(r, du)
	for i := range 3 {
		du[i] /= det
		dv[i] /= det
	}
	for py := range phases {
		for px := range phases {
			minU, maxU, minV, maxV := float64(px)/phases, float64(px)/phases, float64(py)/phases, float64(py)/phases
			for i := range 3 {
				minU += math.Min(0, r[i])
				maxU += math.Max(0, r[i])
				minV += math.Min(0, d[i])
				maxV += math.Max(0, d[i])
			}
			s := &a.sprites[py*phases+px]
			s.rect = image.Rect(int(math.Floor(minU)), int(math.Floor(minV)), int(math.Ceil(maxU)), int(math.Ceil(maxV)))
			s.pixels = make([]sample, s.rect.Dx()*s.rect.Dy())
			for y := s.rect.Min.Y; y < s.rect.Max.Y; y++ {
				for x := s.rect.Min.X; x < s.rect.Max.X; x++ {
					p := &s.pixels[(y-s.rect.Min.Y)*s.rect.Dx()+x-s.rect.Min.X]
					p.face = -1
					// 给恰落在共边上的像素固定归属，避免射线沿对角缝看见已剔除的内面。
					// 偏移远小于子像素精度，所有贴图相位使用同一采样规则。
					u, v := float64(x)+0.5-float64(px)/phases+1e-7, float64(y)+0.5-float64(py)/phases+math.Sqrt2*1e-7
					near, far, face := math.Inf(-1), math.Inf(1), -1
					for i := range 3 {
						o := du[i]*u + dv[i]*v
						if n[i] == 0 {
							if o < 0 || o >= 1 {
								near = math.Inf(1)
							}
							continue
						}
						t0, t1 := -o/n[i], (1-o)/n[i]
						if t0 > t1 {
							t0, t1 = t1, t0
						}
						near = math.Max(near, t0)
						if t1 < far {
							far, face = t1, i
						}
					}
					// 擦过棱或角的射线没有覆盖面积；舍去浮点求交产生的极小伪线段。
					if far-near > 1e-9 && face >= 0 {
						p.depth, p.face = far, face
					}
				}
			}
		}
	}
	return a, nil
}

// Project 将世界坐标变为贴图锚点。RGB 的低 24 位为 0xRRGGBB。
func (a *Atlas) Project(x, y, z int32, rgb uint32) Stamp {
	p := [3]int64{int64(x), int64(y), int64(z)}
	s := Stamp{RGB: rgb}
	for i := range 3 {
		s.U += p[i] * a.u[i]
		s.V += p[i] * a.v[i]
		s.Depth += float64(p[i]) * a.depth[i]
	}
	if a.point {
		// 点模式使用方块中心所在的像素，仍以 1/16 像素单位交付锚点。
		s.U = ((2*s.U + a.u[0] + a.u[1] + a.u[2]) >> (a.shift + 1)) << 4
		s.V = ((2*s.V + a.v[0] + a.v[1] + a.v[2]) >> (a.shift + 1)) << 4
	}
	return s
}

// Bounds 返回方块贴图的全局像素范围，供调用方分发到相交瓦片。
func (a *Atlas) Bounds(s Stamp) image.Rectangle {
	if a.point {
		x, y := int(s.U>>4), int(s.V>>4)
		return image.Rect(x, y, x+1, y+1)
	}
	return a.sprites[(s.V&15)*phases+(s.U&15)].rect.Add(image.Pt(int(s.U>>4), int(s.V>>4)))
}

// Facing 返回朝向观察者的三个邻格方向，供调用方剔除内部方块。
func (a *Atlas) Facing() [3]int {
	var out [3]int
	for i, n := range a.depth {
		if n > 0 {
			out[i] = 1
		} else if n < 0 {
			out[i] = -1
		}
	}
	return out
}

// BoxBounds 返回一个半开世界包围盒的保守像素范围，用于跳过不相交区块。
func (a *Atlas) BoxBounds(low, high [3]int32) image.Rectangle {
	minU, minV, maxU, maxV := int64(0), int64(0), int64(0), int64(0)
	for i := range 3 {
		x0, x1 := int64(low[i])*a.u[i], int64(high[i])*a.u[i]
		y0, y1 := int64(low[i])*a.v[i], int64(high[i])*a.v[i]
		minU += min(x0, x1)
		maxU += max(x0, x1)
		minV += min(y0, y1)
		maxV += max(y0, y1)
	}
	units := int64(1) << a.shift
	return image.Rect(int(minU>>a.shift), int(minV>>a.shift), int((maxU+units-1)>>a.shift), int((maxV+units-1)>>a.shift))
}

// Footprint 将图片范围和高度范围反投影为水平世界范围。
// 调用方只需读取其中的区块，不必建立随世界增长的全图空间索引。
func (a *Atlas) Footprint(rect image.Rectangle, lowY, highY int32) image.Rectangle {
	det := float64(a.u[0]*a.v[2] - a.u[2]*a.v[0])
	units := float64(int64(1) << a.shift)
	minX, minZ, maxX, maxZ := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, u := range []int{rect.Min.X, rect.Max.X} {
		for _, v := range []int{rect.Min.Y, rect.Max.Y} {
			for _, y := range []int32{lowY, highY} {
				ru := float64(u)*units - float64(y)*float64(a.u[1])
				rv := float64(v)*units - float64(y)*float64(a.v[1])
				x := (ru*float64(a.v[2]) - rv*float64(a.u[2])) / det
				z := (rv*float64(a.u[0]) - ru*float64(a.v[0])) / det
				minX = math.Min(minX, x)
				maxX = math.Max(maxX, x)
				minZ = math.Min(minZ, z)
				maxZ = math.Max(maxZ, z)
			}
		}
	}
	return image.Rect(int(math.Floor(minX)), int(math.Floor(minZ)), int(math.Ceil(maxX)), int(math.Ceil(maxZ)))
}

func cross(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func dot(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
