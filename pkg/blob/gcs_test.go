package blob

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
)

func TestGCSImageStoreSecureLifecycle(t *testing.T) {
	ctx := context.Background()
	backend := newFakeGCSBackend()
	images := newGCSImageStore(backend)

	grant, err := images.BeginImageUpload(ctx, ImageUploadIntent{
		Purpose:     ImagePurposeFoundPet,
		FileName:    "../../caller-controlled.jpg",
		ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("BeginImageUpload() error = %v", err)
	}
	if !strings.HasPrefix(grant.ObjectName, "uploads/found-pets/"+grant.ReportID+"/") ||
		strings.Contains(grant.ObjectName, "caller-controlled") {
		t.Fatalf("generated object name trusted caller input: %q", grant.ObjectName)
	}
	if grant.FormFields["key"] != grant.ObjectName ||
		grant.FormFields[metadataReportIDField] != grant.ReportID ||
		grant.FormFields[metadataPurposeField] != string(ImagePurposeFoundPet) ||
		grant.FormFields[metadataUploadTokenField] != tokenDigest(grant.FinalizeToken) ||
		grant.FormFields[metadataFinalizeField] != strconv.FormatInt(grant.ExpiresAt.Unix(), 10) {
		t.Fatalf("upload form fields do not bind the generated report: %#v", grant.FormFields)
	}
	assertPostPolicyConstraints(
		t,
		grant.FormFields["policy"],
		grant.ObjectName,
		grant.ReportID,
		tokenDigest(grant.FinalizeToken),
		strconv.FormatInt(grant.ExpiresAt.Unix(), 10),
	)

	imageBytes := encodedGCSImage(t, "jpeg")
	backend.objects[grant.ObjectName] = fakeGCSObject{
		attrs: gcsObjectAttrs{
			Name:        grant.ObjectName,
			ContentType: "image/jpeg",
			Size:        int64(len(imageBytes)),
			Generation:  7,
			Metadata: map[string]string{
				metadataReportID:       grant.ReportID,
				metadataPurpose:        string(ImagePurposeFoundPet),
				metadataUploadToken:    tokenDigest(grant.FinalizeToken),
				metadataFinalizeBefore: strconv.FormatInt(grant.ExpiresAt.Unix(), 10),
			},
		},
		data: imageBytes,
	}

	finalized, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil {
		t.Fatalf("FinalizeImage() error = %v", err)
	}
	wantFinal := "images/found-pets/" + grant.ReportID + "/image.jpg"
	if finalized.ObjectName != wantFinal || finalized.Width != 2 || finalized.Height != 3 {
		t.Fatalf("FinalizeImage() = %#v, want %q and decoded dimensions", finalized, wantFinal)
	}
	if _, ok := backend.objects[grant.ObjectName]; ok {
		t.Fatal("temporary object was not deleted")
	}
	if backend.copySourceGeneration != 7 {
		t.Fatalf("copy source generation = %d, want 7", backend.copySourceGeneration)
	}

	readURL, err := images.GenerateImageReadURL(ctx, finalized.ObjectName, 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateImageReadURL() error = %v", err)
	}
	if !strings.Contains(readURL, "X-Goog-Signature=") || backend.readMethod != http.MethodGet {
		t.Fatalf("private read URL = %q, method = %q", readURL, backend.readMethod)
	}
	if backend.readExpires.After(time.Now().UTC().Add(6 * time.Minute)) {
		t.Fatalf("read expiry = %v, want short-lived URL", backend.readExpires)
	}

	again, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil || again.ObjectName != finalized.ObjectName {
		t.Fatalf("idempotent FinalizeImage() = %#v, %v", again, err)
	}
}

func TestGCSImageStoreScopesLostPetLifecycle(t *testing.T) {
	ctx := context.Background()
	backend := newFakeGCSBackend()
	images := newGCSImageStore(backend)
	grant, err := images.BeginImageUpload(ctx, ImageUploadIntent{
		Purpose: ImagePurposeLostPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginImageUpload() error = %v", err)
	}
	if !strings.HasPrefix(grant.ReportID, "lost-") ||
		!strings.HasPrefix(grant.ObjectName, "uploads/lost-pets/"+grant.ReportID+"/") ||
		grant.FormFields[metadataPurposeField] != string(ImagePurposeLostPet) {
		t.Fatalf("lost-pet grant = %#v", grant)
	}
	imageBytes := encodedGCSImage(t, "png")
	backend.objects[grant.ObjectName] = fakeGCSObject{
		attrs: gcsObjectAttrs{
			Name: grant.ObjectName, ContentType: "image/png", Size: int64(len(imageBytes)), Generation: 17,
			Metadata: map[string]string{
				metadataReportID: grant.ReportID, metadataPurpose: string(ImagePurposeLostPet),
				metadataUploadToken:    tokenDigest(grant.FinalizeToken),
				metadataFinalizeBefore: strconv.FormatInt(grant.ExpiresAt.Unix(), 10),
			},
		},
		data: imageBytes,
	}
	finalized, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil {
		t.Fatalf("FinalizeImage() error = %v", err)
	}
	wantFinal := "images/lost-pets/" + grant.ReportID + "/image.png"
	if finalized.ObjectName != wantFinal {
		t.Fatalf("finalized object = %q, want %q", finalized.ObjectName, wantFinal)
	}
	if got, err := images.ReadFinalizedImage(ctx, wantFinal); err != nil || !bytes.Equal(got, imageBytes) {
		t.Fatalf("ReadFinalizedImage() = %d bytes, %v", len(got), err)
	}
}

func TestGCSImageStoreRejectsUntrustedObjectMetadata(t *testing.T) {
	ctx := context.Background()
	backend := newFakeGCSBackend()
	images := newGCSImageStore(backend)
	grant, err := images.BeginImageUpload(ctx, ImageUploadIntent{
		Purpose: ImagePurposeFoundPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}
	data := encodedGCSImage(t, "png")
	backend.objects[grant.ObjectName] = fakeGCSObject{
		attrs: gcsObjectAttrs{
			Name: grant.ObjectName, ContentType: "image/png", Size: int64(len(data)), Generation: 4,
			Metadata: map[string]string{
				metadataReportID: "different-report", metadataPurpose: string(ImagePurposeFoundPet),
				metadataUploadToken:    tokenDigest(grant.FinalizeToken),
				metadataFinalizeBefore: strconv.FormatInt(grant.ExpiresAt.Unix(), 10),
			},
		},
		data: data,
	}

	if _, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken); !errors.Is(err, ErrUploadMismatch) {
		t.Fatalf("FinalizeImage() error = %v, want ErrUploadMismatch", err)
	}
	if backend.copyCalls != 0 || backend.deleteCalls != 0 {
		t.Fatalf("rejected object was mutated: copies=%d deletes=%d", backend.copyCalls, backend.deleteCalls)
	}

	finalName := "images/found-pets/" + grant.ReportID + "/image.png"
	backend.objects[finalName] = fakeGCSObject{attrs: gcsObjectAttrs{
		Name: finalName, ContentType: "text/html", Size: int64(len(data)), Generation: 9,
		Metadata: map[string]string{
			metadataReportID:       grant.ReportID,
			metadataPurpose:        string(ImagePurposeFoundPet),
			metadataWidth:          "2",
			metadataHeight:         "3",
			metadataUploadToken:    tokenDigest(grant.FinalizeToken),
			metadataFinalizeBefore: strconv.FormatInt(grant.ExpiresAt.Unix(), 10),
		},
	}, data: data}
	if _, err := images.GenerateImageReadURL(ctx, finalName, time.Minute); !errors.Is(err, ErrNotFinalized) {
		t.Fatalf("GenerateImageReadURL() error = %v, want ErrNotFinalized for mismatched content type", err)
	}
}

func TestGCSImageStoreRejectsExpiredFinalizeCapability(t *testing.T) {
	ctx := context.Background()
	backend := newFakeGCSBackend()
	images := newGCSImageStore(backend)
	grant, err := images.BeginImageUpload(ctx, ImageUploadIntent{
		Purpose: ImagePurposeFoundPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	data := encodedGCSImage(t, "jpeg")
	backend.objects[grant.ObjectName] = fakeGCSObject{
		attrs: gcsObjectAttrs{
			Name: grant.ObjectName, ContentType: "image/jpeg", Size: int64(len(data)), Generation: 4,
			Metadata: map[string]string{
				metadataReportID: grant.ReportID, metadataPurpose: string(ImagePurposeFoundPet),
				metadataUploadToken:    tokenDigest(grant.FinalizeToken),
				metadataFinalizeBefore: strconv.FormatInt(grant.ExpiresAt.Unix(), 10),
			},
		},
		data: data,
	}
	images.now = func() time.Time { return grant.ExpiresAt.Add(time.Nanosecond) }

	if _, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken); !errors.Is(err, ErrUploadExpired) {
		t.Fatalf("FinalizeImage() error = %v, want ErrUploadExpired", err)
	}
	if backend.copyCalls != 0 || backend.deleteCalls != 0 {
		t.Fatalf("expired capability mutated objects: copies=%d deletes=%d", backend.copyCalls, backend.deleteCalls)
	}

	finalName := "images/found-pets/" + grant.ReportID + "/image.jpg"
	backend.objects[finalName] = fakeGCSObject{attrs: gcsObjectAttrs{
		Name: finalName, ContentType: "image/jpeg", Size: int64(len(data)), Generation: 8,
		Metadata: map[string]string{
			metadataReportID: grant.ReportID, metadataPurpose: string(ImagePurposeFoundPet),
			metadataWidth: "2", metadataHeight: "3",
			metadataUploadToken:    tokenDigest(grant.FinalizeToken),
			metadataFinalizeBefore: strconv.FormatInt(grant.ExpiresAt.Unix(), 10),
		},
	}, data: data}
	if _, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken); !errors.Is(err, ErrUploadExpired) {
		t.Fatalf("replayed finalized capability error = %v, want ErrUploadExpired", err)
	}
}

func TestGCSImageStoreCleansFinalizedOrphansSafely(t *testing.T) {
	ctx := context.Background()
	backend := newFakeGCSBackend()
	images := newGCSImageStore(backend)
	old := time.Now().UTC().Add(-DefaultOrphanGracePeriod - time.Hour)
	addFinalObject := func(reportID string, generation int64) string {
		name := "images/found-pets/" + reportID + "/image.jpg"
		backend.objects[name] = fakeGCSObject{attrs: gcsObjectAttrs{
			Name: name, ContentType: "image/jpeg", Size: 100, Generation: generation, Created: old,
			Metadata: map[string]string{
				metadataReportID: reportID, metadataPurpose: string(ImagePurposeFoundPet),
				metadataWidth: "2", metadataHeight: "3", metadataUploadToken: strings.Repeat("a", 64),
				metadataFinalizeBefore: strconv.FormatInt(old.Unix(), 10),
			},
		}}
		return name
	}
	orphan := addFinalObject("found-orphan", 10)
	referenced := addFinalObject("found-referenced", 11)
	concurrent := addFinalObject("found-concurrent", 12)
	replaced := addFinalObject("found-replaced", 13)
	checks := make(map[string]int)
	backend.beforeAttrs = func(object string) {
		if object == replaced {
			value := backend.objects[object]
			value.attrs.Generation = 14
			backend.objects[object] = value
			backend.beforeAttrs = nil
		}
	}

	deleted, err := images.CleanupOrphanedFinalizedImages(
		ctx,
		time.Now().UTC().Add(-DefaultOrphanGracePeriod),
		MaxOrphanCleanupBatch,
		func(_ context.Context, _ string, objectName string) (bool, error) {
			checks[objectName]++
			switch objectName {
			case referenced:
				return true, nil
			case concurrent:
				return checks[objectName] == 2, nil
			default:
				return false, nil
			}
		},
	)
	if err != nil {
		t.Fatalf("CleanupOrphanedFinalizedImages() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, ok := backend.objects[orphan]; ok {
		t.Fatal("unreferenced finalized object was preserved")
	}
	for _, preserved := range []string{referenced, concurrent, replaced} {
		if _, ok := backend.objects[preserved]; !ok {
			t.Fatalf("safe object %q was deleted", preserved)
		}
	}
}

func TestGCSImageStoreScopesOrphanCleanupByPurpose(t *testing.T) {
	ctx := context.Background()
	backend := newFakeGCSBackend()
	images := newGCSImageStore(backend)
	old := time.Now().UTC().Add(-DefaultOrphanGracePeriod - time.Hour)
	addFinalObject := func(purpose ImagePurpose, reportID, segment string, generation int64) string {
		name := "images/" + segment + "/" + reportID + "/image.jpg"
		backend.objects[name] = fakeGCSObject{attrs: gcsObjectAttrs{
			Name: name, ContentType: "image/jpeg", Size: 100, Generation: generation, Created: old,
			Metadata: map[string]string{
				metadataReportID: reportID, metadataPurpose: string(purpose),
				metadataWidth: "2", metadataHeight: "3", metadataUploadToken: strings.Repeat("c", 64),
				metadataFinalizeBefore: strconv.FormatInt(old.Unix(), 10),
			},
		}}
		return name
	}
	foundObject := addFinalObject(ImagePurposeFoundPet, "found-scope", "found-pets", 21)
	lostObject := addFinalObject(ImagePurposeLostPet, "lost-scope", "lost-pets", 22)
	deleted, err := images.CleanupOrphanedFinalizedImagesForPurpose(
		ctx, ImagePurposeLostPet, time.Now().UTC(), MaxOrphanCleanupBatch,
		func(context.Context, string, string) (bool, error) { return false, nil },
	)
	if err != nil || deleted != 1 {
		t.Fatalf("lost-pet cleanup = (%d, %v), want (1, nil)", deleted, err)
	}
	if _, ok := backend.objects[lostObject]; ok {
		t.Fatal("lost-pet orphan was preserved")
	}
	if _, ok := backend.objects[foundObject]; !ok {
		t.Fatal("lost-pet cleanup deleted a found-pet object")
	}
}

func TestGCSImageStoreBoundsOrphanCleanupBatches(t *testing.T) {
	ctx := context.Background()
	backend := newFakeGCSBackend()
	images := newGCSImageStore(backend)
	old := time.Now().UTC().Add(-DefaultOrphanGracePeriod - time.Hour)
	for i := 0; i < 3; i++ {
		reportID := fmt.Sprintf("found-bounded-%d", i)
		name := "images/found-pets/" + reportID + "/image.png"
		backend.objects[name] = fakeGCSObject{attrs: gcsObjectAttrs{
			Name: name, ContentType: "image/png", Size: 100, Generation: int64(i + 1), Created: old,
			Metadata: map[string]string{
				metadataReportID: reportID, metadataPurpose: string(ImagePurposeFoundPet),
				metadataWidth: "2", metadataHeight: "3", metadataUploadToken: strings.Repeat("b", 64),
				metadataFinalizeBefore: strconv.FormatInt(old.Unix(), 10),
			},
		}}
	}
	notReferenced := func(context.Context, string, string) (bool, error) { return false, nil }
	deleted, err := images.CleanupOrphanedFinalizedImages(ctx, time.Now().UTC(), 2, notReferenced)
	if err != nil || deleted != 2 || len(backend.objects) != 1 {
		t.Fatalf("first cleanup = (%d, %v), remaining=%d; want (2, nil), 1", deleted, err, len(backend.objects))
	}
	deleted, err = images.CleanupOrphanedFinalizedImages(ctx, time.Now().UTC(), 2, notReferenced)
	if err != nil || deleted != 1 || len(backend.objects) != 0 {
		t.Fatalf("second cleanup = (%d, %v), remaining=%d; want (1, nil), 0", deleted, err, len(backend.objects))
	}
}

func assertPostPolicyConstraints(t *testing.T, encodedPolicy, objectName, reportID, tokenHash, deadline string) {
	t.Helper()
	policyJSON, err := base64.StdEncoding.DecodeString(encodedPolicy)
	if err != nil {
		t.Fatalf("decode post policy: %v", err)
	}
	var policy struct {
		Conditions []any `json:"conditions"`
	}
	if err := json.Unmarshal(policyJSON, &policy); err != nil {
		t.Fatalf("unmarshal post policy: %v", err)
	}
	serialized := string(policyJSON)
	for _, required := range []string{
		`["content-length-range",1,10485760]`,
		`"content-type":"image/jpeg"`,
		`"key":"` + objectName + `"`,
		`"x-goog-meta-petspotr-report-id":"` + reportID + `"`,
		`"x-goog-meta-petspotr-purpose":"found-pet"`,
		`"x-goog-meta-petspotr-upload-token-sha256":"` + tokenHash + `"`,
		`"x-goog-meta-petspotr-finalize-before":"` + deadline + `"`,
	} {
		if !strings.Contains(serialized, required) {
			t.Fatalf("post policy %s does not contain %s", serialized, required)
		}
	}
}

func encodedGCSImage(t *testing.T, format string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var output bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&output, img, nil)
	case "png":
		err = png.Encode(&output, img)
	default:
		t.Fatalf("unsupported test format %q", format)
	}
	if err != nil {
		t.Fatalf("encode %s: %v", format, err)
	}
	return output.Bytes()
}

type fakeGCSObject struct {
	attrs gcsObjectAttrs
	data  []byte
}

type fakeGCSBackend struct {
	objects              map[string]fakeGCSObject
	copyCalls            int
	deleteCalls          int
	copySourceGeneration int64
	readMethod           string
	readExpires          time.Time
	beforeAttrs          func(string)
}

func newFakeGCSBackend() *fakeGCSBackend {
	return &fakeGCSBackend{objects: make(map[string]fakeGCSObject)}
}

func (f *fakeGCSBackend) GenerateSignedPostPolicyV4(object string, opts *storage.PostPolicyV4Options) (*storage.PostPolicyV4, error) {
	copyOptions := *opts
	copyOptions.GoogleAccessID = "test-signer@example.iam.gserviceaccount.com"
	copyOptions.SignRawBytes = func(data []byte) ([]byte, error) { return []byte("test-signature"), nil }
	return storage.GenerateSignedPostPolicyV4("private-pet-images", object, &copyOptions)
}

func (f *fakeGCSBackend) Attrs(_ context.Context, object string) (gcsObjectAttrs, error) {
	if f.beforeAttrs != nil {
		f.beforeAttrs(object)
	}
	value, ok := f.objects[object]
	if !ok {
		return gcsObjectAttrs{}, ErrNotFound
	}
	return value.attrs, nil
}

func (f *fakeGCSBackend) List(_ context.Context, prefix, startAfter string, limit int) ([]gcsObjectAttrs, string, error) {
	names := make([]string, 0, len(f.objects))
	for name := range f.objects {
		if strings.HasPrefix(name, prefix) && name > startAfter {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > limit {
		names = names[:limit]
	}
	attrs := make([]gcsObjectAttrs, 0, len(names))
	for _, name := range names {
		attrs = append(attrs, f.objects[name].attrs)
	}
	next := ""
	if len(names) == limit {
		next = names[len(names)-1]
	}
	return attrs, next, nil
}

func (f *fakeGCSBackend) Read(_ context.Context, object string, generation, limit int64) ([]byte, error) {
	value, ok := f.objects[object]
	if !ok {
		return nil, ErrNotFound
	}
	if value.attrs.Generation != generation {
		return nil, fmt.Errorf("generation mismatch")
	}
	if int64(len(value.data)) > limit {
		return value.data[:limit], nil
	}
	return value.data, nil
}

func (f *fakeGCSBackend) Copy(_ context.Context, destination, source string, sourceGeneration int64, attrs gcsObjectAttrs) (gcsObjectAttrs, error) {
	f.copyCalls++
	f.copySourceGeneration = sourceGeneration
	sourceObject, ok := f.objects[source]
	if !ok || sourceObject.attrs.Generation != sourceGeneration {
		return gcsObjectAttrs{}, ErrNotFound
	}
	attrs.Name = destination
	attrs.Generation = 8
	f.objects[destination] = fakeGCSObject{attrs: attrs, data: sourceObject.data}
	return attrs, nil
}

func (f *fakeGCSBackend) Delete(_ context.Context, object string, generation int64) error {
	f.deleteCalls++
	value, ok := f.objects[object]
	if !ok {
		return nil
	}
	if value.attrs.Generation != generation {
		return fmt.Errorf("generation mismatch")
	}
	delete(f.objects, object)
	return nil
}

func (f *fakeGCSBackend) SignedURL(object string, opts *storage.SignedURLOptions) (string, error) {
	f.readMethod = opts.Method
	f.readExpires = opts.Expires
	if _, ok := f.objects[object]; !ok {
		return "", ErrNotFound
	}
	return "https://storage.googleapis.com/private-pet-images/" + object + "?X-Goog-Signature=test", nil
}

func (f *fakeGCSBackend) Close() error { return nil }
