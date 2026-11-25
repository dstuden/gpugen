package kernels

//gpu:kernel
func GaussianBlur(id uint32, w uint32, h uint32, radius int32, weights []float32, src []uint32, dst []uint32) {
	x := int32(id % w)
	y := int32(id / w)
	maxX := int32(w) - 1
	maxY := int32(h) - 1

	var r, g, b, a float32
	for dy := -radius; dy <= radius; dy++ {
		ny := min(max(y+dy, 0), maxY)
		wy := weights[dy+radius]
		for dx := -radius; dx <= radius; dx++ {
			nx := min(max(x+dx, 0), maxX)
			k := wy * weights[dx+radius]
			p := src[ny*int32(w)+nx]
			r += k * float32(p&0xFF)
			g += k * float32((p>>8)&0xFF)
			b += k * float32((p>>16)&0xFF)
			a += k * float32(p>>24)
		}
	}

	dst[id] = min(uint32(r+0.5), 255) |
		min(uint32(g+0.5), 255)<<8 |
		min(uint32(b+0.5), 255)<<16 |
		min(uint32(a+0.5), 255)<<24
}
