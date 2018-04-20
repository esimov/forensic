## Forensic

Forensic is an image processing library which aims to detect copy-move forgeries in digital images. It's implementation is mainly based on this paper: https://arxiv.org/pdf/1308.5661.pdf

### Implementation details

* Convert the `RGB` image to `YUV` color space.
* Divide the `R`,`G`,`B`,`Y` components into fixed-sized blocks.
* Obtain each block `R`,`G`,`B` and `Y` components.
* Calculate each block `R`,`G`,`B` and `Y` components `DCT` (Discrete Cosine Transform) coefficients.
* Extract features from the obtained `DCT` coefficients and save it into a matrix. The matrix rows will contain the blocks top-left coordinate position plus the DCT coefficient. The matrix will have `(M − b + 1)(N − b + 1)x9` elements.
* Sort the features in lexicographic order.
* Search for similar pairs of blocks. Because identical blocks are most probably neighbors, after ordering them in lexicographic order we need to apply a specific threshold to filter out the false positive detections. If the distance between two neighboring blocks is smaller than a predefined threshold the blocks are considered as a pair of candidate for the forgery.
* For each pair of candidate compute the cumulative number of shift vectors (how many times the same block is detected). If that number is greater than a predefined threshold the corresponding regions are considered forged.

