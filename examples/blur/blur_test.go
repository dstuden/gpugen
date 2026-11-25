package main

import (
	"image"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func blur(t *testing.T, w, h uint32, weights []float32, src []uint32) []uint32 {
	t.Helper()
	if uint32(len(src)) != w*h {
		t.Fatalf("src has %d pixels, want %d", len(src), w*h)
	}
	dst := make([]uint32, len(src))
	blurCPU(w, h, weights, src, dst)
	return dst
}

func rgba(r, g, b, a uint32) uint32 { return r | g<<8 | b<<16 | a<<24 }

var binomial = []float32{0.25, 0.5, 0.25}

func TestGaussianWeights(t *testing.T) {
	if got := gaussianWeights(0); !slices.Equal(got, []float32{1}) {
		t.Errorf("sigma 0: got %v, want [1]", got)
	}
	ws := gaussianWeights(2)
	if len(ws) != 13 {
		t.Fatalf("sigma 2: got %d weights, want 13", len(ws))
	}
	var sum float64
	for i, w := range ws {
		sum += float64(w)
		if w != ws[len(ws)-1-i] {
			t.Errorf("not symmetric at %d", i)
		}
		if i > 0 && i <= len(ws)/2 && w <= ws[i-1] {
			t.Errorf("not increasing towards the center at %d", i)
		}
	}
	if math.Abs(sum-1) > 1e-6 {
		t.Errorf("weights sum to %v, want 1", sum)
	}
}

func TestNoBlurIsIdentity(t *testing.T) {
	src := make([]uint32, 7*5)
	for i := range src {
		src[i] = rand.Uint32()
	}
	if got := blur(t, 7, 5, gaussianWeights(0), src); !slices.Equal(got, src) {
		t.Errorf("sigma 0 changed the image")
	}
}

func TestUniformImageUnchanged(t *testing.T) {
	for _, c := range []uint32{rgba(12, 34, 56, 78), rgba(255, 255, 255, 255)} {
		src := slices.Repeat([]uint32{c}, 9*6)
		for i, p := range blur(t, 9, 6, gaussianWeights(1.5), src) {
			if p != c {
				t.Fatalf("pixel %d = %#08x, want %#08x", i, p, c)
			}
		}
	}
}

func TestSinglePixelSpreads(t *testing.T) {
	const w, h = 5, 5
	black, white := rgba(0, 0, 0, 255), rgba(255, 255, 255, 255)
	src := slices.Repeat([]uint32{black}, w*h)
	src[2*w+2] = white

	gray := func(v uint32) uint32 { return rgba(v, v, v, 255) }
	got := blur(t, w, h, binomial, src)
	for y := range uint32(h) {
		for x := range uint32(w) {
			want := black
			switch dx, dy := absDiff(x, 2), absDiff(y, 2); {
			case dx == 0 && dy == 0:
				want = gray(64)
			case dx+dy == 1:
				want = gray(32)
			case dx == 1 && dy == 1:
				want = gray(16)
			}
			if p := got[y*w+x]; p != want {
				t.Errorf("(%d,%d) = %#08x, want %#08x", x, y, p, want)
			}
		}
	}
}

func absDiff(a, b uint32) uint32 { return max(a, b) - min(a, b) }

func TestEdgesClamp(t *testing.T) {
	src := []uint32{rgba(0, 0, 0, 0), rgba(0, 0, 0, 0), rgba(90, 0, 0, 0)}
	got := blur(t, 3, 1, binomial, src)
	want := []uint32{rgba(0, 0, 0, 0), rgba(23, 0, 0, 0), rgba(68, 0, 0, 0)}
	if !slices.Equal(got, want) {
		t.Errorf("got %#08x, want %#08x", got, want)
	}
}

func TestPixelsLayout(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	copy(img.Pix, []uint8{1, 2, 3, 4})
	if got := pixels(img)[0]; got != rgba(1, 2, 3, 4) {
		t.Errorf("pixels() = %#08x, want %#08x", got, rgba(1, 2, 3, 4))
	}
}
