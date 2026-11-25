package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
)

type runner func(w, h uint32, weights []float32, src, dst []uint32) error

var runners = map[string]runner{
	"parallel-cpu": blurParallel,
}

func blurParallel(w, h uint32, weights []float32, src, dst []uint32) error {
	radius := int32(len(weights) / 2)
	n := int(w * h)
	order := rand.Perm(n)
	workers := min(runtime.GOMAXPROCS(0), n)
	var wg sync.WaitGroup
	for i := range workers {
		chunk := order[i*n/workers : (i+1)*n/workers]
		wg.Go(func() {
			for _, id := range chunk {
				GaussianBlur(uint32(id), w, h, radius, weights, src, dst)
			}
		})
	}
	wg.Wait()
	return nil
}

type testImage struct {
	name string
	w, h uint32
	pix  []uint32
}

func testImages(t *testing.T) []testImage {
	t.Helper()
	rng := rand.New(rand.NewPCG(1, 2))
	noise := func(w, h uint32) []uint32 {
		p := make([]uint32, w*h)
		for i := range p {
			p[i] = rng.Uint32()
		}
		return p
	}
	imgs := []testImage{
		{"noise-1x1", 1, 1, noise(1, 1)},
		{"noise-1x50", 1, 50, noise(1, 50)},
		{"noise-50x1", 50, 1, noise(50, 1)},
		{"noise-37x23", 37, 23, noise(37, 23)},
		{"noise-129x65", 129, 65, noise(129, 65)},
		{"pattern-256x256", 256, 256, pixels(testPattern(256, 256))},
	}

	var files []string
	for _, pat := range []string{"*.png", "*.jpg", "*.jpeg"} {
		m, _ := filepath.Glob(filepath.Join("testdata", pat))
		files = append(files, m...)
	}
	for _, f := range files {
		r, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(r)
		r.Close()
		if err != nil {
			t.Fatalf("decode %s: %v", f, err)
		}
		rgba := toRGBA(img)
		imgs = append(imgs, testImage{
			name: filepath.Base(f),
			w:    uint32(rgba.Rect.Dx()),
			h:    uint32(rgba.Rect.Dy()),
			pix:  pixels(rgba),
		})
	}
	return imgs
}

func TestRunnersMatchCPU(t *testing.T) {
	names := slices.Sorted(maps.Keys(runners))
	for _, img := range testImages(t) {
		for _, sigma := range []float64{0, 1, 2} {
			weights := gaussianWeights(sigma)
			want := make([]uint32, len(img.pix))
			blurCPU(img.w, img.h, weights, img.pix, want)

			for _, name := range names {
				t.Run(fmt.Sprintf("%s/%s/sigma%g", name, img.name, sigma), func(t *testing.T) {
					src := slices.Clone(img.pix)
					got := make([]uint32, len(src))
					if err := runners[name](img.w, img.h, weights, src, got); err != nil {
						t.Fatal(err)
					}
					comparePixels(t, img.w, img.h, got, want, 1)
					if !slices.Equal(src, img.pix) {
						t.Error("runner modified the input image")
					}
				})
			}
		}
	}
}

func comparePixels(t *testing.T, w, h uint32, got, want []uint32, tol int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d pixels, want %d", len(got), len(want))
	}
	bad, worst := 0, 0
	for i := range want {
		d := channelDiff(got[i], want[i])
		if d <= tol {
			continue
		}
		bad++
		worst = max(worst, d)
		if bad <= 5 {
			t.Errorf("pixel (%d,%d): got %08x, want %08x", uint32(i)%w, uint32(i)/w, got[i], want[i])
		}
	}
	if bad == 0 {
		return
	}
	t.Errorf("%d of %d pixels differ by more than %d (worst %d)", bad, len(want), tol, worst)
	if path, err := writeDiff(w, h, got, want, tol); err == nil {
		t.Logf("diff image: %s", path)
	}
}

func channelDiff(a, b uint32) int {
	d := 0
	for shift := 0; shift < 32; shift += 8 {
		ca, cb := int(a>>shift&0xFF), int(b>>shift&0xFF)
		d = max(d, ca-cb, cb-ca)
	}
	return d
}

func writeDiff(w, h uint32, got, want []uint32, tol int) (string, error) {
	img := image.NewGray(image.Rect(0, 0, int(w), int(h)))
	for i := range want {
		if channelDiff(got[i], want[i]) > tol {
			img.SetGray(int(uint32(i)%w), int(uint32(i)/w), color.Gray{Y: 255})
		}
	}
	f, err := os.CreateTemp("", "blur-diff-*.png")
	if err != nil {
		return "", err
	}
	defer f.Close()
	return f.Name(), png.Encode(f, img)
}
