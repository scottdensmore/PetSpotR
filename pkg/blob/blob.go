package blob

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNotFound indicates the requested blob image was not found.
var ErrNotFound = errors.New("blob: image not found")

var (
	// ErrInvalidImage indicates that an upload is not a supported, valid image.
	ErrInvalidImage = errors.New("blob: invalid image")
	// ErrUploadMismatch indicates that an upload does not belong to the report being finalized.
	ErrUploadMismatch = errors.New("blob: upload does not belong to report")
	// ErrUploadExpired indicates that the upload can no longer create report state.
	ErrUploadExpired = errors.New("blob: upload finalize capability expired")
	// ErrNotFinalized indicates that a private read was requested before validation completed.
	ErrNotFinalized = errors.New("blob: image is not finalized")
)

const (
	// MaxImageBytes is the largest accepted pet image.
	MaxImageBytes     int64 = 10 << 20
	uploadExpiry            = 15 * time.Minute
	maxReadExpiry           = 15 * time.Minute
	maxImageDimension       = 12_000
	maxImagePixels          = 40_000_000
	// DefaultOrphanGracePeriod prevents cleanup from racing active report creation.
	DefaultOrphanGracePeriod = 24 * time.Hour
	// MaxOrphanCleanupBatch bounds one reconciliation pass.
	MaxOrphanCleanupBatch = 100
)

// ImagePurpose scopes a generated object key to an application workflow.
type ImagePurpose string

// ImagePurposeFoundPet identifies an image for a found-pet report.
const ImagePurposeFoundPet ImagePurpose = "found-pet"

// ImageUploadIntent contains the trusted attributes needed to create a direct-upload grant.
// FileName is advisory only and is never used in the generated object key.
type ImageUploadIntent struct {
	Purpose     ImagePurpose `json:"purpose"`
	FileName    string       `json:"fileName,omitempty"`
	ContentType string       `json:"contentType"`
}

// ImageUploadGrant describes a short-lived, policy-constrained direct upload.
type ImageUploadGrant struct {
	UploadURL     string            `json:"uploadUrl"`
	FormFields    map[string]string `json:"formFields"`
	ReportID      string            `json:"reportId"`
	FinalizeToken string            `json:"finalizeToken"`
	ObjectName    string            `json:"objectName"`
	ContentType   string            `json:"contentType"`
	MaxBytes      int64             `json:"maxBytes"`
	ExpiresAt     time.Time         `json:"expiresAt"`
}

// FinalizedImage is a validated private image object.
type FinalizedImage struct {
	ObjectName  string `json:"objectName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Generation  int64  `json:"generation"`
}

// ImageStore defines the secure direct-upload and private-read lifecycle.
type ImageStore interface {
	BeginImageUpload(ctx context.Context, intent ImageUploadIntent) (*ImageUploadGrant, error)
	FinalizeImage(ctx context.Context, reportID, objectName, finalizeToken string) (*FinalizedImage, error)
	ReadFinalizedImage(ctx context.Context, objectName string) ([]byte, error)
	GenerateImageReadURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
}

// FinalizedImageReferenceChecker reports whether durable report state refers
// to the exact finalized object.
type FinalizedImageReferenceChecker func(ctx context.Context, reportID, objectName string) (bool, error)

// OrphanCleaner reconciles finalized images left behind by a process crash or
// report-persistence failure.
type OrphanCleaner interface {
	CleanupOrphanedFinalizedImages(
		ctx context.Context,
		createdBefore time.Time,
		limit int,
		referenced FinalizedImageReferenceChecker,
	) (int, error)
}

// PresignedURLResponse encapsulates direct-to-cloud upload pre-signed parameters.
type PresignedURLResponse struct {
	UploadURL string    `json:"uploadUrl"`
	PublicURL string    `json:"publicUrl"`
	FileName  string    `json:"fileName"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// BlobStore defines image/blob storage operations.
type BlobStore interface {
	UploadImage(ctx context.Context, fileName string, data []byte) (string, error)
	GetImage(ctx context.Context, fileName string) ([]byte, error)
	GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string, expiry time.Duration) (*PresignedURLResponse, error)
}

// MemoryBlobStore implements BlobStore in memory for testing and local dev.
type MemoryBlobStore struct {
	mu                sync.RWMutex
	baseURL           string
	blobs             map[string][]byte
	pending           map[string]pendingImageUpload
	finalized         map[string]FinalizedImage
	finalizedByUpload map[string]string
	finalizeTokens    map[string]string
	finalizeBefore    map[string]time.Time
	now               func() time.Time
}

type pendingImageUpload struct {
	reportID    string
	contentType string
	finalName   string
	expiresAt   time.Time
	tokenDigest string
}

// NewMemoryBlobStore constructs a MemoryBlobStore with a base URL prefix.
func NewMemoryBlobStore(baseURL string) *MemoryBlobStore {
	return &MemoryBlobStore{
		baseURL:           strings.TrimSuffix(baseURL, "/"),
		blobs:             make(map[string][]byte),
		pending:           make(map[string]pendingImageUpload),
		finalized:         make(map[string]FinalizedImage),
		finalizedByUpload: make(map[string]string),
		finalizeTokens:    make(map[string]string),
		finalizeBefore:    make(map[string]time.Time),
		now:               time.Now,
	}
}

// UploadImage stores image bytes in memory and returns its public URL.
func (m *MemoryBlobStore) UploadImage(ctx context.Context, fileName string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.blobs[fileName] = bytes.Clone(data)
	return fmt.Sprintf("%s/%s", m.baseURL, fileName), nil
}

// GetImage retrieves image bytes by fileName.
func (m *MemoryBlobStore) GetImage(ctx context.Context, fileName string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.blobs[fileName]
	if !exists {
		return nil, ErrNotFound
	}
	return bytes.Clone(data), nil
}

// GeneratePresignedUploadURL constructs a pre-signed HTTP PUT upload URL for direct cloud storage uploads.
func (m *MemoryBlobStore) GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string, expiry time.Duration) (*PresignedURLResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	cleanFileName := strings.TrimSpace(fileName)
	if cleanFileName == "" {
		cleanFileName = fmt.Sprintf("upload-%d.jpg", time.Now().UnixNano())
	}

	expiresAt := time.Now().UTC().Add(expiry)
	token, err := randomHex(24)
	if err != nil {
		return nil, err
	}
	uploadURL := fmt.Sprintf("%s/upload/presigned/%s?expires=%d&token=%s", m.baseURL, cleanFileName, expiresAt.Unix(), token)
	publicURL := fmt.Sprintf("%s/%s", m.baseURL, cleanFileName)

	return &PresignedURLResponse{
		UploadURL: uploadURL,
		PublicURL: publicURL,
		FileName:  cleanFileName,
		ExpiresAt: expiresAt,
	}, nil
}

// BeginImageUpload creates a server-owned object name and constrained upload grant.
func (m *MemoryBlobStore) BeginImageUpload(ctx context.Context, intent ImageUploadIntent) (*ImageUploadGrant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if intent.Purpose != ImagePurposeFoundPet {
		return nil, fmt.Errorf("%w: unsupported purpose %q", ErrInvalidImage, intent.Purpose)
	}

	contentType, extension, ok := supportedImageType(intent.ContentType)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported content type %q", ErrInvalidImage, intent.ContentType)
	}
	reportToken, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	uploadToken, err := randomHex(24)
	if err != nil {
		return nil, err
	}
	finalizeToken, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	reportID := "found-" + reportToken
	objectName := fmt.Sprintf("uploads/found-pets/%s/image%s", reportID, extension)
	finalName := fmt.Sprintf("images/found-pets/%s/image%s", reportID, extension)
	expiresAt := m.now().UTC().Add(uploadExpiry)

	m.mu.Lock()
	m.pending[objectName] = pendingImageUpload{
		reportID:    reportID,
		contentType: contentType,
		finalName:   finalName,
		expiresAt:   expiresAt,
		tokenDigest: tokenDigest(finalizeToken),
	}
	m.mu.Unlock()

	return &ImageUploadGrant{
		UploadURL: fmt.Sprintf("%s/upload?token=%s", m.baseURL, uploadToken),
		FormFields: map[string]string{
			"key":                            objectName,
			"content-type":                   contentType,
			"x-goog-meta-petspotr-report-id": reportID,
			"x-goog-meta-petspotr-purpose":   string(ImagePurposeFoundPet),
			"x-goog-meta-petspotr-upload-token-sha256": tokenDigest(finalizeToken),
			"x-goog-meta-petspotr-finalize-before":     strconv.FormatInt(expiresAt.Unix(), 10),
		},
		ReportID:      reportID,
		FinalizeToken: finalizeToken,
		ObjectName:    objectName,
		ContentType:   contentType,
		MaxBytes:      MaxImageBytes,
		ExpiresAt:     expiresAt,
	}, nil
}

// FinalizeImage validates an uploaded image and moves it into the private image namespace.
func (m *MemoryBlobStore) FinalizeImage(ctx context.Context, reportID, objectName, finalizeToken string) (*FinalizedImage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if finalName, ok := m.finalizedByUpload[objectName]; ok {
		finalized := m.finalized[finalName]
		if finalizedReportID(finalized.ObjectName) != reportID || !tokenMatches(m.finalizeTokens[objectName], finalizeToken) {
			return nil, ErrUploadMismatch
		}
		if !m.now().UTC().Before(m.finalizeBefore[objectName]) {
			return nil, ErrUploadExpired
		}
		return cloneFinalized(finalized), nil
	}

	pending, ok := m.pending[objectName]
	if !ok || pending.reportID != reportID || !tokenMatches(pending.tokenDigest, finalizeToken) {
		return nil, ErrUploadMismatch
	}
	if !m.now().UTC().Before(pending.expiresAt) {
		return nil, ErrUploadExpired
	}
	data, ok := m.blobs[objectName]
	if !ok {
		return nil, ErrNotFound
	}
	width, height, err := validateImage(data, pending.contentType)
	if err != nil {
		return nil, err
	}
	finalized := FinalizedImage{
		ObjectName:  pending.finalName,
		ContentType: pending.contentType,
		Size:        int64(len(data)),
		Width:       width,
		Height:      height,
		Generation:  1,
	}
	m.blobs[pending.finalName] = bytes.Clone(data)
	delete(m.blobs, objectName)
	delete(m.pending, objectName)
	m.finalized[pending.finalName] = finalized
	m.finalizedByUpload[objectName] = pending.finalName
	m.finalizeTokens[objectName] = pending.tokenDigest
	m.finalizeBefore[objectName] = pending.expiresAt
	return cloneFinalized(finalized), nil
}

// GenerateImageReadURL returns a short-lived capability for a finalized private object.
func (m *MemoryBlobStore) GenerateImageReadURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.mu.RLock()
	_, ok := m.finalized[objectName]
	m.mu.RUnlock()
	if !ok {
		return "", ErrNotFinalized
	}
	if expiry <= 0 || expiry > maxReadExpiry {
		expiry = maxReadExpiry
	}
	token, err := randomHex(24)
	if err != nil {
		return "", err
	}
	query := url.Values{
		"expires": {fmt.Sprintf("%d", time.Now().UTC().Add(expiry).Unix())},
		"token":   {token},
	}
	return fmt.Sprintf("%s/%s?%s", m.baseURL, objectName, query.Encode()), nil
}

// ReadFinalizedImage returns validated private image bytes for an internal consumer.
func (m *MemoryBlobStore) ReadFinalizedImage(ctx context.Context, objectName string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.finalized[objectName]; !ok {
		return nil, ErrNotFinalized
	}
	return bytes.Clone(m.blobs[objectName]), nil
}

func validateImage(data []byte, contentType string) (int, int, error) {
	if len(data) == 0 || int64(len(data)) > MaxImageBytes {
		return 0, 0, fmt.Errorf("%w: size must be between 1 and %d bytes", ErrInvalidImage, MaxImageBytes)
	}
	if detected := http.DetectContentType(data); detected != contentType {
		return 0, 0, fmt.Errorf("%w: declared %s but detected %s", ErrInvalidImage, contentType, detected)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("%w: decode configuration: %w", ErrInvalidImage, err)
	}
	wantFormat := "jpeg"
	if contentType == "image/png" {
		wantFormat = "png"
	}
	if format != wantFormat || config.Width <= 0 || config.Height <= 0 ||
		config.Width > maxImageDimension || config.Height > maxImageDimension ||
		int64(config.Width)*int64(config.Height) > maxImagePixels {
		return 0, 0, fmt.Errorf("%w: unsupported image dimensions or encoding", ErrInvalidImage)
	}
	return config.Width, config.Height, nil
}

func supportedImageType(value string) (string, string, bool) {
	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
	switch contentType {
	case "image/jpeg":
		return contentType, ".jpg", true
	case "image/png":
		return contentType, ".png", true
	default:
		return "", "", false
	}
}

func randomHex(bytesCount int) (string, error) {
	value := make([]byte, bytesCount)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("blob: generate random identifier: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func tokenDigest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func tokenMatches(wantDigest, token string) bool {
	gotDigest := tokenDigest(token)
	return subtle.ConstantTimeCompare([]byte(wantDigest), []byte(gotDigest)) == 1
}

func finalizedReportID(objectName string) string {
	parts := strings.Split(objectName, "/")
	if len(parts) == 4 && parts[0] == "images" && parts[1] == "found-pets" {
		return parts[2]
	}
	return ""
}

func cloneFinalized(value FinalizedImage) *FinalizedImage {
	clone := value
	return &clone
}
