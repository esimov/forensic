package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"math"
	"os"
)

func main() {
	const BlockSize int = 16

	type pixel struct {
		r, g, b, y float64
	}
	type dctPx [][]pixel

	var imageSet = make([]dctPx, 0)

	input, err := os.Open("test.jpg")
	defer input.Close()

	if err != nil {
		fmt.Printf("Error reading the image file: %v", err)
	}
	img, _, err := image.Decode(input)
	if err != nil {
		fmt.Printf("Error decoding the image: %v", err)
	}

	newImg := image.NewRGBA(img.Bounds())
	dx, dy := img.Bounds().Max.X, img.Bounds().Max.Y
	bdx, bdy := (dx - BlockSize), (dy - BlockSize)

	for i := 0; i < bdx; i++ {
		for j := 0; j < bdy; j++ {
			r, g, b, _ := img.At(i, j).RGBA()
			yc, u, v := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))

			dctPixels := make(dctPx, BlockSize)

			for x := 0; x < BlockSize; x++ {
				for y := 0; y < BlockSize; y++ {
					dctPixels[x] = make([]pixel, BlockSize)

					br := dct(float64(x), float64(y), float64(i), float64(j), max(bdx, bdy)) * float64(r)
					bg := dct(float64(x), float64(y), float64(i), float64(j), max(bdx, bdy)) * float64(g)
					bb := dct(float64(x), float64(y), float64(i), float64(j), max(bdx, bdy)) * float64(b)
					by := dct(float64(x), float64(y), float64(i), float64(j), max(bdx, bdy)) * float64(yc)

					dctPixels[x][y] = pixel{br, bg, bb, by}
				}
			}
			imageSet = append(imageSet, dctPixels)
			newImg.Set(i, j, color.RGBA{uint8(yc), uint8(u), uint8(v), 255})

			//r1, g1, b1 := color.YCbCrToRGB(y, u, v)
			//newImg.Set(i, j, color.RGBA{uint8(r1>>8), uint8(g1>>8), uint8(b1>>8), 255})
		}
	}
	fmt.Println(imageSet)

	output, err := os.Create("output.png")
	if err != nil {
		fmt.Printf("Error creating output file: %v", err)
	}

	if err := png.Encode(output, newImg); err != nil {
		fmt.Printf("Error encoding image file: %v", err)
	}
}

func RGBtoYUV(r, g, b uint32) (uint32, uint32, uint32) {
	y := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	u := (((float64(b) - float64(y)) * 0.493) + 111) / 222 * 255
	v := (((float64(r) - float64(y)) * 0.877) + 155) / 312 * 255

	return uint32(y), uint32(u), uint32(v)
}

func YUVtoRGB(y, u, v uint32) (uint32, uint32, uint32) {
	r := float64(y) + (1.13983 * float64(v))
	g := float64(y) - (0.39465 * float64(u)) - (0.58060 * float64(v))
	b := float64(y) + (2.03211 * float64(u))

	return uint32(r), uint32(g), uint32(b)
}

func clamp255(x uint32) uint8 {
	if x < 0 {
		return 0
	}
	if x > 255 {
		return 255
	}
	return uint8(x)
}

func dct(x, y, i, j, n float64) float64 {
	// normalization
	alpha := func(a float64) float64 {
		if a == 0 {
			return math.Sqrt(1.0 / n)
		} else {
			return math.Sqrt(2.0 / n)
		}
	}
	return alpha(i) * alpha(j) * math.Cos(((2*x+1)*(i*math.Pi))/(2*n)) * math.Cos(((2*y+1)*(j*math.Pi))/(2*n))
}

// max returns the biggest value between two numbers.
func max(x, y int) float64 {
	if x > y {
		return float64(x)
	}
	return float64(y)
}
