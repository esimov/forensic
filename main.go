package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"github.com/nfnt/resize"
	"gopkg.in/cheggaaa/pb.v1"
)

// MaxImageSize is the resized image maximum width or height depending on the image ratio.
const MaxImageSize = 320

const Banner = `
  __                          _
 / _| ___  _ __ ___ _ __  ___(_) ___
| |_ / _ \| '__/ _ \ '_ \/ __| |/ __|
|  _| (_) | | |  __/ | | \__ \ | (__
|_|  \___/|_|  \___|_| |_|___/_|\___|

Image forgery detection library.
    Version: %s

`

// Version indicates the current build version.
var Version string

var (
	// Flags
	source            = flag.String("in", "", "Input image")
	destination       = flag.String("out", "", "Output image")
	blurRadius        = flag.Int("blur", 1, "Blur radius")
	blockSize         = flag.Int("bs", 4, "Block size")
	offsetThreshold   = flag.Int("ot", 72, "Offset threshold")
	distanceThreshold = flag.Float64("dt", 0.4, "Distance threshold")
	forgeryThreshold  = flag.Float64("ft", 210, "Forgery threshold")
)

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

// q4x4 is the quantization matrix table.
var q4x4 = [][]float64{
	{16.0, 10.0, 24.0, 51.0},
	{14.0, 16.0, 40.0, 69.0},
	{18.0, 37.0, 68.0, 103.0},
	{49.0, 78.0, 103.0, 120.0},
}

var (
	resizedImg     image.Image
	features       []feature
	vectors        []vector
	cr, cg, cb, cy float64
)

func main() {
	done := make(chan struct{})
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, fmt.Sprintf(Banner, Version))
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(*source) == 0 || len(*destination) == 0 {
		log.Fatal("Usage: forensic -in input.jpg -out out.jpg")
	}

	if *blockSize <= 1 {
		log.Fatal("ERROR: the block size must be greater then 1.")
	}

	start := time.Now()

	input, err := os.Open(*source)
	defer input.Close()

	if err != nil {
		log.Fatalf("Error reading the image file: %v", err)
	}
	src, _, err := image.Decode(input)
	if err != nil {
		log.Fatalf("Error decoding the image: %v", err)
	}

	if src.Bounds().Dx() > MaxImageSize {
		resizedImg = resize.Resize(MaxImageSize, 0, src, resize.Lanczos3)
	} else if src.Bounds().Dy() > MaxImageSize {
		resizedImg = resize.Resize(0, MaxImageSize, src, resize.Lanczos3)
	} else {
		resizedImg = src
	}

	go func() {
		var output string
		precision := float64(process(resizedImg, done))
		if precision > 50.0 {
			output = fmt.Sprintf("%.0f%% the image is forged!", precision)
		} else {
			precision = 100 - precision
			output = fmt.Sprintf("%.0f%% the image is NOT forged!", precision)
		}
		fmt.Println(output)
	}()
	<-done

	fmt.Printf("\nDone in: %.2fs\n", time.Since(start).Seconds())
}

// process analyze the input image and detect forgeries.
// It returns the precision score and a boolean value indication
func process(input image.Image, done chan struct{}) float64 {
	img := imgToNRGBA(input)
	output := image.NewRGBA(img.Bounds())
	draw.Draw(output, image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()), img, image.ZP, draw.Src)

	// Blur the image to eliminate the details.
	if *blurRadius > 0 {
		img = StackBlur(img, uint32(*blurRadius))
	}

	// Convert image to YUV color space
	yuv := convertRGBImageToYUV(img)
	newImg := image.NewRGBA(yuv.Bounds())
	draw.Draw(newImg, image.Rect(0, 0, yuv.Bounds().Dx(), yuv.Bounds().Dy()), yuv, image.ZP, draw.Src)

	dx, dy := yuv.Bounds().Max.X, yuv.Bounds().Max.Y
	bdx, bdy := (dx - *blockSize + 1), (dy - *blockSize + 1)
	n := math.Max(float64(dx), float64(dy))

	var blocks []imageBlock
	for i := 0; i < bdx; i++ {
		for j := 0; j < bdy; j++ {
			r := image.Rect(i, j, i+*blockSize, j+*blockSize)
			block := newImg.SubImage(r).(*image.RGBA)
			blocks = append(blocks, imageBlock{x: i, y: j, img: block})
		}
	}

	bar := pb.StartNew(len(blocks))
	bar.Prefix("Generate: ")

	for _, block := range blocks {
		// Average RGB value.
		var avr, avg, avb float64

		b := block.img.(*image.RGBA)
		i0 := b.PixOffset(b.Bounds().Min.X, b.Bounds().Min.Y)
		i1 := i0 + b.Bounds().Dx()*4

		dctPixels := make(dctPx, *blockSize**blockSize)
		for u := 0; u < *blockSize; u++ {
			dctPixels[u] = make([]pixel, *blockSize)
			for v := 0; v < *blockSize; v++ {
				for i := i0; i < i1; i += 4 {
					// Obtain the pixels converted to YUV color space
					yc, uc, vc, _ := b.Pix[i+0], b.Pix[i+2], b.Pix[i+2], b.Pix[i+3]
					// Convert YUV to RGB and obtain the R,G,B value
					r, g, b := color.YCbCrToRGB(yc, uc, vc)

					for x := 0; x < *blockSize; x++ {
						for y := 0; y < *blockSize; y++ {
							// Compute Discrete Cosine coefficients
							cr += dct(float64(x), float64(y), float64(u), float64(v), float64(*blockSize)) * float64(r)
							cg += dct(float64(x), float64(y), float64(u), float64(v), float64(*blockSize)) * float64(g)
							cb += dct(float64(x), float64(y), float64(u), float64(v), float64(*blockSize)) * float64(b)
							cy += dct(float64(x), float64(y), float64(u), float64(v), float64(*blockSize)) * float64(yc)

							avr += float64(r)
							avg += float64(g)
							avb += float64(b)
						}
					}
				}

				// Normalize alpha channel.
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

				// Obtain the quantized DCT coefficients.
				if *blockSize <= 4 {
					dctPixels[u][v].r = dctPixels[u][v].r / q4x4[u][v]
					dctPixels[u][v].g = dctPixels[u][v].g / q4x4[u][v]
					dctPixels[u][v].b = dctPixels[u][v].b / q4x4[u][v]
					dctPixels[u][v].y = dctPixels[u][v].y / q4x4[u][v]
				}
			}
		}
		avr /= float64(*blockSize * *blockSize)
		avg /= float64(*blockSize * *blockSize)
		avb /= float64(*blockSize * *blockSize)

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
		bar.Increment()
	}
	bar.Finish()

	// Lexicographically sort the feature vectors
	sort.Sort(featVec(features))

	bar = pb.StartNew(len(features) - 1)
	bar.Prefix("Analyze: ")

	for i := 0; i < len(features)-1; i++ {
		blockA, blockB := features[i], features[i+1]
		result := analyzeBlocks(blockA, blockB)

		if result != nil {
			vectors = append(vectors, *result)
		}
		bar.Increment()
	}
	bar.Finish()

	simBlocks := getSuspiciousBlocks(vectors)
	forgedBlocks, _ := filterOutNeighbors(simBlocks)

	simBlocksNum := len(simBlocks)
	forgedBlocksNum := len(forgedBlocks)

	// precision indicates the detection accuracy
	var precision = 0.0
	if forgedBlocksNum > 0 {
		precision = 100 - (float64(forgedBlocksNum) / (float64(forgedBlocksNum + simBlocksNum)) * 100)
	}

	forgedImg := image.NewRGBA(img.Bounds())
	overlay := color.RGBA{255, 0, 0, 255}

	fmt.Println("\nNumber of forged blocks detected: ", forgedBlocksNum)
	for _, bl := range forgedBlocks {
		draw.Draw(forgedImg, image.Rect(bl.xa, bl.ya, bl.xa+*blockSize*2, bl.ya+*blockSize*2), &image.Uniform{overlay}, image.ZP, draw.Over)
	}

	final := StackBlur(imgToNRGBA(forgedImg), 10)
	draw.Draw(output, img.Bounds(), final, image.ZP, draw.Over)

	out, err := os.Create(*destination)
	if err != nil {
		fmt.Printf("Error creating output file: %v", err)
	}

	if err := png.Encode(out, output); err != nil {
		fmt.Printf("Error encoding image file: %v", err)
	}
	done <- struct{}{}

	return precision
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

	if dist < *distanceThreshold {
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

	bar := pb.StartNew(len(vect)).Prefix("Detect: ")

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
		if duplicates[*offset] > *offsetThreshold {
			suspiciousBlocks = append(suspiciousBlocks, vector{
				v.xa, v.ya, v.xb, v.yb, v.offsetX, v.offsetY,
			})
		}
		bar.Increment()
	}
	bar.Finish()
	return suspiciousBlocks
}

// filterOutNeighbors filters out the neighboring blocks.
func filterOutNeighbors(vect []vector) (newVector, bool) {
	var forgedBlocks newVector
	var isForged bool

	bar := pb.StartNew(len(vect)).Prefix("Filter: ")

	for i := 1; i < len(vect); i++ {
		blockA, blockB := vect[i-1], vect[i]

		// Calculate the euclidean distance between both regions.
		dx := float64(blockA.xa - blockB.xa)
		dy := float64(blockA.ya - blockB.ya)
		dist := math.Sqrt(math.Pow(dx, 2) + math.Pow(dy, 2))

		// Evaluate the euclidean distance distance between two regions
		// and make sure the distance is greater than a predefined threshold.
		if dist > *forgeryThreshold {
			forgedBlocks = append(forgedBlocks, vector{
				blockA.xa, blockA.ya, blockA.xb, blockA.yb, blockA.offsetX, vect[i].offsetY,
			})
			// We need to verify if an image is forged only once.
			if !isForged {
				isForged = true
			}
		}
		bar.Increment()
	}
	bar.Finish()
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
