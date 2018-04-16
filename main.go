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
	"sort"
	"time"
)

const BlockSize int = 4
const MagnitudeThreshold = 20
const FrequencyThreshold = 40

type pixel struct {
	r, g, b, y float64
}

type dctPx [][]pixel

type imageBlock struct {
	x   int
	y   int
	img image.Image
}

type vector struct {
	xa, ya           int
	xb, yb           int
	offsetX, offsetY int
}

type feature struct {
	x   int
	y   int
	val float64
}

var (
	features       []feature
	vectors        []vector
	cr, cg, cb, cy float64
)

func main() {
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

	var blocks []imageBlock
	for i := 0; i < bdx; i++ {
		for j := 0; j < bdy; j++ {
			r := image.Rect(i, j, i+BlockSize, j+BlockSize)
			block := newImg.SubImage(r).(*image.RGBA)
			blocks = append(blocks, imageBlock{x: i, y: j, img: block})
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
		b := block.img.(*image.RGBA)
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

							avr += float64(r)
							avg += float64(g)
							avb += float64(b)
						}
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

		features = append(features, feature{x: block.x, y: block.y, val: dctPixels[0][0].y})
		features = append(features, feature{x: block.x, y: block.y, val: dctPixels[0][1].y})
		features = append(features, feature{x: block.x, y: block.y, val: dctPixels[1][0].y})
		features = append(features, feature{x: block.x, y: block.y, val: dctPixels[0][0].r})
		features = append(features, feature{x: block.x, y: block.y, val: dctPixels[0][0].g})
		features = append(features, feature{x: block.x, y: block.y, val: dctPixels[0][0].b})

		// Append average red, green and blue values
		features = append(features, feature{x: block.x, y: block.y, val: avr})
		features = append(features, feature{x: block.x, y: block.y, val: avb})
		features = append(features, feature{x: block.x, y: block.y, val: avg})
	}

	// Lexicographically sort the feature vectors
	sort.Sort(featVec(features))

	for i := 0; i < len(features)-1; i++ {
		blockA, blockB := features[i], features[i+1]
		result := analyze(blockA, blockB)

		if result != nil {
			vectors = append(vectors, *result)
		}
	}
	res := checkForSimilarity(vectors)
	fmt.Println(res)

	//fmt.Println(features)
	fmt.Printf("Features length: %d", len(features))

	fmt.Printf("\nDone in: %.2fs\n", time.Since(start).Seconds())
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

// Check weather two neighboring blocks are considered almost identical.
func analyze(blockA, blockB feature) *vector {
	// Compute the euclidean distance between two neighboring blocks.
	dx := float64(blockA.x) - float64(blockB.x)
	dy := float64(blockA.y) - float64(blockB.y)
	dist := math.Sqrt(math.Pow(dx, 2) + math.Pow(dy, 2))

	res := &vector{
		xa:      blockA.x,
		ya:      blockA.y,
		xb:      blockB.x,
		yb:      blockB.y,
		offsetX: int(dx),
		offsetY: int(dy),
	}
	if dist < MagnitudeThreshold {
		return res
	}
	return nil
}

type offset struct {
	x, y int
}

type newVector []vector

// Analyze pair of candidate and check for similarity by computing the accumulative number of shift vectors.
func checkForSimilarity(vect []vector) newVector {
	var identicalBlocks newVector
	//For each pair of candidate compute the accumulative number of the corresponding shift vectors.
	duplicates := make(map[offset]int)

	for _, v := range vect {
		// Check if the element exist in the duplicateItems map.
		offsetX := v.offsetX
		offsetY := v.offsetY
		offset := &offset{offsetX, offsetY}

		_, exists := duplicates[*offset]
		if exists {
			duplicates[*offset]++
		} else {
			duplicates[*offset] = 1
		}

		// If the accumulative number of corresponding shift vectors is greater than
		// a predefined threshold, the corresponding regions are marked as suspicious.
		if duplicates[*offset] > FrequencyThreshold {
			identicalBlocks = append(identicalBlocks, vector{
				v.xa, v.xb, v.ya, v.yb, v.offsetX, v.offsetY,
			})
		}
	}
	return identicalBlocks
}

// Implement sorting function on feature vector
type featVec []feature

func (a featVec) Len() int      { return len(a) }
func (a featVec) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a featVec) Less(i, j int) bool {
	if a[i].val < a[j].val {
		return true
	}
	if a[i].val > a[j].val {
		return false
	}
	return a[i].val < a[j].val
}

func unique(intSlice []int) []int {
	keys := make(map[int]bool)
	list := []int{}
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
