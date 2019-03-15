# Forensic

[![Build Status](https://travis-ci.org/esimov/forensic.svg?branch=master)](https://travis-ci.org/esimov/forensic)

Forensic is an image processing library which aims to detect copy-move forgeries in digital images. The implementation is mainly based on this paper: https://arxiv.org/pdf/1308.5661.pdf

### Implementation details

* Convert the `RGB` image to `YUV` color space.
* Divide the `R`,`G`,`B`,`Y` components into fixed-sized blocks.
* Obtain each block `R`,`G`,`B` and `Y` components.
* Calculate each block `R`,`G`,`B` and `Y` components `DCT` (Discrete Cosine Transform) coefficients.
* Extract features from the obtained `DCT` coefficients and save it into a matrix. The matrix rows will contain the blocks top-left coordinate position plus the DCT coefficient. The matrix will have `(M − b + 1)(N − b + 1)x9` elements.
* Sort the features in lexicographic order.
* Search for similar pairs of blocks. Because identical blocks are most probably neighbors, after ordering them in lexicographic order we need to apply a specific threshold to filter out the false positive detections. If the distance between two neighboring blocks is smaller than a predefined threshold the blocks are considered as a pair of candidate for the forgery.
* For each pair of candidate compute the cumulative number of shift vectors (how many times the same block is detected). If that number is greater than a predefined threshold the corresponding regions are considered forged.

## Install
First install Go if you haven't installed already, set your `GOPATH`, and make sure `$GOPATH/bin` is on your `PATH`.

```bash
$ export GOPATH="$HOME/go"
$ export PATH="$PATH:$GOPATH/bin"
```
Next download the project and build the binary file.

```bash
$ go get -u -f github.com/esimov/forensic
$ go install
```

In case you do not want to build the binary file yourself you can obtain the prebuilt one from the [releases](https://github.com/esimov/forensic/releases) folder.

## Usage

```bash
$ forensic -in input.jpg -out output.jpg
```

### Supported commands:
```bash 
$ forensic --help

Image forgery detection library.
    Version: 

  -blur int
    	Blur radius (default 1)
  -bs int
    	Block size (default 4)
  -dt float
    	Distance threshold (default 0.4)
  -ft float
    	Forgery threshold (default 210)
  -in string
    	Input image
  -ot int
    	Offset threshold (default 72)
  -out string
    	Output image
```

## Results
| Original image | Forged image | Detection result |
| --- | --- | --- |
| ![dogs_original](https://user-images.githubusercontent.com/883386/39047347-3fee70cc-44a2-11e8-8729-c4312c631017.jpg) | ![dogs_forged](https://user-images.githubusercontent.com/883386/39047218-c1c8c530-44a1-11e8-8eb6-f9a8470848bd.jpg) | ![dogs_result](https://user-images.githubusercontent.com/883386/39047481-aec6f0f0-44a2-11e8-9f0f-041b9f2a0eb4.png) |

### Notice
The library sometimes produce false positive detection, depending on the image content. For this reason i advice to adjust the settings. Also sometimes the human judgement is required, but in the most cases the library do a pretty good job in detecting forged images. The more intensive the overlayed color is, the more certain is that the image is tampered.

## Author

* Endre Simo

## License

Copyright © 2018 Endre Simo

This project is under the MIT License. See the LICENSE file for the full license text.
