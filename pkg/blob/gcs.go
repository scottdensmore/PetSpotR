package blob

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	metadataReportID         = "petspotr-report-id"
	metadataPurpose          = "petspotr-purpose"
	metadataWidth            = "petspotr-width"
	metadataHeight           = "petspotr-height"
	metadataUploadToken      = "petspotr-upload-token-sha256"
	metadataFinalizeBefore   = "petspotr-finalize-before"
	metadataReportIDField    = "x-goog-meta-" + metadataReportID
	metadataPurposeField     = "x-goog-meta-" + metadataPurpose
	metadataUploadTokenField = "x-goog-meta-" + metadataUploadToken
	metadataFinalizeField    = "x-goog-meta-" + metadataFinalizeBefore
)

// GCSConfig configures a private Google Cloud Storage image store.
type GCSConfig struct {
	BucketName     string
	Endpoint       string
	GoogleAccessID string
	PrivateKey     []byte
}

// GCSImageStore implements the secure image lifecycle in a private GCS bucket.
type GCSImageStore struct {
	backend       gcsBackend
	cleanupMu     sync.Mutex
	cleanupCursor string
	now           func() time.Time
}

type gcsObjectAttrs struct {
	Name        string
	ContentType string
	Size        int64
	Generation  int64
	Created     time.Time
	Metadata    map[string]string
}

type gcsBackend interface {
	GenerateSignedPostPolicyV4(string, *storage.PostPolicyV4Options) (*storage.PostPolicyV4, error)
	Attrs(context.Context, string) (gcsObjectAttrs, error)
	Read(context.Context, string, int64, int64) ([]byte, error)
	Copy(context.Context, string, string, int64, gcsObjectAttrs) (gcsObjectAttrs, error)
	Delete(context.Context, string, int64) error
	SignedURL(string, *storage.SignedURLOptions) (string, error)
	List(context.Context, string, string, int) ([]gcsObjectAttrs, string, error)
	Close() error
}

type googleStorageBackend struct {
	client         *storage.Client
	bucket         *storage.BucketHandle
	googleAccessID string
	privateKey     []byte
	hostname       string
	insecure       bool
}

// NewGCSImageStore constructs a managed or emulator-backed GCS image store.
// Managed mode leaves signing credentials empty so the supported client
// library can use Application Default Credentials and IAM signBlob.
func NewGCSImageStore(ctx context.Context, config GCSConfig) (*GCSImageStore, error) {
	config.BucketName = strings.TrimSpace(config.BucketName)
	if config.BucketName == "" {
		return nil, errors.New("blob: GCS bucket name is required")
	}
	clientOptions := make([]option.ClientOption, 0, 2)
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint != "" {
		clientOptions = append(clientOptions, option.WithEndpoint(endpoint), option.WithoutAuthentication())
	}
	client, err := storage.NewClient(ctx, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("blob: create GCS client: %w", err)
	}
	backend := &googleStorageBackend{
		client:         client,
		bucket:         client.Bucket(config.BucketName),
		googleAccessID: strings.TrimSpace(config.GoogleAccessID),
		privateKey:     config.PrivateKey,
		hostname:       endpoint,
		insecure:       strings.HasPrefix(endpoint, "http://"),
	}
	return newGCSImageStore(backend), nil
}

func newGCSImageStore(backend gcsBackend) *GCSImageStore {
	return &GCSImageStore{backend: backend, now: time.Now}
}

// BeginImageUpload creates a V4 POST policy constrained to one generated key,
// one supported media type, exact metadata, a short expiry, and MaxImageBytes.
func (s *GCSImageStore) BeginImageUpload(ctx context.Context, intent ImageUploadIntent) (*ImageUploadGrant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	purposePath, ok := imagePathForPurpose(intent.Purpose)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported purpose %q", ErrInvalidImage, intent.Purpose)
	}
	contentType, extension, ok := supportedImageType(intent.ContentType)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported content type %q", ErrInvalidImage, intent.ContentType)
	}
	token, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	reportID := purposePath.reportPrefix + token
	finalizeToken, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	objectName := fmt.Sprintf("uploads/%s/%s/image%s", purposePath.objectSegment, reportID, extension)
	expiresAt := s.now().UTC().Add(uploadExpiry)
	policy, err := s.backend.GenerateSignedPostPolicyV4(objectName, &storage.PostPolicyV4Options{
		Expires: expiresAt,
		Fields: &storage.PolicyV4Fields{
			ContentType:         contentType,
			StatusCodeOnSuccess: http.StatusCreated,
			Metadata: map[string]string{
				metadataReportIDField:    reportID,
				metadataPurposeField:     string(intent.Purpose),
				metadataUploadTokenField: tokenDigest(finalizeToken),
				metadataFinalizeField:    strconv.FormatInt(expiresAt.Unix(), 10),
			},
		},
		Conditions: []storage.PostPolicyV4Condition{
			storage.ConditionContentLengthRange(1, uint64(MaxImageBytes)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("blob: sign GCS upload policy: %w", err)
	}
	return &ImageUploadGrant{
		UploadURL:     policy.URL,
		FormFields:    policy.Fields,
		ReportID:      reportID,
		FinalizeToken: finalizeToken,
		ObjectName:    objectName,
		ContentType:   contentType,
		MaxBytes:      MaxImageBytes,
		ExpiresAt:     expiresAt,
	}, nil
}

// FinalizeImage verifies immutable object attributes and decoded bytes before
// copying the object into the private finalized namespace.
func (s *GCSImageStore) FinalizeImage(ctx context.Context, reportID, objectName, finalizeToken string) (*FinalizedImage, error) {
	contentType, finalName, err := validateUploadBinding(reportID, objectName)
	if err != nil {
		return nil, err
	}
	purpose, _, ok := imagePurposeForPath(reportID, objectName, "uploads")
	if !ok {
		return nil, ErrUploadMismatch
	}

	if attrs, attrsErr := s.backend.Attrs(ctx, finalName); attrsErr == nil {
		if !tokenMatches(attrs.Metadata[metadataUploadToken], finalizeToken) {
			return nil, ErrUploadMismatch
		}
		if err := s.validateFinalizeDeadline(attrs.Metadata); err != nil {
			return nil, err
		}
		return finalizedFromAttrs(reportID, attrs)
	} else if !errors.Is(attrsErr, ErrNotFound) {
		return nil, fmt.Errorf("blob: inspect finalized GCS image: %w", attrsErr)
	}

	attrs, err := s.backend.Attrs(ctx, objectName)
	if err != nil {
		return nil, fmt.Errorf("blob: inspect uploaded GCS image: %w", err)
	}
	if attrs.Name != objectName || attrs.ContentType != contentType ||
		attrs.Metadata[metadataReportID] != reportID ||
		attrs.Metadata[metadataPurpose] != string(purpose) {
		return nil, ErrUploadMismatch
	}
	if !tokenMatches(attrs.Metadata[metadataUploadToken], finalizeToken) {
		return nil, ErrUploadMismatch
	}
	if err := s.validateFinalizeDeadline(attrs.Metadata); err != nil {
		return nil, err
	}
	if attrs.Size <= 0 || attrs.Size > MaxImageBytes {
		return nil, fmt.Errorf("%w: object size %d", ErrInvalidImage, attrs.Size)
	}
	data, err := s.backend.Read(ctx, objectName, attrs.Generation, MaxImageBytes+1)
	if err != nil {
		return nil, fmt.Errorf("blob: read uploaded GCS image: %w", err)
	}
	if int64(len(data)) != attrs.Size {
		return nil, fmt.Errorf("%w: object size changed during validation", ErrInvalidImage)
	}
	width, height, err := validateImage(data, contentType)
	if err != nil {
		return nil, err
	}
	finalAttrs, err := s.backend.Copy(ctx, finalName, objectName, attrs.Generation, gcsObjectAttrs{
		ContentType: contentType,
		Size:        attrs.Size,
		Metadata: map[string]string{
			metadataReportID:       reportID,
			metadataPurpose:        string(purpose),
			metadataWidth:          strconv.Itoa(width),
			metadataHeight:         strconv.Itoa(height),
			metadataUploadToken:    attrs.Metadata[metadataUploadToken],
			metadataFinalizeBefore: attrs.Metadata[metadataFinalizeBefore],
		},
	})
	if err != nil {
		if isPreconditionFailed(err) {
			if existing, attrsErr := s.backend.Attrs(ctx, finalName); attrsErr == nil {
				if !tokenMatches(existing.Metadata[metadataUploadToken], finalizeToken) {
					return nil, ErrUploadMismatch
				}
				if err := s.validateFinalizeDeadline(existing.Metadata); err != nil {
					return nil, err
				}
				return finalizedFromAttrs(reportID, existing)
			}
		}
		return nil, fmt.Errorf("blob: finalize GCS image: %w", err)
	}
	if err := s.backend.Delete(ctx, objectName, attrs.Generation); err != nil {
		return nil, fmt.Errorf("blob: delete temporary GCS image: %w", err)
	}
	return finalizedFromAttrs(reportID, finalAttrs)
}

// FinalizeImageForPurpose rejects a valid capability issued for a different
// report workflow before finalizing it.
func (s *GCSImageStore) FinalizeImageForPurpose(
	ctx context.Context,
	purpose ImagePurpose,
	reportID, objectName, finalizeToken string,
) (*FinalizedImage, error) {
	gotPurpose, _, ok := imagePurposeForPath(reportID, objectName, "uploads")
	if !ok || gotPurpose != purpose {
		return nil, ErrUploadMismatch
	}
	return s.FinalizeImage(ctx, reportID, objectName, finalizeToken)
}

// GenerateImageReadURL creates a private, short-lived V4 GET capability.
func (s *GCSImageStore) GenerateImageReadURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	attrs, err := s.backend.Attrs(ctx, objectName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrNotFinalized
		}
		return "", fmt.Errorf("blob: inspect private GCS image: %w", err)
	}
	if _, err := finalizedFromAttrs(finalizedReportID(objectName), attrs); err != nil {
		return "", ErrNotFinalized
	}
	if expiry <= 0 || expiry > maxReadExpiry {
		expiry = maxReadExpiry
	}
	readURL, err := s.backend.SignedURL(objectName, &storage.SignedURLOptions{
		Method:          http.MethodGet,
		Expires:         s.now().UTC().Add(expiry),
		Scheme:          storage.SigningSchemeV4,
		QueryParameters: nil,
	})
	if err != nil {
		return "", fmt.Errorf("blob: sign private GCS read URL: %w", err)
	}
	return readURL, nil
}

// ReadFinalizedImage reads a validated private object for an internal service.
func (s *GCSImageStore) ReadFinalizedImage(ctx context.Context, objectName string) ([]byte, error) {
	attrs, err := s.backend.Attrs(ctx, objectName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFinalized
		}
		return nil, fmt.Errorf("blob: inspect private GCS image: %w", err)
	}
	if _, err := finalizedFromAttrs(finalizedReportID(objectName), attrs); err != nil {
		return nil, ErrNotFinalized
	}
	if attrs.Size <= 0 || attrs.Size > MaxImageBytes {
		return nil, fmt.Errorf("%w: object size %d", ErrInvalidImage, attrs.Size)
	}
	data, err := s.backend.Read(ctx, objectName, attrs.Generation, MaxImageBytes+1)
	if err != nil {
		return nil, fmt.Errorf("blob: read private GCS image: %w", err)
	}
	if int64(len(data)) != attrs.Size {
		return nil, fmt.Errorf("%w: object size changed during read", ErrInvalidImage)
	}
	return data, nil
}

// CleanupOrphanedFinalizedImages deletes only old, valid finalized objects
// that remain unreferenced across two durable-state checks. Listing is
// cursor-backed and bounded; deletion is pinned to the inspected generation.
func (s *GCSImageStore) CleanupOrphanedFinalizedImages(
	ctx context.Context,
	createdBefore time.Time,
	limit int,
	referenced FinalizedImageReferenceChecker,
) (int, error) {
	return s.CleanupOrphanedFinalizedImagesForPurpose(
		ctx, ImagePurposeFoundPet, createdBefore, limit, referenced,
	)
}

// CleanupOrphanedFinalizedImagesForPurpose scopes reconciliation to one report
// workflow's private namespace.
func (s *GCSImageStore) CleanupOrphanedFinalizedImagesForPurpose(
	ctx context.Context,
	purpose ImagePurpose,
	createdBefore time.Time,
	limit int,
	referenced FinalizedImageReferenceChecker,
) (int, error) {
	if referenced == nil {
		return 0, errors.New("blob: finalized image reference checker is required")
	}
	purposePath, ok := imagePathForPurpose(purpose)
	if !ok {
		return 0, fmt.Errorf("%w: unsupported purpose %q", ErrInvalidImage, purpose)
	}
	if limit <= 0 || limit > MaxOrphanCleanupBatch {
		limit = MaxOrphanCleanupBatch
	}
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()

	objects, nextCursor, err := s.backend.List(ctx, "images/"+purposePath.objectSegment+"/", s.cleanupCursor, limit)
	if err != nil {
		return 0, fmt.Errorf("blob: list finalized GCS images: %w", err)
	}
	s.cleanupCursor = nextCursor
	deleted := 0
	for _, candidate := range objects {
		if candidate.Created.IsZero() || !candidate.Created.Before(createdBefore) {
			continue
		}
		reportID := finalizedReportID(candidate.Name)
		if _, err := finalizedFromAttrs(reportID, candidate); err != nil {
			continue
		}
		isReferenced, err := referenced(ctx, reportID, candidate.Name)
		if err != nil {
			return deleted, fmt.Errorf("blob: check finalized image reference: %w", err)
		}
		if isReferenced {
			continue
		}
		current, err := s.backend.Attrs(ctx, candidate.Name)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return deleted, fmt.Errorf("blob: recheck finalized GCS image generation: %w", err)
		}
		if current.Generation != candidate.Generation {
			continue
		}
		isReferenced, err = referenced(ctx, reportID, candidate.Name)
		if err != nil {
			return deleted, fmt.Errorf("blob: recheck finalized image reference: %w", err)
		}
		if isReferenced {
			continue
		}
		if err := s.backend.Delete(ctx, candidate.Name, candidate.Generation); err != nil {
			return deleted, fmt.Errorf("blob: delete orphaned finalized GCS image: %w", err)
		}
		deleted++
	}
	return deleted, nil
}

// Close releases the underlying GCS client.
func (s *GCSImageStore) Close() error { return s.backend.Close() }

func (s *GCSImageStore) validateFinalizeDeadline(metadata map[string]string) error {
	deadlineUnix, err := strconv.ParseInt(metadata[metadataFinalizeBefore], 10, 64)
	if err != nil || deadlineUnix <= 0 {
		return ErrUploadMismatch
	}
	if !s.now().UTC().Before(time.Unix(deadlineUnix, 0).UTC()) {
		return ErrUploadExpired
	}
	return nil
}

func validateUploadBinding(reportID, objectName string) (string, string, error) {
	if strings.TrimSpace(reportID) == "" || strings.Contains(reportID, "/") {
		return "", "", ErrUploadMismatch
	}
	_, purposePath, ok := imagePurposeForPath(reportID, objectName, "uploads")
	if !ok {
		return "", "", ErrUploadMismatch
	}
	prefix := "uploads/" + purposePath.objectSegment + "/" + reportID + "/image"
	extension := strings.TrimPrefix(objectName, prefix)
	var contentType string
	switch extension {
	case ".jpg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	default:
		return "", "", ErrUploadMismatch
	}
	return contentType, "images/" + purposePath.objectSegment + "/" + reportID + "/image" + extension, nil
}

func finalizedFromAttrs(reportID string, attrs gcsObjectAttrs) (*FinalizedImage, error) {
	purpose, purposePath, ok := imagePurposeForPath(reportID, attrs.Name, "images")
	if !ok {
		return nil, ErrNotFinalized
	}
	wantJPEG := "images/" + purposePath.objectSegment + "/" + reportID + "/image.jpg"
	wantPNG := "images/" + purposePath.objectSegment + "/" + reportID + "/image.png"
	validNameAndType := attrs.Name == wantJPEG && attrs.ContentType == "image/jpeg" ||
		attrs.Name == wantPNG && attrs.ContentType == "image/png"
	if reportID == "" || !validNameAndType || attrs.Size <= 0 || attrs.Size > MaxImageBytes ||
		attrs.Metadata[metadataReportID] != reportID ||
		attrs.Metadata[metadataPurpose] != string(purpose) ||
		len(attrs.Metadata[metadataUploadToken]) != sha256.Size*2 {
		return nil, ErrNotFinalized
	}
	deadlineUnix, deadlineErr := strconv.ParseInt(attrs.Metadata[metadataFinalizeBefore], 10, 64)
	if deadlineErr != nil || deadlineUnix <= 0 {
		return nil, ErrNotFinalized
	}
	width, widthErr := strconv.Atoi(attrs.Metadata[metadataWidth])
	height, heightErr := strconv.Atoi(attrs.Metadata[metadataHeight])
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return nil, ErrNotFinalized
	}
	return &FinalizedImage{
		ObjectName: attrs.Name, ContentType: attrs.ContentType, Size: attrs.Size,
		Width: width, Height: height, Generation: attrs.Generation,
	}, nil
}

func (b *googleStorageBackend) GenerateSignedPostPolicyV4(object string, opts *storage.PostPolicyV4Options) (*storage.PostPolicyV4, error) {
	copyOptions := *opts
	copyOptions.GoogleAccessID = b.googleAccessID
	copyOptions.PrivateKey = b.privateKey
	copyOptions.Hostname = b.hostname
	copyOptions.Insecure = b.insecure
	return b.bucket.GenerateSignedPostPolicyV4(object, &copyOptions)
}

func (b *googleStorageBackend) Attrs(ctx context.Context, object string) (gcsObjectAttrs, error) {
	attrs, err := b.bucket.Object(object).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return gcsObjectAttrs{}, ErrNotFound
	}
	if err != nil {
		return gcsObjectAttrs{}, err
	}
	return gcsObjectAttrs{
		Name: attrs.Name, ContentType: attrs.ContentType, Size: attrs.Size,
		Generation: attrs.Generation, Created: attrs.Created, Metadata: attrs.Metadata,
	}, nil
}

func (b *googleStorageBackend) List(ctx context.Context, prefix, startAfter string, limit int) ([]gcsObjectAttrs, string, error) {
	query := &storage.Query{Prefix: prefix, StartOffset: startAfter}
	objects := make([]gcsObjectAttrs, 0, limit)
	objectsIterator := b.bucket.Objects(ctx, query)
	for len(objects) < limit {
		attrs, err := objectsIterator.Next()
		if errors.Is(err, iterator.Done) {
			return objects, "", nil
		}
		if err != nil {
			return nil, "", err
		}
		if attrs.Name <= startAfter {
			continue
		}
		objects = append(objects, gcsObjectAttrs{
			Name: attrs.Name, ContentType: attrs.ContentType, Size: attrs.Size,
			Generation: attrs.Generation, Created: attrs.Created, Metadata: attrs.Metadata,
		})
	}
	return objects, objects[len(objects)-1].Name, nil
}

func (b *googleStorageBackend) Read(ctx context.Context, object string, generation, limit int64) ([]byte, error) {
	reader, err := b.bucket.Object(object).Generation(generation).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(io.LimitReader(reader, limit))
}

func (b *googleStorageBackend) Copy(ctx context.Context, destination, source string, sourceGeneration int64, attrs gcsObjectAttrs) (gcsObjectAttrs, error) {
	destinationObject := b.bucket.Object(destination).If(storage.Conditions{DoesNotExist: true})
	sourceObject := b.bucket.Object(source).Generation(sourceGeneration)
	copier := destinationObject.CopierFrom(sourceObject)
	copier.ContentType = attrs.ContentType
	copier.CacheControl = "private, no-store"
	copier.Metadata = attrs.Metadata
	copied, err := copier.Run(ctx)
	if err != nil {
		return gcsObjectAttrs{}, err
	}
	return gcsObjectAttrs{
		Name: copied.Name, ContentType: copied.ContentType, Size: copied.Size,
		Generation: copied.Generation, Metadata: copied.Metadata,
	}, nil
}

func (b *googleStorageBackend) Delete(ctx context.Context, object string, generation int64) error {
	err := b.bucket.Object(object).Generation(generation).Delete(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	return err
}

func (b *googleStorageBackend) SignedURL(object string, opts *storage.SignedURLOptions) (string, error) {
	copyOptions := *opts
	copyOptions.GoogleAccessID = b.googleAccessID
	copyOptions.PrivateKey = b.privateKey
	copyOptions.Hostname = b.hostname
	copyOptions.Insecure = b.insecure
	return b.bucket.SignedURL(object, &copyOptions)
}

func (b *googleStorageBackend) Close() error { return b.client.Close() }

func isPreconditionFailed(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusPreconditionFailed
}
