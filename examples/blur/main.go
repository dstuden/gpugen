package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	"time"
	"unsafe"
)

func main() {
	in := flag.String("in", "", "input PNG or JPEG (default: test pattern)")
	out := flag.String("out", "blurred.png", "output PNG")
	sigma := flag.Float64("sigma", 3, "Gaussian standard deviation in pixels")
	flag.Parse()

	if err := run(*in, *out, *sigma); err != nil {
		log.Fatal(err)
	}
}

func run(in, out string, sigma float64) error {
	if sigma < 0 {
		return errors.New("sigma must be >= 0")
	}

	var img image.Image = testPattern(512, 512)
	if in != "" {
		f, err := os.Open(in)
		if err != nil {
			return err
		}
		img, _, err = image.Decode(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("decode %s: %w", in, err)
		}
	}

	src := toRGBA(img)
	dst := image.NewRGBA(src.Rect)
	w, h := uint32(src.Rect.Dx()), uint32(src.Rect.Dy())
	if w == 0 || h == 0 {
		return errors.New("image is empty")
	}

	weights := gaussianWeights(sigma)
	start := time.Now()
	blurCPU(w, h, weights, pixels(src), pixels(dst))
	fmt.Printf("blurred %dx%d, sigma %g (radius %d) in %v\n", w, h, sigma, len(weights)/2, time.Since(start))

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := png.Encode(f, dst); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func blurCPU(w, h uint32, weights []float32, src, dst []uint32) {
	radius := int32(len(weights) / 2)
	for id := range w * h {
		GaussianBlur(id, w, h, radius, weights, src, dst)
	}
}

func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Rect, img, b.Min, draw.Src)
	return dst
}

// pixels views img.Pix as one uint32 per pixel without copying.
func pixels(img *image.RGBA) []uint32 {
	if len(img.Pix) == 0 {
		return nil
	}
	return unsafe.Slice((*uint32)(unsafe.Pointer(&img.Pix[0])), len(img.Pix)/4)
}

func testPattern(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	colors := []color.RGBA{
		{230, 57, 70, 255}, {241, 250, 238, 255},
		{69, 123, 157, 255}, {29, 53, 87, 255},
	}
	for y := range h {
		for x := range w {
			c := colors[((x/32)+(y/32)*2)%len(colors)]
			if (x+y)%64 < 2 {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}
