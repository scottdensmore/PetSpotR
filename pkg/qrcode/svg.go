package qrcode

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// GenerateSVG generates a scalable SVG vector representation of a QR code
// with Error Correction Level M, crisp edges, a white background, and #111827 dark foreground.
//
// Parameters:
//   - content: text or URL payload to encode (must be non-empty).
//   - size: pixel width and height for viewport dimension (must be > 0).
//   - margin: quiet zone border width in modules (negative values are treated as 0).
func GenerateSVG(content string, size int, margin int) ([]byte, error) {
	if len(content) == 0 {
		return nil, errors.New("qrcode: content cannot be empty")
	}
	if size <= 0 {
		return nil, errors.New("qrcode: size must be greater than zero")
	}
	if margin < 0 {
		margin = 0
	}

	matrix, err := GenerateMatrix(content)
	if err != nil {
		return nil, err
	}

	matrixSize := len(matrix)
	totalModules := matrixSize + (margin * 2)

	var pathBuilder strings.Builder
	for y := 0; y < matrixSize; y++ {
		for x := 0; x < matrixSize; {
			if !matrix[y][x] {
				x++
				continue
			}
			start := x
			for x < matrixSize && matrix[y][x] {
				x++
			}
			run := x - start
			if pathBuilder.Len() > 0 {
				pathBuilder.WriteByte(' ')
			}
			fmt.Fprintf(&pathBuilder, "M%d,%d h%d v1 h-%d z", start+margin, y+margin, run, run)
		}
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" shape-rendering="crispEdges">
<rect width="100%%" height="100%%" fill="#ffffff"/>
<path d="%s" fill="#111827"/>
</svg>
`, totalModules, totalModules, size, size, pathBuilder.String())

	return buf.Bytes(), nil
}
