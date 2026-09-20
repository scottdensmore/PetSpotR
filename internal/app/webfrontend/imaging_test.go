package webfrontend_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func createTestJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / w),
				G: uint8((y * 255) / h),
				B: 128,
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func buildTestJPEGWithFullEXIF() []byte {
	order := binary.LittleEndian
	buf := new(bytes.Buffer)

	// Header: II, 42, offset 8
	buf.WriteString("II")
	_ = binary.Write(buf, order, uint16(42))
	_ = binary.Write(buf, order, uint32(8))

	exifIFDOffset := uint32(62)
	gpsIFDOffset := uint32(104)
	dateStringOffset := uint32(158)
	latRationalOffset := uint32(178)
	lonRationalOffset := uint32(202)

	// IFD0 (4 entries)
	_ = binary.Write(buf, order, uint16(4))
	// 1. Orientation = 6 (SHORT)
	_ = binary.Write(buf, order, uint16(0x0112))
	_ = binary.Write(buf, order, uint16(3))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint16(6))
	_ = binary.Write(buf, order, uint16(0))
	// 2. ImageWidth = 4032 (LONG)
	_ = binary.Write(buf, order, uint16(0x0100))
	_ = binary.Write(buf, order, uint16(4))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint32(4032))
	// 3. ExifIFDPointer
	_ = binary.Write(buf, order, uint16(0x8769))
	_ = binary.Write(buf, order, uint16(4))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, exifIFDOffset)
	// 4. GPSInfoIFDPointer
	_ = binary.Write(buf, order, uint16(0x8825))
	_ = binary.Write(buf, order, uint16(4))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, gpsIFDOffset)
	_ = binary.Write(buf, order, uint32(0)) // next IFD

	// Exif SubIFD (3 entries)
	_ = binary.Write(buf, order, uint16(3))
	// 1. DateTimeOriginal
	_ = binary.Write(buf, order, uint16(0x9003))
	_ = binary.Write(buf, order, uint16(2))
	_ = binary.Write(buf, order, uint32(20))
	_ = binary.Write(buf, order, dateStringOffset)
	// 2. PixelXDimension = 4032
	_ = binary.Write(buf, order, uint16(0xA002))
	_ = binary.Write(buf, order, uint16(4))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint32(4032))
	// 3. PixelYDimension = 3024
	_ = binary.Write(buf, order, uint16(0xA003))
	_ = binary.Write(buf, order, uint16(4))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint32(3024))
	_ = binary.Write(buf, order, uint32(0)) // next IFD

	// GPS SubIFD (4 entries)
	_ = binary.Write(buf, order, uint16(4))
	// 1. GPSLatitudeRef "N\x00"
	_ = binary.Write(buf, order, uint16(0x0001))
	_ = binary.Write(buf, order, uint16(2))
	_ = binary.Write(buf, order, uint32(2))
	buf.Write([]byte("N\x00\x00\x00"))
	// 2. GPSLatitude rationals
	_ = binary.Write(buf, order, uint16(0x0002))
	_ = binary.Write(buf, order, uint16(5))
	_ = binary.Write(buf, order, uint32(3))
	_ = binary.Write(buf, order, latRationalOffset)
	// 3. GPSLongitudeRef "W\x00"
	_ = binary.Write(buf, order, uint16(0x0003))
	_ = binary.Write(buf, order, uint16(2))
	_ = binary.Write(buf, order, uint32(2))
	buf.Write([]byte("W\x00\x00\x00"))
	// 4. GPSLongitude rationals
	_ = binary.Write(buf, order, uint16(0x0004))
	_ = binary.Write(buf, order, uint16(5))
	_ = binary.Write(buf, order, uint32(3))
	_ = binary.Write(buf, order, lonRationalOffset)
	_ = binary.Write(buf, order, uint32(0)) // next IFD

	// Extra data
	buf.Write([]byte("2026:09:19 18:15:00\x00")) // 20 bytes (offset 158)
	// Lat: 47 + 36/60 + 46.8/3600 = 47.613
	_ = binary.Write(buf, order, uint32(47))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint32(36))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint32(468))
	_ = binary.Write(buf, order, uint32(10))
	// Lon: -(122 + 20/60 + 13.2/3600) = -122.337
	_ = binary.Write(buf, order, uint32(122))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint32(20))
	_ = binary.Write(buf, order, uint32(1))
	_ = binary.Write(buf, order, uint32(132))
	_ = binary.Write(buf, order, uint32(10))

	tiffBytes := buf.Bytes()

	// Assemble JPEG
	jpegBuf := new(bytes.Buffer)
	jpegBuf.Write([]byte{0xFF, 0xD8}) // SOI
	// APP1
	jpegBuf.Write([]byte{0xFF, 0xE1})
	app1Len := uint16(2 + 6 + len(tiffBytes))
	_ = binary.Write(jpegBuf, binary.BigEndian, app1Len)
	jpegBuf.Write([]byte("Exif\x00\x00"))
	jpegBuf.Write(tiffBytes)
	// SOF0 (4032 x 3024)
	jpegBuf.Write([]byte{0xFF, 0xC0})
	sofLen := uint16(17)
	_ = binary.Write(jpegBuf, binary.BigEndian, sofLen)
	jpegBuf.WriteByte(8)
	_ = binary.Write(jpegBuf, binary.BigEndian, uint16(3024))
	_ = binary.Write(jpegBuf, binary.BigEndian, uint16(4032))
	jpegBuf.WriteByte(3)
	jpegBuf.Write([]byte{1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0})
	jpegBuf.Write([]byte{0xFF, 0xD9}) // EOI

	return jpegBuf.Bytes()
}

func createMultipartReq(t *testing.T, method, url, fieldName, fileName string, fileBytes []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if fieldName != "" {
		part, err := writer.CreateFormFile(fieldName, fileName)
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			t.Fatalf("failed to write file part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestImagingEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		DisableRateLimiting: true,
	})
	defer srv.Close()

	t.Run("POST /api/v1/images/extract-metadata", func(t *testing.T) {
		t.Run("invalid non-multipart request returns 400", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/images/extract-metadata", strings.NewReader("not multipart"))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
			}
		})

		t.Run("multipart missing file field returns 400", func(t *testing.T) {
			req := createMultipartReq(t, http.MethodPost, "/api/v1/images/extract-metadata", "wrong_field", "photo.jpg", []byte("some data"))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request for missing file field, got %d", rec.Code)
			}
		})

		t.Run("corrupt image data returns 400", func(t *testing.T) {
			req := createMultipartReq(t, http.MethodPost, "/api/v1/images/extract-metadata", "file", "photo.jpg", []byte("corrupt random text bytes"))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request for corrupt image data, got %d", rec.Code)
			}
		})

		t.Run("valid image with EXIF returns 200 and extracted metadata", func(t *testing.T) {
			exifJPEG := buildTestJPEGWithFullEXIF()
			req := createMultipartReq(t, http.MethodPost, "/api/v1/images/extract-metadata", "file", "photo.jpg", exifJPEG)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
			}

			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("expected Content-Type application/json, got %q", ct)
			}

			var resp struct {
				GPS *struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"gps"`
				CaptureTime *time.Time `json:"captureTime"`
				Orientation int        `json:"orientation"`
				Width       int        `json:"width"`
				Height      int        `json:"height"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode JSON response: %v", err)
			}

			if resp.GPS == nil {
				t.Fatal("expected GPS coordinates to be present, got nil")
			}
			if math.Abs(resp.GPS.Latitude-47.613) > 1e-3 {
				t.Errorf("expected Latitude ~47.613, got %f", resp.GPS.Latitude)
			}
			if math.Abs(resp.GPS.Longitude-(-122.337)) > 1e-3 {
				t.Errorf("expected Longitude ~-122.337, got %f", resp.GPS.Longitude)
			}
			if resp.CaptureTime == nil {
				t.Fatal("expected CaptureTime to be present, got nil")
			}
			expectedTime := "2026-09-19T18:15:00Z"
			if resp.CaptureTime.UTC().Format(time.RFC3339) != expectedTime {
				t.Errorf("expected CaptureTime %s, got %s", expectedTime, resp.CaptureTime.UTC().Format(time.RFC3339))
			}
			if resp.Orientation != 6 {
				t.Errorf("expected Orientation 6, got %d", resp.Orientation)
			}
			if resp.Width != 4032 {
				t.Errorf("expected Width 4032, got %d", resp.Width)
			}
			if resp.Height != 3024 {
				t.Errorf("expected Height 3024, got %d", resp.Height)
			}
		})

		t.Run("valid image without EXIF returns 200 with nil/omitted GPS", func(t *testing.T) {
			rawJPEG := createTestJPEG(120, 80)
			req := createMultipartReq(t, http.MethodPost, "/api/v1/images/extract-metadata", "file", "no-exif.jpg", rawJPEG)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp struct {
				GPS *struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"gps"`
				CaptureTime *time.Time `json:"captureTime"`
				Orientation int        `json:"orientation"`
				Width       int        `json:"width"`
				Height      int        `json:"height"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode JSON response: %v", err)
			}

			if resp.GPS != nil {
				t.Errorf("expected GPS to be nil for image without EXIF, got %+v", resp.GPS)
			}
			if resp.Width != 120 {
				t.Errorf("expected Width 120, got %d", resp.Width)
			}
			if resp.Height != 80 {
				t.Errorf("expected Height 80, got %d", resp.Height)
			}
			if resp.Orientation != 1 {
				t.Errorf("expected Orientation 1, got %d", resp.Orientation)
			}
		})
	})

	t.Run("POST /api/v1/images/enhance", func(t *testing.T) {
		t.Run("invalid non-multipart request returns 400", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/images/enhance", strings.NewReader("not multipart"))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
			}
		})

		t.Run("multipart missing file field returns 400", func(t *testing.T) {
			req := createMultipartReq(t, http.MethodPost, "/api/v1/images/enhance", "wrong_field", "photo.jpg", []byte("some data"))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request for missing file field, got %d", rec.Code)
			}
		})

		t.Run("corrupt image data returns 400", func(t *testing.T) {
			req := createMultipartReq(t, http.MethodPost, "/api/v1/images/enhance", "file", "photo.jpg", []byte("corrupt text bytes"))
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request for corrupt image data, got %d", rec.Code)
			}
		})

		t.Run("valid image returns 200, Content-Type image/jpeg, and X-Enhancement-Applied header", func(t *testing.T) {
			rawJPEG := createTestJPEG(64, 48)
			req := createMultipartReq(t, http.MethodPost, "/api/v1/images/enhance", "file", "pet.jpg", rawJPEG)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
			}

			if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
				t.Errorf("expected Content-Type image/jpeg, got %q", ct)
			}

			if enh := rec.Header().Get("X-Enhancement-Applied"); enh != "true" {
				t.Errorf("expected X-Enhancement-Applied header 'true', got %q", enh)
			}

			// Verify decoded image
			decoded, err := jpeg.Decode(rec.Body)
			if err != nil {
				t.Fatalf("failed to decode enhanced JPEG response: %v", err)
			}
			if decoded.Bounds().Dx() != 64 || decoded.Bounds().Dy() != 48 {
				t.Errorf("expected enhanced dimensions 64x48, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
			}
		})
	})

	t.Run("Method Not Allowed", func(t *testing.T) {
		methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
		endpoints := []string{"/api/v1/images/extract-metadata", "/api/v1/images/enhance"}

		for _, endpoint := range endpoints {
			for _, method := range methods {
				t.Run(method+" "+endpoint, func(t *testing.T) {
					req := httptest.NewRequest(method, endpoint, nil)
					rec := httptest.NewRecorder()
					srv.ServeHTTP(rec, req)

					if rec.Code != http.StatusMethodNotAllowed {
						t.Errorf("expected 405 Method Not Allowed for %s %s, got %d", method, endpoint, rec.Code)
					}
				})
			}
		}
	})
}
