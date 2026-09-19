package qrcode_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/qrcode"
)

func TestGenerateMatrix(t *testing.T) {
	t.Parallel()

	t.Run("generates square matrix for standard URL", func(t *testing.T) {
		matrix, err := qrcode.GenerateMatrix("https://petspotr.io/p/lost-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(matrix) == 0 {
			t.Fatal("expected non-empty matrix")
		}
		for i, row := range matrix {
			if len(row) != len(matrix) {
				t.Fatalf("row %d length %d does not match matrix height %d", i, len(row), len(matrix))
			}
		}
	})

	t.Run("rejects empty content", func(t *testing.T) {
		_, err := qrcode.GenerateMatrix("")
		if err == nil {
			t.Fatal("expected error for empty content")
		}
	})

	t.Run("returns error when content exceeds max QR capacity", func(t *testing.T) {
		hugeContent := strings.Repeat("https://petspotr.io/long-path-that-exceeds-max-qr-code-capacity/", 200)
		_, err := qrcode.GenerateMatrix(hugeContent)
		if err == nil {
			t.Fatal("expected error for oversized content")
		}
	})
}

func TestGenerateSVG(t *testing.T) {
	t.Parallel()

	t.Run("generates valid SVG for standard URL", func(t *testing.T) {
		url := "https://petspotr.io/p/lost-123"
		svgBytes, err := qrcode.GenerateSVG(url, 256, 2)
		if err != nil {
			t.Fatalf("unexpected error generating SVG: %v", err)
		}
		svg := string(svgBytes)
		if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(strings.TrimSpace(svg), "</svg>") {
			t.Fatalf("expected valid svg root tag, got:\n%s", svg)
		}
		if !strings.Contains(svg, `viewBox="0 0`) {
			t.Fatal("expected viewBox attribute in SVG")
		}
		if !strings.Contains(svg, `width="256"`) || !strings.Contains(svg, `height="256"`) {
			t.Fatal("expected width and height attributes matching size in SVG")
		}
		if !strings.Contains(svg, `<path`) {
			t.Fatal("expected path element in SVG")
		}
		if !strings.Contains(svg, `shape-rendering="crispEdges"`) {
			t.Fatal("expected crispEdges shape-rendering attribute in SVG")
		}
		if !strings.Contains(svg, `fill="#ffffff"`) {
			t.Fatal("expected white background in SVG")
		}
		if !strings.Contains(svg, `fill="#111827"`) {
			t.Fatal("expected foreground fill (#111827) in SVG")
		}
	})

	t.Run("calculates correct viewBox based on matrix and margin", func(t *testing.T) {
		url := "https://petspotr.io/p/test"
		matrix, err := qrcode.GenerateMatrix(url)
		if err != nil {
			t.Fatalf("unexpected error generating matrix: %v", err)
		}
		matrixDim := len(matrix)

		margin := 4
		expectedDim := matrixDim + (margin * 2)
		svgBytes, err := qrcode.GenerateSVG(url, 300, margin)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedViewBox := fmt.Sprintf(`viewBox="0 0 %d %d"`, expectedDim, expectedDim)
		if !strings.Contains(string(svgBytes), expectedViewBox) {
			t.Fatalf("expected %q in SVG viewBox, got:\n%s", expectedViewBox, string(svgBytes))
		}
	})

	t.Run("rejects empty content", func(t *testing.T) {
		_, err := qrcode.GenerateSVG("", 256, 2)
		if err == nil {
			t.Fatal("expected error for empty content")
		}
	})

	t.Run("enforces minimum dimensions", func(t *testing.T) {
		svgBytes, err := qrcode.GenerateSVG("https://petspotr.io/p/test", 16, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(svgBytes) == 0 {
			t.Fatal("expected non-empty svg output")
		}
	})

	t.Run("handles negative margin by defaulting or zero margin", func(t *testing.T) {
		url := "https://petspotr.io/p/test"
		matrix, err := qrcode.GenerateMatrix(url)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedDim := len(matrix)

		svgBytes, err := qrcode.GenerateSVG(url, 200, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedViewBox := fmt.Sprintf(`viewBox="0 0 %d %d"`, expectedDim, expectedDim)
		if !strings.Contains(string(svgBytes), expectedViewBox) {
			t.Fatalf("expected %q for zero margin, got:\n%s", expectedViewBox, string(svgBytes))
		}
	})

	t.Run("rejects non-positive size", func(t *testing.T) {
		_, err := qrcode.GenerateSVG("https://petspotr.io/p/test", 0, 2)
		if err == nil {
			t.Fatal("expected error for zero size")
		}
		_, err = qrcode.GenerateSVG("https://petspotr.io/p/test", -10, 2)
		if err == nil {
			t.Fatal("expected error for negative size")
		}
	})

	t.Run("returns error when content exceeds max QR capacity", func(t *testing.T) {
		hugeContent := strings.Repeat("https://petspotr.io/long-path-that-exceeds-max-qr-code-capacity/", 200)
		_, err := qrcode.GenerateSVG(hugeContent, 256, 2)
		if err == nil {
			t.Fatal("expected error for oversized content")
		}
	})

	t.Run("is concurrent safe", func(t *testing.T) {
		const goroutines = 20
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				url := fmt.Sprintf("https://petspotr.io/p/pet-%04d", idx)
				svgBytes, err := qrcode.GenerateSVG(url, 256, 2)
				if err != nil {
					t.Errorf("goroutine %d failed: %v", idx, err)
					return
				}
				if len(svgBytes) == 0 {
					t.Errorf("goroutine %d got empty bytes", idx)
				}
			}(i)
		}
		wg.Wait()
	})
}
