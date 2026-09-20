package imaging_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/imaging"
)

// buildSyntheticTIFF creates a binary TIFF payload with the given byte order and tags.
func buildSyntheticTIFF(byteOrder binary.ByteOrder, isLittleEndian bool, fn func(tw *tiffWriter)) []byte {
	buf := new(bytes.Buffer)

	// Header: 8 bytes
	if isLittleEndian {
		buf.WriteString("II")
		_ = binary.Write(buf, byteOrder, uint16(42))
	} else {
		buf.WriteString("MM")
		_ = binary.Write(buf, byteOrder, uint16(42))
	}
	// Offset to IFD0 is 8
	_ = binary.Write(buf, byteOrder, uint32(8))

	tw := &tiffWriter{
		buf:            buf,
		order:          byteOrder,
		isLittleEndian: isLittleEndian,
	}

	fn(tw)

	return buf.Bytes()
}

type tiffWriter struct {
	buf            *bytes.Buffer
	order          binary.ByteOrder
	isLittleEndian bool
}

// buildJPEGWithEXIF wraps a TIFF payload in a valid JPEG structure with APP1 and optional SOF0.
func buildJPEGWithEXIF(tiffBytes []byte, width, height int) []byte {
	buf := new(bytes.Buffer)

	// SOI
	buf.Write([]byte{0xFF, 0xD8})

	// APP1 (EXIF)
	if tiffBytes != nil {
		buf.Write([]byte{0xFF, 0xE1})
		// Length includes 2 bytes for length itself + 6 bytes "Exif\x00\x00" + tiff length
		app1Len := uint16(2 + 6 + len(tiffBytes))
		_ = binary.Write(buf, binary.BigEndian, app1Len)
		buf.Write([]byte("Exif\x00\x00"))
		buf.Write(tiffBytes)
	}

	// SOF0 (Baseline DCT)
	if width > 0 && height > 0 {
		buf.Write([]byte{0xFF, 0xC0})
		sofLen := uint16(17) // standard SOF0 with 3 components
		_ = binary.Write(buf, binary.BigEndian, sofLen)
		buf.WriteByte(8) // 8-bit precision
		_ = binary.Write(buf, binary.BigEndian, uint16(height))
		_ = binary.Write(buf, binary.BigEndian, uint16(width))
		buf.WriteByte(3) // 3 components (Y, Cb, Cr)
		buf.Write([]byte{1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0})
	}

	// EOI
	buf.Write([]byte{0xFF, 0xD9})

	return buf.Bytes()
}

func TestExtractMetadata_InvalidDummyBytes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		data []byte
	}{
		{"dummy text bytes", []byte("dummy jpeg bytes with EXIF")},
		{"empty slice", []byte{}},
		{"nil slice", nil},
		{"short slice", []byte{0xFF, 0xD8}},
		{"corrupt marker", []byte{0xFF, 0xD8, 0xFF, 0x00}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := imaging.ExtractMetadata(tc.data)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestExtractMetadata_JPEGWithoutExif(t *testing.T) {
	t.Parallel()

	// JPEG with SOF0 (width=800, height=600) but without APP1 EXIF
	data := buildJPEGWithEXIF(nil, 800, 600)

	_, err := imaging.ExtractMetadata(data)
	if err == nil {
		t.Fatalf("expected error for JPEG without EXIF, got nil")
	}
	if !errors.Is(err, imaging.ErrNoExif) {
		t.Fatalf("expected ErrNoExif, got: %v", err)
	}
}

func TestExtractMetadata_JPEGWithFullEXIF_LittleEndian(t *testing.T) {
	t.Parallel()

	// Target coordinates from spec: lat: 47.613, lon: -122.337
	// Lat: 47 deg, 36 min, 46.8 sec N (47 + 36/60 + 46.8/3600 = 47.613)
	// Lon: 122 deg, 20 min, 13.2 sec W (-(122 + 20/60 + 13.2/3600) = -122.337)
	// CaptureTime: "2026-09-19T18:15:00Z" -> "2026:09:19 18:15:00"
	// Orientation: 6
	// Width: 4032, Height: 3024

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		// Layout plan:
		// Offset 8: IFD0
		// IFD0 has 4 entries (12 bytes each) = 48 bytes + 2 (count) + 4 (next IFD) = 54 bytes
		// Tags:
		// 1. 0x0112 Orientation (SHORT, count=1, value=6)
		// 2. 0x0100 ImageWidth (LONG, count=1, value=4032)
		// 3. 0x8769 ExifIFDPointer (LONG, count=1, value=offset of Exif SubIFD)
		// 4. 0x8825 GPSInfoIFDPointer (LONG, count=1, value=offset of GPS SubIFD)

		// Exact offsets:
		// Header = 8 bytes
		// IFD0: offset 8, size = 2 + 4*12 + 4 = 54 bytes. Next offset = 62.
		// Exif SubIFD: offset 62.
		// Exif SubIFD has 3 entries:
		// 1. 0x9003 DateTimeOriginal (ASCII, count=20, offset to date string)
		// 2. 0xA002 PixelXDimension (LONG, count=1, value=4032)
		// 3. 0xA003 PixelYDimension (LONG, count=1, value=3024)
		// Size of Exif SubIFD = 2 + 3*12 + 4 = 42 bytes. Next offset = 62 + 42 = 104.
		// GPS SubIFD: offset 104.
		// GPS SubIFD has 4 entries:
		// 1. 0x0001 GPSLatitudeRef (ASCII, count=2, inline "N\x00")
		// 2. 0x0002 GPSLatitude (RATIONAL, count=3, offset to 3 rationals)
		// 3. 0x0003 GPSLongitudeRef (ASCII, count=2, inline "W\x00")
		// 4. 0x0004 GPSLongitude (RATIONAL, count=3, offset to 3 rationals)
		// Size of GPS SubIFD = 2 + 4*12 + 4 = 54 bytes. Next offset = 104 + 54 = 158.

		// Extra data block starting at offset 158:
		// - DateTimeOriginal string: "2026:09:19 18:15:00\x00" (20 bytes) -> offset 158
		// - GPSLatitude rationals (3 * 8 = 24 bytes) -> offset 178
		// - GPSLongitude rationals (3 * 8 = 24 bytes) -> offset 202

		exifIFDOffset := uint32(62)
		gpsIFDOffset := uint32(104)
		dateStringOffset := uint32(158)
		latRationalOffset := uint32(178)
		lonRationalOffset := uint32(202)

		// Write IFD0
		_ = binary.Write(tw.buf, order, uint16(4)) // 4 entries

		// Entry 1: Orientation (0x0112, SHORT=3, count=1, value=6)
		_ = binary.Write(tw.buf, order, uint16(0x0112))
		_ = binary.Write(tw.buf, order, uint16(3))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint16(6))
		_ = binary.Write(tw.buf, order, uint16(0)) // padding

		// Entry 2: ImageWidth (0x0100, LONG=4, count=1, value=4032)
		_ = binary.Write(tw.buf, order, uint16(0x0100))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(4032))

		// Entry 3: ExifIFDPointer (0x8769, LONG=4, count=1, value=exifIFDOffset)
		_ = binary.Write(tw.buf, order, uint16(0x8769))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, exifIFDOffset)

		// Entry 4: GPSInfoIFDPointer (0x8825, LONG=4, count=1, value=gpsIFDOffset)
		_ = binary.Write(tw.buf, order, uint16(0x8825))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, gpsIFDOffset)

		// Next IFD = 0
		_ = binary.Write(tw.buf, order, uint32(0))

		// Write Exif SubIFD at offset 62
		_ = binary.Write(tw.buf, order, uint16(3)) // 3 entries

		// Entry 1: DateTimeOriginal (0x9003, ASCII=2, count=20, offset=dateStringOffset)
		_ = binary.Write(tw.buf, order, uint16(0x9003))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(20))
		_ = binary.Write(tw.buf, order, dateStringOffset)

		// Entry 2: PixelXDimension (0xA002, LONG=4, count=1, value=4032)
		_ = binary.Write(tw.buf, order, uint16(0xA002))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(4032))

		// Entry 3: PixelYDimension (0xA003, LONG=4, count=1, value=3024)
		_ = binary.Write(tw.buf, order, uint16(0xA003))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(3024))

		// Next IFD = 0
		_ = binary.Write(tw.buf, order, uint32(0))

		// Write GPS SubIFD at offset 104
		_ = binary.Write(tw.buf, order, uint16(4)) // 4 entries

		// Entry 1: GPSLatitudeRef (0x0001, ASCII=2, count=2, inline "N\x00")
		_ = binary.Write(tw.buf, order, uint16(0x0001))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("N\x00\x00\x00"))

		// Entry 2: GPSLatitude (0x0002, RATIONAL=5, count=3, offset=latRationalOffset)
		_ = binary.Write(tw.buf, order, uint16(0x0002))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, latRationalOffset)

		// Entry 3: GPSLongitudeRef (0x0003, ASCII=2, count=2, inline "W\x00")
		_ = binary.Write(tw.buf, order, uint16(0x0003))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("W\x00\x00\x00"))

		// Entry 4: GPSLongitude (0x0004, RATIONAL=5, count=3, offset=lonRationalOffset)
		_ = binary.Write(tw.buf, order, uint16(0x0004))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, lonRationalOffset)

		// Next IFD = 0
		_ = binary.Write(tw.buf, order, uint32(0))

		// Extra data:
		// Date string: "2026:09:19 18:15:00\x00" (offset 158)
		tw.buf.Write([]byte("2026:09:19 18:15:00\x00"))

		// Latitude rationals: (offset 178)
		// 47/1 deg, 36/1 min, 468/10 sec (46.8 sec) -> 47 + 36/60 + 46.8/3600 = 47.613
		_ = binary.Write(tw.buf, order, uint32(47))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(36))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(468))
		_ = binary.Write(tw.buf, order, uint32(10))

		// Longitude rationals: (offset 202)
		// 122/1 deg, 20/1 min, 132/10 sec (13.2 sec) -> 122 + 20/60 + 13.2/3600 = 122.337
		_ = binary.Write(tw.buf, order, uint32(122))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(20))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(132))
		_ = binary.Write(tw.buf, order, uint32(10))
	})

	jpegBytes := buildJPEGWithEXIF(rawTIFF, 4032, 3024)

	meta, err := imaging.ExtractMetadata(jpegBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Orientation != 6 {
		t.Errorf("expected Orientation=6, got %d", meta.Orientation)
	}
	if meta.Width != 4032 {
		t.Errorf("expected Width=4032, got %d", meta.Width)
	}
	if meta.Height != 3024 {
		t.Errorf("expected Height=3024, got %d", meta.Height)
	}
	if meta.CaptureTime == nil {
		t.Fatalf("expected CaptureTime to be non-nil")
	}
	expectedTime := time.Date(2026, 9, 19, 18, 15, 0, 0, time.UTC)
	if !meta.CaptureTime.Equal(expectedTime) {
		t.Errorf("expected CaptureTime=%v, got %v", expectedTime, *meta.CaptureTime)
	}

	if meta.GPS == nil {
		t.Fatalf("expected GPS to be non-nil")
	}

	if math.Abs(meta.GPS.Latitude-47.613) > 1e-6 {
		t.Errorf("expected Latitude=47.613, got %f", meta.GPS.Latitude)
	}
	if math.Abs(meta.GPS.Longitude-(-122.337)) > 1e-6 {
		t.Errorf("expected Longitude=-122.337, got %f", meta.GPS.Longitude)
	}
}

func TestExtractMetadata_BigEndian_SouthEast(t *testing.T) {
	t.Parallel()

	// Sydney coordinates:
	// Lat: 33 deg 52 min 7.68 sec S -> -33.8688
	// Lon: 151 deg 12 min 33.48 sec E -> 151.2093
	order := binary.BigEndian
	rawTIFF := buildSyntheticTIFF(order, false, func(tw *tiffWriter) {
		exifIFDOffset := uint32(50)
		gpsIFDOffset := uint32(68)
		dateStringOffset := uint32(122)
		latRationalOffset := uint32(142)
		lonRationalOffset := uint32(166)

		// IFD0 (3 entries)
		_ = binary.Write(tw.buf, order, uint16(3))

		// Orientation (1)
		_ = binary.Write(tw.buf, order, uint16(0x0112))
		_ = binary.Write(tw.buf, order, uint16(3))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0))

		// ExifIFDPointer
		_ = binary.Write(tw.buf, order, uint16(0x8769))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, exifIFDOffset)

		// GPSInfoIFDPointer
		_ = binary.Write(tw.buf, order, uint16(0x8825))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, gpsIFDOffset)

		_ = binary.Write(tw.buf, order, uint32(0))

		// Exif SubIFD (1 entry: DateTimeDigitized)
		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x9004))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(20))
		_ = binary.Write(tw.buf, order, dateStringOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// GPS SubIFD (4 entries)
		_ = binary.Write(tw.buf, order, uint16(4))

		// GPSLatitudeRef "S"
		_ = binary.Write(tw.buf, order, uint16(0x0001))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("S\x00\x00\x00"))

		// GPSLatitude
		_ = binary.Write(tw.buf, order, uint16(0x0002))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, latRationalOffset)

		// GPSLongitudeRef "E"
		_ = binary.Write(tw.buf, order, uint16(0x0003))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("E\x00\x00\x00"))

		// GPSLongitude
		_ = binary.Write(tw.buf, order, uint16(0x0004))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, lonRationalOffset)

		_ = binary.Write(tw.buf, order, uint32(0))

		// Extra data:
		// Date string
		tw.buf.Write([]byte("2026:09:19 12:30:00\x00"))

		// Lat rationals: 33 deg, 52 min, 768/100 sec -> 33 + 52/60 + 7.68/3600 = 33.8688
		_ = binary.Write(tw.buf, order, uint32(33))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(52))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(768))
		_ = binary.Write(tw.buf, order, uint32(100))

		// Lon rationals: 151 deg, 12 min, 3348/100 sec -> 151 + 12/60 + 33.48/3600 = 151.2093
		_ = binary.Write(tw.buf, order, uint32(151))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(12))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(3348))
		_ = binary.Write(tw.buf, order, uint32(100))
	})

	jpegBytes := buildJPEGWithEXIF(rawTIFF, 1920, 1080)

	meta, err := imaging.ExtractMetadata(jpegBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Orientation != 1 {
		t.Errorf("expected Orientation=1, got %d", meta.Orientation)
	}
	if meta.Width != 1920 {
		t.Errorf("expected Width=1920, got %d", meta.Width)
	}
	if meta.Height != 1080 {
		t.Errorf("expected Height=1080, got %d", meta.Height)
	}
	if meta.CaptureTime == nil {
		t.Fatalf("expected CaptureTime to be non-nil")
	}
	expectedTime := time.Date(2026, 9, 19, 12, 30, 0, 0, time.UTC)
	if !meta.CaptureTime.Equal(expectedTime) {
		t.Errorf("expected CaptureTime=%v, got %v", expectedTime, *meta.CaptureTime)
	}

	if meta.GPS == nil {
		t.Fatalf("expected GPS to be non-nil")
	}

	// Should be negative for South
	if math.Abs(meta.GPS.Latitude-(-33.8688)) > 1e-4 {
		t.Errorf("expected Latitude=-33.8688, got %f", meta.GPS.Latitude)
	}
	// Should be positive for East
	if math.Abs(meta.GPS.Longitude-151.2093) > 1e-4 {
		t.Errorf("expected Longitude=151.2093, got %f", meta.GPS.Longitude)
	}
}

func TestExtractMetadata_StandaloneTIFF(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		// IFD0 with Width, Height, Orientation
		_ = binary.Write(tw.buf, order, uint16(3))

		// ImageWidth (tag 0x0100) = 640
		_ = binary.Write(tw.buf, order, uint16(0x0100))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(640))

		// ImageLength (tag 0x0101) = 480
		_ = binary.Write(tw.buf, order, uint16(0x0101))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(480))

		// Orientation (tag 0x0112) = 3
		_ = binary.Write(tw.buf, order, uint16(0x0112))
		_ = binary.Write(tw.buf, order, uint16(3))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint16(3))
		_ = binary.Write(tw.buf, order, uint16(0))

		_ = binary.Write(tw.buf, order, uint32(0))
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error parsing standalone TIFF: %v", err)
	}

	if meta.Width != 640 || meta.Height != 480 {
		t.Errorf("expected 640x480, got %dx%d", meta.Width, meta.Height)
	}
	if meta.Orientation != 3 {
		t.Errorf("expected orientation 3, got %d", meta.Orientation)
	}
	if meta.GPS != nil {
		t.Errorf("expected nil GPS, got %+v", meta.GPS)
	}
}

func TestExtractMetadata_DivisionByZeroSafety(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		gpsIFDOffset := uint32(26)
		latOffset := uint32(56)

		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x8825))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, gpsIFDOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// GPS IFD
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint16(0x0001))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("N\x00\x00\x00"))

		_ = binary.Write(tw.buf, order, uint16(0x0002))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, latOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// Lat rationals with zero denominator: 47/0, 36/0, 10/0
		for i := 0; i < 3; i++ {
			_ = binary.Write(tw.buf, order, uint32(47))
			_ = binary.Write(tw.buf, order, uint32(0)) // 0 denominator!
		}
	})

	jpegBytes := buildJPEGWithEXIF(rawTIFF, 100, 100)

	// Must not panic!
	meta, err := imaging.ExtractMetadata(jpegBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GPS longitude was missing, so GPS should be nil
	if meta.GPS != nil {
		t.Errorf("expected nil GPS when coordinates are incomplete, got %+v", meta.GPS)
	}
}

func TestExtractMetadata_DimensionSafetyGuards(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		_ = binary.Write(tw.buf, order, uint16(2))

		// ImageWidth exceeds 65535 limit
		_ = binary.Write(tw.buf, order, uint16(0x0100))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(1000000))

		// ImageLength
		_ = binary.Write(tw.buf, order, uint16(0x0101))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(500))

		_ = binary.Write(tw.buf, order, uint32(0))
	})

	_, err := imaging.ExtractMetadata(rawTIFF)
	if err == nil {
		t.Fatalf("expected error for excessive image dimensions, got nil")
	}
}

func TestExtractMetadata_TimeWithTimezoneOffset(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		exifIFDOffset := uint32(26)
		dateStrOffset := uint32(56)
		offsetStrOffset := uint32(76)

		// IFD0: 1 entry (ExifIFDPointer)
		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x8769))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, exifIFDOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// Exif SubIFD: 2 entries (DateTimeOriginal, OffsetTimeOriginal)
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint16(0x9003)) // DateTimeOriginal
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(20))
		_ = binary.Write(tw.buf, order, dateStrOffset)

		_ = binary.Write(tw.buf, order, uint16(0x9010)) // OffsetTimeOriginal
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(7))
		_ = binary.Write(tw.buf, order, offsetStrOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// String payloads
		tw.buf.Write([]byte("2026:09:19 11:15:00\x00")) // offset 56 (20 bytes: 56..76)
		tw.buf.Write([]byte("-07:00\x00"))              // offset 76 (7 bytes: 76..83)
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CaptureTime == nil {
		t.Fatalf("expected non-nil CaptureTime")
	}
	// 11:15:00 at -07:00 is 18:15:00 UTC
	expected := time.Date(2026, 9, 19, 18, 15, 0, 0, time.UTC)
	if !meta.CaptureTime.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, *meta.CaptureTime)
	}
}

func TestExtractMetadata_GPSDateTimeFallback(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		gpsIFDOffset := uint32(26)
		dateStrOffset := uint32(56)
		timeRatsOffset := uint32(68)

		// IFD0
		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x8825))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, gpsIFDOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// GPS SubIFD: GPSDateStamp, GPSTimeStamp
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint16(0x001D)) // GPSDateStamp
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(11))
		_ = binary.Write(tw.buf, order, dateStrOffset)

		_ = binary.Write(tw.buf, order, uint16(0x0007)) // GPSTimeStamp
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, timeRatsOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// String: "2026:09:19\x00\x00" (12 bytes: 56..68)
		tw.buf.Write([]byte("2026:09:19\x00\x00"))

		// Rationals: 18/1, 15/1, 0/1 (offset 68)
		_ = binary.Write(tw.buf, order, uint32(18))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(15))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(0))
		_ = binary.Write(tw.buf, order, uint32(1))
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CaptureTime == nil {
		t.Fatalf("expected non-nil CaptureTime")
	}
	expected := time.Date(2026, 9, 19, 18, 15, 0, 0, time.UTC)
	if !meta.CaptureTime.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, *meta.CaptureTime)
	}
}

func TestExtractMetadata_IFD0DateTimeFallback(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		dateOffset := uint32(26)

		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x0132)) // DateTime
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(20))
		_ = binary.Write(tw.buf, order, dateOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		tw.buf.Write([]byte("2026:09:19 14:00:00\x00"))
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CaptureTime == nil {
		t.Fatalf("expected non-nil CaptureTime")
	}
	expected := time.Date(2026, 9, 19, 14, 0, 0, 0, time.UTC)
	if !meta.CaptureTime.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, *meta.CaptureTime)
	}
}

func TestExtractMetadata_JPEG_SOFDimensionsFallback(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		// Only Orientation in IFD0, no width or height tags
		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x0112))
		_ = binary.Write(tw.buf, order, uint16(3))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint16(0))
		_ = binary.Write(tw.buf, order, uint32(0))
	})

	// SOF0 has width=1280, height=720
	jpegBytes := buildJPEGWithEXIF(rawTIFF, 1280, 720)

	meta, err := imaging.ExtractMetadata(jpegBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Width != 1280 || meta.Height != 720 {
		t.Errorf("expected 1280x720 from SOF fallback, got %dx%d", meta.Width, meta.Height)
	}
	if meta.Orientation != 5 {
		t.Errorf("expected orientation 5, got %d", meta.Orientation)
	}
}

func TestExtractMetadata_CircularIFD(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		// ExifIFDPointer points right back to IFD0 offset 8!
		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x8769))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(8))
		_ = binary.Write(tw.buf, order, uint32(0))
	})

	// Must terminate safely without infinite recursion
	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Orientation != 1 {
		t.Errorf("expected default orientation 1, got %d", meta.Orientation)
	}
}

func TestExtractMetadata_CorruptTIFFHeaders(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		data []byte
	}{
		{"wrong magic", []byte{'I', 'I', 0x99, 0x00, 0x08, 0x00, 0x00, 0x00}},
		{"out of bounds IFD offset", []byte{'I', 'I', 0x2A, 0x00, 0xFF, 0xFF, 0x00, 0x00}},
		{"truncated IFD count", []byte{'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, 0x05}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := imaging.ExtractMetadata(tc.data)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !errors.Is(err, imaging.ErrCorruptExif) {
				t.Fatalf("expected ErrCorruptExif, got %v", err)
			}
		})
	}
}

func TestExtractMetadata_CorruptJPEGMarkers(t *testing.T) {
	t.Parallel()

	// Truncated segment length in JPEG
	data := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x50, 0x45}
	_, err := imaging.ExtractMetadata(data)
	if err == nil {
		t.Fatalf("expected error for truncated JPEG, got nil")
	}
}

func TestExtractMetadata_InvalidOrientationFallback(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		_ = binary.Write(tw.buf, order, uint16(1))
		// Orientation = 99 (invalid)
		_ = binary.Write(tw.buf, order, uint16(0x0112))
		_ = binary.Write(tw.buf, order, uint16(3))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint16(99))
		_ = binary.Write(tw.buf, order, uint16(0))
		_ = binary.Write(tw.buf, order, uint32(0))
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Orientation != 1 {
		t.Errorf("expected orientation to fallback to 1, got %d", meta.Orientation)
	}
}

func TestExtractMetadata_OutOfRangeCoordinates(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		gpsIFDOffset := uint32(26)
		latOffset := uint32(80)
		lonOffset := uint32(104)

		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x8825))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, gpsIFDOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// GPS SubIFD: 4 entries
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint16(0x0001))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("N\x00\x00\x00"))

		_ = binary.Write(tw.buf, order, uint16(0x0002))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, latOffset)

		_ = binary.Write(tw.buf, order, uint16(0x0003))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("W\x00\x00\x00"))

		_ = binary.Write(tw.buf, order, uint16(0x0004))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, lonOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// Latitude: 95 degrees (invalid, > 90)
		_ = binary.Write(tw.buf, order, uint32(95))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(0))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(0))
		_ = binary.Write(tw.buf, order, uint32(1))

		// Longitude: 122 degrees
		_ = binary.Write(tw.buf, order, uint32(122))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(0))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint32(0))
		_ = binary.Write(tw.buf, order, uint32(1))
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.GPS != nil {
		t.Errorf("expected nil GPS for out-of-bounds latitude, got %+v", meta.GPS)
	}
}

func TestExtractMetadata_NullIslandCoordinatesIgnored(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		gpsIFDOffset := uint32(26)
		latOffset := uint32(80)
		lonOffset := uint32(104)

		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x8825))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, gpsIFDOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// GPS SubIFD: 4 entries
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint16(0x0001))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("N\x00\x00\x00"))

		_ = binary.Write(tw.buf, order, uint16(0x0002))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, latOffset)

		_ = binary.Write(tw.buf, order, uint16(0x0003))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("E\x00\x00\x00"))

		_ = binary.Write(tw.buf, order, uint16(0x0004))
		_ = binary.Write(tw.buf, order, uint16(5))
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, lonOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// Latitude: 0/1, 0/1, 0/1 (0.0)
		for i := 0; i < 3; i++ {
			_ = binary.Write(tw.buf, order, uint32(0))
			_ = binary.Write(tw.buf, order, uint32(1))
		}

		// Longitude: 0/1, 0/1, 0/1 (0.0)
		for i := 0; i < 3; i++ {
			_ = binary.Write(tw.buf, order, uint32(0))
			_ = binary.Write(tw.buf, order, uint32(1))
		}
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.GPS != nil {
		t.Errorf("expected nil GPS for Null Island (0.0, 0.0), got %+v", meta.GPS)
	}
}

func TestExtractMetadata_SRATIONAL_Signed(t *testing.T) {
	t.Parallel()

	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		gpsIFDOffset := uint32(26)
		latOffset := uint32(80)
		lonOffset := uint32(104)

		_ = binary.Write(tw.buf, order, uint16(1))
		_ = binary.Write(tw.buf, order, uint16(0x8825))
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, gpsIFDOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// GPS SubIFD: 4 entries with fieldType 10 (SRATIONAL)
		_ = binary.Write(tw.buf, order, uint16(4))
		_ = binary.Write(tw.buf, order, uint16(0x0001))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("N\x00\x00\x00"))

		_ = binary.Write(tw.buf, order, uint16(0x0002))
		_ = binary.Write(tw.buf, order, uint16(10)) // SRATIONAL (type 10)
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, latOffset)

		_ = binary.Write(tw.buf, order, uint16(0x0003))
		_ = binary.Write(tw.buf, order, uint16(2))
		_ = binary.Write(tw.buf, order, uint32(2))
		tw.buf.Write([]byte("W\x00\x00\x00"))

		_ = binary.Write(tw.buf, order, uint16(0x0004))
		_ = binary.Write(tw.buf, order, uint16(10)) // SRATIONAL (type 10)
		_ = binary.Write(tw.buf, order, uint32(3))
		_ = binary.Write(tw.buf, order, lonOffset)
		_ = binary.Write(tw.buf, order, uint32(0))

		// Latitude: 45/1, 30/1, 0/1 -> 45.5
		_ = binary.Write(tw.buf, order, int32(45))
		_ = binary.Write(tw.buf, order, int32(1))
		_ = binary.Write(tw.buf, order, int32(30))
		_ = binary.Write(tw.buf, order, int32(1))
		_ = binary.Write(tw.buf, order, int32(0))
		_ = binary.Write(tw.buf, order, int32(1))

		// Longitude: -90/1 (negative in DMS is unusual, but tests int32 decode): -90 deg -> -90.0, with Ref 'W' (-(-90)) or 90/1
		_ = binary.Write(tw.buf, order, int32(90))
		_ = binary.Write(tw.buf, order, int32(1))
		_ = binary.Write(tw.buf, order, int32(15))
		_ = binary.Write(tw.buf, order, int32(1))
		_ = binary.Write(tw.buf, order, int32(0))
		_ = binary.Write(tw.buf, order, int32(1))
	})

	meta, err := imaging.ExtractMetadata(rawTIFF)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.GPS == nil {
		t.Fatalf("expected non-nil GPS")
	}
	if math.Abs(meta.GPS.Latitude-45.5) > 1e-4 {
		t.Errorf("expected lat 45.5, got %f", meta.GPS.Latitude)
	}
}
