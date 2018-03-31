package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"math"
	"os"
	"time"
)

func main() {
	const BlockSize int = 8

	type pixel struct {
		r, g, b, y float64
	}

	type dctPx [][]pixel

	var (
		features       []float64
		cr, cg, cb, cy float64
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

	start := time.Now()

	// Convert image to YUV color space
	yuv := convertRGBImageToYUV(img)
	newImg := image.NewRGBA(yuv.Bounds())
	draw.Draw(newImg, image.Rect(0, 0, yuv.Bounds().Dx(), yuv.Bounds().Dy()), yuv, image.ZP, draw.Src)

	dx, dy := yuv.Bounds().Max.X, yuv.Bounds().Max.Y
	bdx, bdy := (dx - BlockSize + 1), (dy - BlockSize + 1)

	var blocks []image.Image
	for i := 0; i < bdx; i++ {
		for j := 0; j < bdy; j++ {
			r := image.Rect(i, j, i+BlockSize, j+BlockSize)
			block := newImg.SubImage(r).(*image.RGBA)
			blocks = append(blocks, block)
			draw.Draw(newImg, image.Rect(0, 0, yuv.Bounds().Max.X, yuv.Bounds().Max.Y), block, image.ZP, draw.Src)
		}
	}

	fmt.Printf("Len: %d", len(blocks))

	out, err := os.Create("output.png")
	if err != nil {
		fmt.Printf("Error creating output file: %v", err)
	}

	if err := png.Encode(out, newImg); err != nil {
		fmt.Printf("Error encoding image file: %v", err)
	}

	// Average Red, Green and Blue
	var avr, avg, avb float64

	for _, block := range blocks {
		b := block.(*image.RGBA)
		i0 := b.PixOffset(b.Bounds().Min.X, b.Bounds().Min.Y)
		i1 := i0 + b.Bounds().Dx()*4

		dctPixels := make(dctPx, BlockSize*BlockSize)
		for u := 0; u < BlockSize; u++ {
			dctPixels[u] = make([]pixel, BlockSize)
			for v := 0; v < BlockSize; v++ {
				for i := i0; i < i1; i += 4 {
					// Get the YUV converted image pixels
					yc, uc, vc, _ := b.Pix[i+0], b.Pix[i+2], b.Pix[i+2], b.Pix[i+3]
					// Convert YUV to RGB and obtain the R value
					r, g, b := color.YCbCrToRGB(yc, uc, vc)

					for x := 0; x < BlockSize; x++ {
						for y := 0; y < BlockSize; y++ {
							// Compute Discrete Cosine coefficients
							cr += dct(float64(x), float64(y), float64(u), float64(v), float64(BlockSize)) * float64(r)
							cg += dct(float64(x), float64(y), float64(u), float64(v), float64(BlockSize)) * float64(g)
							cb += dct(float64(x), float64(y), float64(u), float64(v), float64(BlockSize)) * float64(b)
							cy += dct(float64(x), float64(y), float64(u), float64(v), float64(BlockSize)) * float64(yc)
						}
					}
					// Compute the average of R,G,B.
					avr += float64(r)
					avg += float64(g)
					avb += float64(b)
				}

				// normalization
				alpha := func(a float64) float64 {
					if a == 0 {
						return math.Sqrt(1.0 / float64(dx))
					}
					return math.Sqrt(2.0 / float64(dy))
				}

				fi, fj := float64(u), float64(v)
				cr *= alpha(fi) * alpha(fj)
				cg *= alpha(fi) * alpha(fj)
				cb *= alpha(fi) * alpha(fj)
				cy *= alpha(fi) * alpha(fj)

				dctPixels[u][v] = pixel{cr, cg, cb, cy}
			}
		}
		avr /= float64(BlockSize * BlockSize)
		avg /= float64(BlockSize * BlockSize)
		avb /= float64(BlockSize * BlockSize)

		features = append(features, dctPixels[0][0].y)
		features = append(features, dctPixels[0][1].y)
		features = append(features, dctPixels[1][0].y)
		features = append(features, dctPixels[0][0].r)
		features = append(features, dctPixels[0][0].g)
		features = append(features, dctPixels[0][0].b)

		// Append average red, green and blue values
		features = append(features, avr)
		features = append(features, avb)
		features = append(features, avg)
	}

	fmt.Println(features)

	fmt.Printf("\nDone in: %.2fs\n", time.Since(start).Seconds())
	os.Exit(2)

	/*dctPixels := make(dctPx, bdx*bdy)
	for x := 0; x < bdx; x++ {
		dctPixels[x] = make([]pixel, bdy)
		for y := 0; y < bdy; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			yc, uc, vc := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))

			for i := 0; i < BlockSize-1; i++ {
				for j := 0; j < BlockSize-1; j++ {
					r, g, b, _ := img.At(x + i, y + j).RGBA()
					yc, _, _ := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))

					// Compute Discrete Cosine coefficients
					cr += dct(float64(i), float64(j), float64(x), float64(y), float64(dx), float64(dy)) * float64(r)
					cg += dct(float64(i), float64(j), float64(x), float64(y), float64(dx), float64(dy)) * float64(g)
					cb += dct(float64(i), float64(j), float64(x), float64(y), float64(dx), float64(dy)) * float64(b)
					cy += dct(float64(i), float64(j), float64(x), float64(y), float64(dx), float64(dy)) * float64(yc)
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

			fi, fj := float64(x), float64(y)
			cr *= alpha(fi) * alpha(fj)
			cg *= alpha(fi) * alpha(fj)
			cb *= alpha(fi) * alpha(fj)
			cy *= alpha(fi) * alpha(fj)

			dctPixels[x][y] = pixel{cr, cg, cb, cy}
			newImg.Set(x, y, color.RGBA{uint8(yc), uint8(uc), uint8(vc), 255})
		}
	}
	fmt.Println(len(dctPixels))

	output, err := os.Create("output.png")
	if err != nil {
		fmt.Printf("Error creating output file: %v", err)
	}

	if err := png.Encode(output, newImg); err != nil {
		fmt.Printf("Error encoding image file: %v", err)
	}*/
}

func convertRGBImageToYUV(img image.Image) image.Image {
	bounds := img.Bounds()
	dx, dy := bounds.Max.X, bounds.Max.Y

	yuvImage := image.NewRGBA(bounds)
	for x := 0; x < dx; x++ {
		for y := 0; y < dy; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			yc, uc, vc := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			yuvImage.Set(x, y, color.RGBA{uint8(yc), uint8(uc), uint8(vc), 255})
		}
	}
	return yuvImage
}

func dct(x, y, u, v, w float64) float64 {
	a := math.Cos(((2.0*x + 1) * (u * math.Pi)) / (2 * w))
	b := math.Cos(((2.0*y + 1) * (v * math.Pi)) / (2 * w))

	return a * b
}

func idct(u, v, x, y, w float64) float64 {
	// normalization
	alpha := func(a float64) float64 {
		if a == 0 {
			return 1.0 / math.Sqrt(2.0)
		}
		return 1.0
	}

	return dct(u, v, x, y, w) * alpha(u) * alpha(v)
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

// max returns the biggest value between two numbers.
func max(x, y int) float64 {
	if x > y {
		return float64(x)
	}
	return float64(y)
}
