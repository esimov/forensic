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

const (
	BlockSize         int = 4
	DistanceThreshold     = 0.4
	OffsetThreshold       = 72
	ForgeryThreshold      = 220
)

var q4x4 = [][]float64{
	{16.0, 10.0, 24.0, 51.0},
	{14.0, 16.0, 40.0, 69.0},
	{18.0, 37.0, 68.0, 103.0},
	{49.0, 78.0, 103.0, 120.0},
}

// pixel struct contains the discrete cosine transformation R,G,B,Y values.
type pixel struct {
	r, g, b, y float64
}

// dctPx stores the DCT pixel values.
type dctPx [][]pixel

// imageBlock contains the generated block upper left position and the stored image.
type imageBlock struct {
	x   int
	y   int
	img image.Image
}

// vector struct contains the neighboring blocks top left position and the shift vectors between them.
type vector struct {
	xa, ya           int
	xb, yb           int
	offsetX, offsetY float64
}

// feature struct contains the feature blocks x, y position and their respective values.
type feature struct {
	x    int
	y    int
	coef float64
}

var (
	features       []feature
	vectors        []vector
	cr, cg, cb, cy float64
)

func main() {
	start := time.Now()

	input, err := os.Open("parade_forged.jpg")
	defer input.Close()

	if err != nil {
		fmt.Printf("Error reading the image file: %v", err)
	}
	src, _, err := image.Decode(input)
	if err != nil {
		fmt.Printf("Error decoding the image: %v", err)
	}

	img := imgToNRGBA(src)
	output := image.NewRGBA(img.Bounds())
	draw.Draw(output, image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()), img, image.ZP, draw.Src)

	// Blur the image to eliminate the details.
	blurImg := StackBlur(img, 1)

	// Convert image to YUV color space
	yuv := convertRGBImageToYUV(blurImg)
	newImg := image.NewRGBA(yuv.Bounds())
	draw.Draw(newImg, image.Rect(0, 0, yuv.Bounds().Dx(), yuv.Bounds().Dy()), yuv, image.ZP, draw.Src)

	dx, dy := yuv.Bounds().Max.X, yuv.Bounds().Max.Y
	bdx, bdy := (dx - BlockSize + 1), (dy - BlockSize + 1)
	n := math.Max(float64(dx), float64(dy))

	var blocks []imageBlock
	for i := 0; i < bdx; i++ {
		for j := 0; j < bdy; j++ {
			r := image.Rect(i, j, i+BlockSize, j+BlockSize)
			block := newImg.SubImage(r).(*image.RGBA)
			blocks = append(blocks, imageBlock{x: i, y: j, img: block})
			//draw.Draw(newImg, image.Rect(0, 0, yuv.Bounds().Max.X, yuv.Bounds().Max.Y), block, image.ZP, draw.Src)
		}
	}

	fmt.Printf("Len: %d\n", len(blocks))

	for _, block := range blocks {
		// Average RGB value.
		var avr, avg, avb float64

		b := block.img.(*image.RGBA)
		i0 := b.PixOffset(b.Bounds().Min.X, b.Bounds().Min.Y)
		i1 := i0 + b.Bounds().Dx() * 4

		dctPixels := make(dctPx, BlockSize * BlockSize)
		for u := 0; u < BlockSize; u++ {
			dctPixels[u] = make([]pixel, BlockSize)
			for v := 0; v < BlockSize; v++ {
				for i := i0; i < i1; i += 4 {
					// Get the YUV converted image pixels
					yc, uc, vc, _ := b.Pix[i + 0], b.Pix[i + 2], b.Pix[i + 2], b.Pix[i + 3]
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
						return math.Sqrt(1.0 / float64(n))
					} else {
						return math.Sqrt(2.0 / float64(n))
					}
				}

				cu, cv := float64(u), float64(v)
				cr *= alpha(cu) * alpha(cv)
				cg *= alpha(cu) * alpha(cv)
				cb *= alpha(cu) * alpha(cv)
				cy *= alpha(cu) * alpha(cv)

				dctPixels[u][v] = pixel{cr, cg, cb, cy}

				// Get the quantized DCT coefficients.
				dctPixels[u][v].r = (dctPixels[u][v].r / q4x4[u][v])
				dctPixels[u][v].g = (dctPixels[u][v].g / q4x4[u][v])
				dctPixels[u][v].b = (dctPixels[u][v].b / q4x4[u][v])
				dctPixels[u][v].y = (dctPixels[u][v].y / q4x4[u][v])
			}
		}
		avr /= float64(BlockSize * BlockSize)
		avg /= float64(BlockSize * BlockSize)
		avb /= float64(BlockSize * BlockSize)

		features = append(features, feature{x: block.x, y: block.y, coef: dctPixels[0][0].y})
		features = append(features, feature{x: block.x, y: block.y, coef: dctPixels[0][1].y})
		features = append(features, feature{x: block.x, y: block.y, coef: dctPixels[1][0].y})
		features = append(features, feature{x: block.x, y: block.y, coef: dctPixels[0][0].r})
		features = append(features, feature{x: block.x, y: block.y, coef: dctPixels[0][0].g})
		features = append(features, feature{x: block.x, y: block.y, coef: dctPixels[0][0].b})

		// Append average R,G,B values to the features vector(slice).
		features = append(features, feature{x: block.x, y: block.y, coef: avr})
		features = append(features, feature{x: block.x, y: block.y, coef: avb})
		features = append(features, feature{x: block.x, y: block.y, coef: avg})
	}

	// Lexicographically sort the feature vectors
	sort.Sort(featVec(features))

	for i := 0; i < len(features)-1; i++ {
		blockA, blockB := features[i], features[i+1]
		result := analyzeBlocks(blockA, blockB)

		if result != nil {
			vectors = append(vectors, *result)
		}
	}

	simBlocks := getSuspiciousBlocks(vectors)
	forgedBlocks, result := filterOutNeighbors(simBlocks)

	for _, bl := range forgedBlocks {
		background := color.RGBA{255, 0, 0, 255}
		draw.Draw(output, image.Rect(bl.xa, bl.ya, bl.xa+BlockSize, bl.ya+BlockSize), &image.Uniform{background}, image.ZP, draw.Src)
	}

	out, err := os.Create("output.png")
	if err != nil {
		fmt.Printf("Error creating output file: %v", err)
	}

	if err := png.Encode(out, output); err != nil {
		fmt.Printf("Error encoding image file: %v", err)
	}

	fmt.Println("\n", result)

	fmt.Printf("Features length: %d", len(features))

	fmt.Printf("\nDone in: %.2fs\n", time.Since(start).Seconds())
}

//convertRGBImageToYUV coverts the image from RGB to YUV color space.
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

// analyzeBlocks checks weather two neighboring blocks are considered almost identical.
func analyzeBlocks(blockA, blockB feature) *vector {
	// Compute the euclidean distance between two neighboring blocks.
	dx := float64(blockA.x) - float64(blockB.x)
	dy := float64(blockA.y) - float64(blockB.y)
	dist := math.Sqrt(math.Pow(dx, 2) + math.Pow(dy, 2))

	res := &vector{
		xa:      blockA.x,
		ya:      blockA.y,
		xb:      blockB.x,
		yb:      blockB.y,
		offsetX: math.Abs(dx),
		offsetY: math.Abs(dy),
	}

	if dist < DistanceThreshold {
		return res
	}
	return nil
}

type offset struct {
	x, y float64
}

type newVector []vector

// getSuspiciousBlocks analyze pair of candidate and check for
// similarity by computing the accumulative number of shift vectors.
func getSuspiciousBlocks(vect []vector) newVector {
	var suspiciousBlocks newVector
	//For each pair of candidate compute the accumulative number of the corresponding shift vectors.
	duplicates := make(map[offset]int)

	for _, v := range vect {
		// Check for duplicate blocks
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
		if duplicates[*offset] > OffsetThreshold {
			suspiciousBlocks = append(suspiciousBlocks, vector{
				v.xa, v.ya, v.xb, v.yb, v.offsetX, v.offsetY,
			})
		}
	}
	fmt.Println("Blocks: ", len(suspiciousBlocks))
	return suspiciousBlocks
}

// filterOutNeighbors filters out the neighboring blocks.
func filterOutNeighbors(vect []vector) (newVector, bool) {
	var forgedBlocks newVector
	var isForged bool

	for i := 1; i < len(vect); i++ {
		blockA, blockB := vect[i-1], vect[i]

		// Continue only if two regions are not neighbors.
		if blockA.xa != blockB.xa && blockA.ya != blockB.ya {
			// Calculate the euclidean distance between both regions.
			dx := float64(blockA.xa - blockB.xa)
			dy := float64(blockA.ya - blockB.ya)
			dist := math.Sqrt(math.Pow(dx, 2) + math.Pow(dy, 2))

			// Evaluate the euclidean distance distance between two regions
			// and make sure the distance is greater than a predefined threshold.
			if dist > ForgeryThreshold {
				forgedBlocks = append(forgedBlocks, vector{
					vect[i].xa, vect[i].ya, vect[i].xb, vect[i].yb, vect[i].offsetX, vect[i].offsetY,
				})
				// We need to verify if an image is forged only once.
				if !isForged {
					isForged = true
				}
			}
		}
	}
	return forgedBlocks, isForged
}

// dct computes the Discrete Cosine Transform.
// https://en.wikipedia.org/wiki/Discrete_cosine_transform
func dct(x, y, u, v, w float64) float64 {
	a := math.Cos(((2.0*x + 1) * (u * math.Pi)) / (2 * w))
	b := math.Cos(((2.0*y + 1) * (v * math.Pi)) / (2 * w))

	return a * b
}

// idct computes the Inverse Discrete Cosine Transform. (Only for testing purposes.)
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

// round rounds float number to it's nearest integer part.
func round(x float64) float64 {
	t := math.Trunc(x)
	if math.Abs(x-t) >= 0.5 {
		return t + math.Copysign(1, x)
	}
	return t
}

// clamp255 converts a float64 number to uint8.
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

// unique returns slice's unique values.
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

// Implement sorting function on feature vector
type featVec []feature

func (a featVec) Len() int      { return len(a) }
func (a featVec) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a featVec) Less(i, j int) bool {
	if a[i].coef < a[j].coef {
		return true
	}
	if a[i].coef > a[j].coef {
		return false
	}
	return a[i].coef < a[j].coef
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

// Converts any image type to *image.NRGBA with min-point at (0, 0).
func imgToNRGBA(img image.Image) *image.NRGBA {
	srcBounds := img.Bounds()
	if srcBounds.Min.X == 0 && srcBounds.Min.Y == 0 {
		if src0, ok := img.(*image.NRGBA); ok {
			return src0
		}
	}
	srcMinX := srcBounds.Min.X
	srcMinY := srcBounds.Min.Y

	dstBounds := srcBounds.Sub(srcBounds.Min)
	dstW := dstBounds.Dx()
	dstH := dstBounds.Dy()
	dst := image.NewNRGBA(dstBounds)

	switch src := img.(type) {
	case *image.NRGBA:
		rowSize := srcBounds.Dx() * 4
		for dstY := 0; dstY < dstH; dstY++ {
			di := dst.PixOffset(0, dstY)
			si := src.PixOffset(srcMinX, srcMinY+dstY)
			for dstX := 0; dstX < dstW; dstX++ {
				copy(dst.Pix[di:di+rowSize], src.Pix[si:si+rowSize])
			}
		}
	case *image.YCbCr:
		for dstY := 0; dstY < dstH; dstY++ {
			di := dst.PixOffset(0, dstY)
			for dstX := 0; dstX < dstW; dstX++ {
				srcX := srcMinX + dstX
				srcY := srcMinY + dstY
				siy := src.YOffset(srcX, srcY)
				sic := src.COffset(srcX, srcY)
				r, g, b := color.YCbCrToRGB(src.Y[siy], src.Cb[sic], src.Cr[sic])
				dst.Pix[di+0] = r
				dst.Pix[di+1] = g
				dst.Pix[di+2] = b
				dst.Pix[di+3] = 0xff
				di += 4
			}
		}
	default:
		for dstY := 0; dstY < dstH; dstY++ {
			di := dst.PixOffset(0, dstY)
			for dstX := 0; dstX < dstW; dstX++ {
				c := color.NRGBAModel.Convert(img.At(srcMinX+dstX, srcMinY+dstY)).(color.NRGBA)
				dst.Pix[di+0] = c.R
				dst.Pix[di+1] = c.G
				dst.Pix[di+2] = c.B
				dst.Pix[di+3] = c.A
				di += 4
			}
		}
	}
	return dst
}
