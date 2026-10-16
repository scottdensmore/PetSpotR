package cameratrap

import (
	"fmt"
	"image"
	"math/bits"
)

// DefaultDuplicateMaxHamming is the default maximum Hamming distance between
// consecutive 64-bit dHash values to classify a frame as a duplicate.
const DefaultDuplicateMaxHamming = 6

// ComputeDHash computes a 64-bit perceptual difference hash (dHash) by sampling
// the image onto a 9x8 grayscale luminance grid and comparing adjacent horizontal pixels.
func ComputeDHash(img image.Image) uint64 {
	if img == nil {
		return 0
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return 0
	}

	var lum [8][9]float64
	for row := 0; row < 8; row++ {
		sy := bounds.Min.Y + (row*h+h/2)/8
		if sy >= bounds.Max.Y {
			sy = bounds.Max.Y - 1
		}
		for col := 0; col < 9; col++ {
			sx := bounds.Min.X + (col*w+w/2)/9
			if sx >= bounds.Max.X {
				sx = bounds.Max.X - 1
			}
			r, g, b, _ := img.At(sx, sy).RGBA()
			rf := float64(r >> 8)
			gf := float64(g >> 8)
			bf := float64(b >> 8)
			lum[row][col] = 0.299*rf + 0.587*gf + 0.114*bf
		}
	}

	var hash uint64
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			if lum[row][col] > lum[row][col+1] {
				hash |= 1 << uint(row*8+col)
			}
		}
	}
	return hash
}

// FormatDHashHex formats a 64-bit dHash as a 16-character zero-padded hex string.
func FormatDHashHex(hash uint64) string {
	return fmt.Sprintf("%016x", hash)
}

// HammingDistance returns the number of differing bits between two 64-bit dHashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// IsDuplicateFrame checks whether currHash is within maxDist bits of prevHash.
func IsDuplicateFrame(prevHash, currHash uint64, hasPrev bool, maxDist int) (bool, int) {
	if !hasPrev {
		return false, 64
	}
	if maxDist <= 0 {
		maxDist = DefaultDuplicateMaxHamming
	}
	dist := HammingDistance(prevHash, currHash)
	return dist <= maxDist, dist
}
