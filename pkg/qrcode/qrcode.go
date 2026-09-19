package qrcode

import (
	"errors"

	skip2 "github.com/skip2/go-qrcode"
)

// GenerateMatrix generates a 2D boolean matrix representing the QR code
// for the given content using Error Correction Level M, without any quiet zone border.
// True indicates a dark module (foreground) and false indicates a light module (background).
func GenerateMatrix(content string) ([][]bool, error) {
	if len(content) == 0 {
		return nil, errors.New("qrcode: content cannot be empty")
	}

	qr, err := skip2.New(content, skip2.Medium)
	if err != nil {
		return nil, err
	}
	qr.DisableBorder = true

	return qr.Bitmap(), nil
}
