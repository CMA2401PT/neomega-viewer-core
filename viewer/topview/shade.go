package topview

import "image/color"

func Shade(pixels []color.NRGBA, heights []int32, width int) {
	// 原地添加坡向与海拔明暗，保留透明度；空格和图外位置不提供坡度。
	// 两个切片必须等长，按 z*width+x 排列完整矩形，width 必须为正。
	// 每张区域图独立调用时无法取得相邻区域高度，接缝仍留在区域边上。
	height := len(pixels) / width
	for z := range height {
		for x := range width {
			index := z*width + x
			if pixels[index].A == 0 {
				continue
			}
			amount := shadeAmount(pixels, heights, width, height, x, z)
			pixel := &pixels[index]
			pixel.R = uint8(max(0, min(255, int(pixel.R)+amount)))
			pixel.G = uint8(max(0, min(255, int(pixel.G)+amount)))
			pixel.B = uint8(max(0, min(255, int(pixel.B)+amount)))
		}
	}
}

func shadeAmount(pixels []color.NRGBA, heights []int32, width, height, x, z int) int {
	// 用当前高度替代缺失邻居，避免把未生成区块当作海拔 0 的悬崖。
	index := z*width + x
	here := float64(heights[index])
	west, east, north, south := here, here, here, here
	if x > 0 && pixels[index-1].A != 0 {
		west = float64(heights[index-1])
	}
	if x+1 < width && pixels[index+1].A != 0 {
		east = float64(heights[index+1])
	}
	if z > 0 && pixels[index-width].A != 0 {
		north = float64(heights[index-width])
	}
	if z+1 < height && pixels[index+width].A != 0 {
		south = float64(heights[index+width])
	}
	xShade, zShade := east-west, south-north
	// 内部使用双侧高度差加强地形起伏；图边保持单侧差值。
	if x > 0 && x < width-1 {
		xShade *= 2
	}
	if z > 0 && z < height-1 {
		zShade *= 2
	}
	slope := max(-8, min(8, xShade+zShade))
	// 海平面 64 为基准，海拔每升高 256 格增加 12 级，避免高山过曝。
	altitude := max(-4, min(12, (here-64)*(12.0/256)))
	return int((slope + altitude) * 8)
}
