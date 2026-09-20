package webfrontend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"time"

	"github.com/scottdensmore/petspotr/pkg/imaging"
)

const maxImageUploadBytes = 32 * 1024 * 1024 // 32MB

// ExtractMetadataResponse represents the JSON response for image metadata extraction.
type ExtractMetadataResponse struct {
	GPS         *imaging.GPSCoords `json:"gps"`
	CaptureTime *time.Time         `json:"captureTime"`
	Orientation int                `json:"orientation"`
	Width       int                `json:"width"`
	Height      int                `json:"height"`
}

func (s *Server) handleApiImageExtractMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)

	file, _, err := r.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Image exceeds maximum allowed size (32MB)", http.StatusBadRequest)
			return
		}
		http.Error(w, "Invalid multipart form or missing 'file' field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read image data", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "Empty image file", http.StatusBadRequest)
		return
	}

	meta, err := imaging.ExtractMetadata(data)
	if err != nil {
		if errors.Is(err, imaging.ErrNoExif) || errors.Is(err, imaging.ErrInvalidImage) {
			// Fallback: check if the image is a valid decodeable image (JPEG, PNG, GIF, etc.)
			cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(data))
			if cfgErr != nil {
				http.Error(w, "Invalid or unsupported image format", http.StatusBadRequest)
				return
			}
			if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > imaging.MaxImageDimension || cfg.Height > imaging.MaxImageDimension {
				http.Error(w, "Image dimensions exceed safe limits", http.StatusBadRequest)
				return
			}
			meta = &imaging.ImageMetadata{
				Orientation: 1,
				Width:       cfg.Width,
				Height:      cfg.Height,
			}
		} else {
			http.Error(w, fmt.Sprintf("Failed to extract metadata: %v", err), http.StatusBadRequest)
			return
		}
	}

	orientation := meta.Orientation
	if orientation < 1 || orientation > 8 {
		orientation = 1
	}

	resp := ExtractMetadataResponse{
		GPS:         meta.GPS,
		CaptureTime: meta.CaptureTime,
		Orientation: orientation,
		Width:       meta.Width,
		Height:      meta.Height,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleApiImageEnhance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)

	file, _, err := r.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Image exceeds maximum allowed size (32MB)", http.StatusBadRequest)
			return
		}
		http.Error(w, "Invalid multipart form or missing 'file' field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read image data", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "Empty image file", http.StatusBadRequest)
		return
	}

	enhanced, err := imaging.EnhanceImage(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to enhance image: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Enhancement-Applied", "true")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(enhanced)
}
