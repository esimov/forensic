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
		r, g, b, y uint8
	}
	type dctPx [][]pixel

	var (
		br, bg, bb, by float64
	)

	input, err := os.Open("test.jpg")
	defer input.Close()

	if err != nil {
		fmt.Printf("Error reading the image file: %v", err)
	}
	img, _, err := image.Decode(input)
	if err != nil {
		fmt.Printf("Error decoding the image: %v", err)
	}



	in := [][]float64{
		{0, 1, 2, 3, 4, 5, 6, 7},
	};
	dcta := make([][]float64, len(in))

	alpha := func(a float64) float64 {
		if a == 0 {
			return math.Sqrt(1.0 / 8)
		} else {
			return math.Sqrt(2.0 / 8)
		}
	}
	var res float64
	for u := 0; u < len(in); u++ {
		dcta[u] = make([]float64, len(in[0]))
		for v := 0; v < len(in[0]); v++ {
			for x := 0; x < len(in); x++ {
				for y := 0; y < len(in[0]); y++ {
					//fmt.Println(in[y][x])
					res += dct(float64(u), float64(v), float64(x), float64(y), float64(len(in)), float64(len(in[0]))) * in[x][y]
				}
			}
			dcta[u][v] = res * alpha(float64(u)) * alpha(float64(v))
		}
	}
	fmt.Println(dcta)
	idcta := make([][]float64, len(in))
	for x := 0; x < len(in); x++ {
		idcta[x] = make([]float64, len(in[0]))
		for y := 0; y < len(in[0]); y++ {
			for u := 0; u < len(in); u++ {
				for v := 0; v < len(in[0]); v++ {
					//fmt.Println(in[y][x])
					res += idct(float64(u), float64(v), float64(x), float64(y), float64(len(in)), float64(len(in[0]))) * dcta[u][v]
				}
			}
			idcta[x][y] = res * (1.0/8.0 + 1.0/1.0)
		}
	}
	fmt.Println(idcta)
	os.Exit(2)

	newImg := image.NewRGBA(img.Bounds())
	dx, dy := img.Bounds().Max.X, img.Bounds().Max.Y
	bdx, bdy := (dx - BlockSize + 1), (dy - BlockSize + 1)

	dctPixels := make(dctPx, dx*dy)

	for u := 0; u < dx; u++ {
		dctPixels[u] = make([]pixel, dy)
		for v := 0; v < dy; v++ {
			r, g, b, _ := img.At(u, v).RGBA()
			y, uc, vc := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			for x := 0; x < bdx; x++ {
				for y := 0; y < bdy; y++ {
					r, g, b, _ := img.At(x, y).RGBA()
					yc, _, _ := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))

					br += dct(float64(x), float64(y), float64(u), float64(v), float64(dx), float64(dy)) * float64(r)
					bg += dct(float64(x), float64(y), float64(u), float64(v), float64(dx), float64(dy)) * float64(g)
					bb += dct(float64(x), float64(y), float64(u), float64(v), float64(dx), float64(dy)) * float64(b)
					by += dct(float64(x), float64(y), float64(u), float64(v), float64(dx), float64(dy)) * float64(yc)
				}
			}
			// normalization
			alpha := func(a float64) float64 {
				if a == 0 {
					return math.Sqrt(1.0 / float64(dx))
				} else {
					return math.Sqrt(2.0 / float64(dy))
				}
			}

			fi, fj := float64(u), float64(v)
			br *= alpha(fi) * alpha(fj)
			bg *= alpha(fi) * alpha(fj)
			bb *= alpha(fi) * alpha(fj)
			by *= alpha(fi) * alpha(fj)

			dctPixels[u][v] = pixel{uint8(br), uint8(bg), uint8(bb), uint8(by)}
			newImg.Set(u, v, color.RGBA{uint8(y), uint8(uc), uint8(vc), 255})
		}
	}
	fmt.Println(dctPixels)

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

func clamp255(x float64) uint8 {
	if x < 0 {
		return 0
	}
	if x > 255 {
		return 255
	}
	return uint8(x)
}

func dct(x, y, u, v, w, h float64) float64 {
	a := math.Cos(((2.0*x+1)*(u*math.Pi))/(2*w))
	b := math.Cos(((2.0*y+1)*(v*math.Pi))/(2*h))

	return a * b
}

func idct(u, v, x, y, w, h float64) float64 {
	// normalization
	alpha := func(a float64) float64 {
		if a == 0 {
			return 1.0 / math.Sqrt(2.0)
		}
		return 1.0
	}

	return dct(u, v, x, y, w, h) * alpha(u) * alpha(v)
}

// max returns the biggest value between two numbers.
func max(x, y int) float64 {
	if x > y {
		return float64(x)
	}
	return float64(y)
}