package imaging

import (
	"encoding/binary"
	"errors"
	"strings"
	"time"
)

// Common errors returned by the EXIF metadata extractor.
var (
	ErrInvalidImage           = errors.New("imaging: invalid or unsupported image format")
	ErrNoExif                 = errors.New("imaging: no EXIF metadata found")
	ErrCorruptExif            = errors.New("imaging: corrupted EXIF data")
	ErrDimensionLimitExceeded = errors.New("imaging: image dimensions exceed safe limits")
)

// MaxImageDimension defines the maximum allowed width or height in pixels
// to prevent memory exhaustion and decompression bombs.
const MaxImageDimension = 65535

// GPSCoords represents parsed decimal geographic coordinates.
type GPSCoords struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ImageMetadata contains extracted image properties, GPS coordinates, and capture time.
type ImageMetadata struct {
	GPS         *GPSCoords `json:"gps,omitempty"`
	CaptureTime *time.Time `json:"captureTime,omitempty"`
	Orientation int        `json:"orientation"`
	Width       int        `json:"width"`
	Height      int        `json:"height"`
}

// ExtractMetadata inspects image bytes (JPEG or TIFF) and extracts EXIF metadata,
// including GPS coordinates, orientation, dimensions, and capture timestamps.
func ExtractMetadata(data []byte) (*ImageMetadata, error) {
	if len(data) < 8 {
		return nil, ErrInvalidImage
	}

	// 1. Detect format
	var (
		tiffBytes []byte
		sofWidth  int
		sofHeight int
	)

	if isJPEG(data) {
		var foundApp1 bool
		var err error
		tiffBytes, sofWidth, sofHeight, foundApp1, err = parseJPEG(data)
		if err != nil {
			return nil, err
		}
		if !foundApp1 || len(tiffBytes) == 0 {
			return nil, ErrNoExif
		}
	} else if isTIFF(data) {
		tiffBytes = data
	} else {
		return nil, ErrInvalidImage
	}

	// 2. Parse TIFF structure
	meta, err := parseTIFF(tiffBytes)
	if err != nil {
		return nil, err
	}

	// 3. Backfill dimensions from JPEG SOF if not present in EXIF tags
	if meta.Width == 0 && sofWidth > 0 {
		meta.Width = sofWidth
	}
	if meta.Height == 0 && sofHeight > 0 {
		meta.Height = sofHeight
	}

	// Default orientation is 1 if unspecified or invalid
	if meta.Orientation < 1 || meta.Orientation > 8 {
		meta.Orientation = 1
	}

	// Enforce dimension safety
	if meta.Width > MaxImageDimension || meta.Height > MaxImageDimension || meta.Width < 0 || meta.Height < 0 {
		return nil, ErrDimensionLimitExceeded
	}

	return meta, nil
}

func isJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8
}

func isTIFF(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	isLittle := data[0] == 'I' && data[1] == 'I'
	isBig := data[0] == 'M' && data[1] == 'M'
	return isLittle || isBig
}

func isSOFMarker(marker byte) bool {
	// Baseline, Extended, Progressive, Lossless, Differential
	return (marker >= 0xC0 && marker <= 0xC3) ||
		(marker >= 0xC5 && marker <= 0xC7) ||
		(marker >= 0xC9 && marker <= 0xCB) ||
		(marker >= 0xCD && marker <= 0xCF)
}

func parseJPEG(data []byte) (exifData []byte, sofWidth, sofHeight int, foundApp1 bool, err error) {
	offset := 2 // Skip SOI (0xFF, 0xD8)

	for offset < len(data)-1 {
		if data[offset] != 0xFF {
			offset++
			continue
		}

		// Skip consecutive 0xFF fill bytes
		for offset < len(data) && data[offset] == 0xFF {
			offset++
		}
		if offset >= len(data) {
			break
		}

		marker := data[offset]
		offset++

		// Standalone markers without payload
		if marker == 0xD8 { // SOI
			continue
		}
		if marker == 0xD9 || marker == 0xDA { // EOI or SOS
			break
		}
		if marker >= 0xD0 && marker <= 0xD7 { // RST markers
			continue
		}

		// Markers with payload length (2 bytes big endian)
		if offset+2 > len(data) {
			break
		}
		length := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if length < 2 || offset+length > len(data) {
			return nil, 0, 0, false, ErrCorruptExif
		}

		segment := data[offset+2 : offset+length]

		if marker == 0xE1 { // APP1 EXIF
			if len(segment) >= 6 && string(segment[:6]) == "Exif\x00\x00" {
				exifData = segment[6:]
				foundApp1 = true
			}
		} else if isSOFMarker(marker) {
			if len(segment) >= 5 {
				sofHeight = int(binary.BigEndian.Uint16(segment[1:3]))
				sofWidth = int(binary.BigEndian.Uint16(segment[3:5]))
			}
		}

		offset += length
	}

	return exifData, sofWidth, sofHeight, foundApp1, nil
}

type tiffContext struct {
	data    []byte
	order   binary.ByteOrder
	visited map[uint32]bool
}

func (tc *tiffContext) readUint16(offset uint32) (uint16, bool) {
	if int(offset)+2 > len(tc.data) {
		return 0, false
	}
	return tc.order.Uint16(tc.data[offset : offset+2]), true
}

func (tc *tiffContext) readUint32(offset uint32) (uint32, bool) {
	if int(offset)+4 > len(tc.data) {
		return 0, false
	}
	return tc.order.Uint32(tc.data[offset : offset+4]), true
}

func (tc *tiffContext) readBytes(offset uint32, length uint32) ([]byte, bool) {
	start := int(offset)
	end := start + int(length)
	if start < 0 || end < start || end > len(tc.data) {
		return nil, false
	}
	return tc.data[start:end], true
}

func parseTIFF(data []byte) (*ImageMetadata, error) {
	if len(data) < 8 {
		return nil, ErrCorruptExif
	}

	var order binary.ByteOrder
	if data[0] == 'I' && data[1] == 'I' {
		order = binary.LittleEndian
	} else if data[0] == 'M' && data[1] == 'M' {
		order = binary.BigEndian
	} else {
		return nil, ErrCorruptExif
	}

	tc := &tiffContext{
		data:    data,
		order:   order,
		visited: make(map[uint32]bool),
	}

	magic, ok := tc.readUint16(2)
	if !ok || magic != 42 {
		return nil, ErrCorruptExif
	}

	ifd0Offset, ok := tc.readUint32(4)
	if !ok || ifd0Offset < 8 || int(ifd0Offset) >= len(data) {
		return nil, ErrCorruptExif
	}

	meta := &ImageMetadata{
		Orientation: 1,
	}

	var (
		exifSubIFDOffset uint32
		gpsSubIFDOffset  uint32
		ifd0DateTime     string
	)

	// Parse IFD0
	err := tc.readIFD(ifd0Offset, func(tagID, fieldType uint16, count uint32, valBytes []byte) {
		switch tagID {
		case 0x0112: // Orientation
			if v, ok := tc.parseShort(valBytes); ok {
				meta.Orientation = int(v)
			}
		case 0x0100: // ImageWidth
			if v, ok := tc.parseUint(fieldType, valBytes); ok {
				meta.Width = int(v)
			}
		case 0x0101: // ImageLength (Height)
			if v, ok := tc.parseUint(fieldType, valBytes); ok {
				meta.Height = int(v)
			}
		case 0x0132: // DateTime
			if s, ok := tc.parseString(fieldType, count, valBytes); ok {
				ifd0DateTime = s
			}
		case 0x8769: // ExifIFDPointer
			if v, ok := tc.readUint32FromBytes(valBytes); ok {
				exifSubIFDOffset = v
			}
		case 0x8825: // GPSInfoIFDPointer
			if v, ok := tc.readUint32FromBytes(valBytes); ok {
				gpsSubIFDOffset = v
			}
		}
	})
	if err != nil {
		return nil, err
	}

	var (
		dateTimeOriginal  string
		dateTimeDigitized string
		offsetTime        string
	)

	// Parse Exif SubIFD
	if exifSubIFDOffset > 0 {
		_ = tc.readIFD(exifSubIFDOffset, func(tagID, fieldType uint16, count uint32, valBytes []byte) {
			switch tagID {
			case 0x9003: // DateTimeOriginal
				if s, ok := tc.parseString(fieldType, count, valBytes); ok {
					dateTimeOriginal = s
				}
			case 0x9004: // DateTimeDigitized
				if s, ok := tc.parseString(fieldType, count, valBytes); ok {
					dateTimeDigitized = s
				}
			case 0x9010, 0x9011, 0x9012: // OffsetTimeOriginal / OffsetTimeDigitized / OffsetTime
				if offsetTime == "" {
					if s, ok := tc.parseString(fieldType, count, valBytes); ok {
						offsetTime = s
					}
				}
			case 0xA002: // PixelXDimension (Width)
				if v, ok := tc.parseUint(fieldType, valBytes); ok && meta.Width == 0 {
					meta.Width = int(v)
				}
			case 0xA003: // PixelYDimension (Height)
				if v, ok := tc.parseUint(fieldType, valBytes); ok && meta.Height == 0 {
					meta.Height = int(v)
				}
			}
		})
	}

	// Parse GPS SubIFD
	var (
		gpsLatRef    string
		gpsLatRats   []float64
		gpsLonRef    string
		gpsLonRats   []float64
		gpsDateStamp string
		gpsTimeStamp []float64
	)

	if gpsSubIFDOffset > 0 {
		_ = tc.readIFD(gpsSubIFDOffset, func(tagID, fieldType uint16, count uint32, valBytes []byte) {
			switch tagID {
			case 0x0001: // GPSLatitudeRef
				if s, ok := tc.parseString(fieldType, count, valBytes); ok {
					gpsLatRef = strings.ToUpper(strings.TrimSpace(s))
				}
			case 0x0002: // GPSLatitude
				if rats, ok := tc.parseRationals(fieldType, count, valBytes); ok {
					gpsLatRats = rats
				}
			case 0x0003: // GPSLongitudeRef
				if s, ok := tc.parseString(fieldType, count, valBytes); ok {
					gpsLonRef = strings.ToUpper(strings.TrimSpace(s))
				}
			case 0x0004: // GPSLongitude
				if rats, ok := tc.parseRationals(fieldType, count, valBytes); ok {
					gpsLonRats = rats
				}
			case 0x0007: // GPSTimeStamp
				if rats, ok := tc.parseRationals(fieldType, count, valBytes); ok {
					gpsTimeStamp = rats
				}
			case 0x001D: // GPSDateStamp
				if s, ok := tc.parseString(fieldType, count, valBytes); ok {
					gpsDateStamp = strings.TrimSpace(s)
				}
			}
		})
	}

	// Convert GPS coordinates if both lat and lon are present
	if len(gpsLatRats) == 3 && len(gpsLonRats) == 3 {
		lat := dmsToDecimal(gpsLatRats[0], gpsLatRats[1], gpsLatRats[2])
		if gpsLatRef == "S" {
			lat = -lat
		}

		lon := dmsToDecimal(gpsLonRats[0], gpsLonRats[1], gpsLonRats[2])
		if gpsLonRef == "W" {
			lon = -lon
		}

		if lat >= -90.0 && lat <= 90.0 && lon >= -180.0 && lon <= 180.0 {
			meta.GPS = &GPSCoords{
				Latitude:  lat,
				Longitude: lon,
			}
		}
	}

	// Resolve CaptureTime
	meta.CaptureTime = resolveCaptureTime(
		dateTimeOriginal,
		dateTimeDigitized,
		ifd0DateTime,
		offsetTime,
		gpsDateStamp,
		gpsTimeStamp,
	)

	return meta, nil
}

func (tc *tiffContext) readIFD(offset uint32, fn func(tagID, fieldType uint16, count uint32, valBytes []byte)) error {
	if tc.visited[offset] {
		return nil // Cycle detected, stop safely
	}
	tc.visited[offset] = true

	numEntries, ok := tc.readUint16(offset)
	if !ok {
		return ErrCorruptExif
	}

	const maxEntries = 1024
	if numEntries > maxEntries {
		return ErrCorruptExif
	}

	entryOffset := offset + 2
	for i := uint16(0); i < numEntries; i++ {
		cur := entryOffset + uint32(i)*12
		tagID, ok1 := tc.readUint16(cur)
		fieldType, ok2 := tc.readUint16(cur + 2)
		count, ok3 := tc.readUint32(cur + 4)
		valBytes, ok4 := tc.readBytes(cur+8, 4)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return ErrCorruptExif
		}

		fn(tagID, fieldType, count, valBytes)
	}

	return nil
}

func (tc *tiffContext) readUint32FromBytes(valBytes []byte) (uint32, bool) {
	if len(valBytes) < 4 {
		return 0, false
	}
	return tc.order.Uint32(valBytes[:4]), true
}

func (tc *tiffContext) parseShort(valBytes []byte) (uint16, bool) {
	if len(valBytes) < 2 {
		return 0, false
	}
	return tc.order.Uint16(valBytes[:2]), true
}

func (tc *tiffContext) parseUint(fieldType uint16, valBytes []byte) (uint32, bool) {
	switch fieldType {
	case 3: // SHORT
		s, ok := tc.parseShort(valBytes)
		return uint32(s), ok
	case 4: // LONG
		return tc.readUint32FromBytes(valBytes)
	default:
		return 0, false
	}
}

func (tc *tiffContext) parseString(fieldType uint16, count uint32, valBytes []byte) (string, bool) {
	if fieldType != 2 && fieldType != 7 { // ASCII or UNDEFINED
		return "", false
	}
	if count == 0 {
		return "", false
	}
	// Bound count against total buffer size
	if count > uint32(len(tc.data)) {
		return "", false
	}

	var raw []byte
	if count <= 4 {
		raw = valBytes[:count]
	} else {
		offset, ok := tc.readUint32FromBytes(valBytes)
		if !ok {
			return "", false
		}
		var ok2 bool
		raw, ok2 = tc.readBytes(offset, count)
		if !ok2 {
			return "", false
		}
	}

	str := strings.TrimRight(string(raw), "\x00 \t\r\n")
	return str, true
}

func (tc *tiffContext) parseRationals(fieldType uint16, count uint32, valBytes []byte) ([]float64, bool) {
	if fieldType != 5 && fieldType != 10 { // RATIONAL or SRATIONAL
		return nil, false
	}
	if count == 0 || count > 100 {
		return nil, false
	}

	totalBytes := count * 8
	if totalBytes > uint32(len(tc.data)) {
		return nil, false
	}

	offset, ok := tc.readUint32FromBytes(valBytes)
	if !ok {
		return nil, false
	}

	raw, ok := tc.readBytes(offset, totalBytes)
	if !ok {
		return nil, false
	}

	results := make([]float64, count)
	for i := uint32(0); i < count; i++ {
		chunk := raw[i*8 : (i+1)*8]
		num := tc.order.Uint32(chunk[:4])
		den := tc.order.Uint32(chunk[4:8])
		if den == 0 {
			results[i] = 0
		} else {
			results[i] = float64(num) / float64(den)
		}
	}

	return results, true
}

func dmsToDecimal(deg, min, sec float64) float64 {
	return deg + (min / 60.0) + (sec / 3600.0)
}

func resolveCaptureTime(
	dtOriginal, dtDigitized, dtIFD0, offset string,
	gpsDate string, gpsTime []float64,
) *time.Time {
	candidates := []string{dtOriginal, dtDigitized, dtIFD0}

	for _, raw := range candidates {
		if raw == "" {
			continue
		}
		if t, ok := parseTimeString(raw, offset); ok {
			return &t
		}
	}

	// Fallback to GPS Date & Time if available
	if gpsDate != "" && len(gpsTime) >= 3 {
		// gpsDate is typically "YYYY:MM:DD" or "YYYY-MM-DD"
		gpsDateNorm := strings.ReplaceAll(gpsDate, "-", ":")
		parts := strings.Split(gpsDateNorm, ":")
		if len(parts) == 3 {
			if parsed, err := time.Parse("2006:01:02", gpsDateNorm); err == nil {
				year, month, day := parsed.Date()
				h := int(gpsTime[0])
				m := int(gpsTime[1])
				s := int(gpsTime[2])
				frac := gpsTime[2] - float64(s)
				ns := int(frac * 1e9)
				t := time.Date(year, month, day, h, m, s, ns, time.UTC)
				return &t
			}
		}
	}

	return nil
}

func parseTimeString(raw, offset string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	offset = strings.TrimSpace(offset)

	// If offset is provided and raw string doesn't include timezone info
	if offset != "" && !strings.ContainsAny(raw, "+-Z") {
		withOffset := raw + " " + offset
		layouts := []string{
			"2006:01:02 15:04:05 -07:00",
			"2006:01:02 15:04:05 -0700",
			"2006-01-02 15:04:05 -07:00",
			"2006-01-02 15:04:05 -0700",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, withOffset); err == nil {
				return t.UTC(), true
			}
		}
	}

	layouts := []string{
		"2006:01:02 15:04:05 -07:00",
		"2006:01:02 15:04:05 -0700",
		"2006:01:02 15:04:05 MST",
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006:01:02",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t.UTC(), true
		}
	}

	return time.Time{}, false
}
