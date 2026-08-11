package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"testing"
	"time"
)

func TestGCSDeployedSecureImageLifecycle(t *testing.T) {
	bucket := os.Getenv("PETSPOTR_GCS_INTEGRATION_BUCKET")
	if bucket == "" {
		t.Skip("PETSPOTR_GCS_INTEGRATION_BUCKET is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	images, err := NewGCSImageStore(ctx, GCSConfig{BucketName: bucket})
	if err != nil {
		t.Fatalf("NewGCSImageStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := images.Close(); err != nil {
			t.Errorf("close GCS integration client: %v", err)
		}
	})

	grant, err := images.BeginImageUpload(ctx, ImageUploadIntent{
		Purpose: ImagePurposeFoundPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginImageUpload() error = %v", err)
	}
	_, finalName, err := validateUploadBinding(grant.ReportID, grant.ObjectName)
	if err != nil {
		t.Fatalf("validate generated upload binding: %v", err)
	}
	registerGCSIntegrationCleanup(t, images, grant.ObjectName, finalName)
	imageBytes := encodedGCSImage(t, "png")
	if err := uploadSignedPost(ctx, grant, imageBytes); err != nil {
		t.Fatalf("signed POST upload: %v", err)
	}
	finalized, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil {
		t.Fatalf("FinalizeImage() error = %v", err)
	}
	readBytes, err := images.ReadFinalizedImage(ctx, finalized.ObjectName)
	if err != nil {
		t.Fatalf("ReadFinalizedImage() error = %v", err)
	}
	if !bytes.Equal(readBytes, imageBytes) {
		t.Fatal("finalized image bytes differ from uploaded bytes")
	}
	readURL, err := images.GenerateImageReadURL(ctx, finalized.ObjectName, time.Minute)
	if err != nil {
		t.Fatalf("GenerateImageReadURL() error = %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, readURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET signed read URL: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		t.Fatalf("GET signed read URL status = %d: %s", response.StatusCode, body)
	}
}

func registerGCSIntegrationCleanup(t *testing.T, images *GCSImageStore, objectNames ...string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, objectName := range objectNames {
			attrs, err := images.backend.Attrs(cleanupCtx, objectName)
			if errors.Is(err, ErrNotFound) {
				continue
			}
			if err != nil {
				t.Errorf("inspect GCS integration object %q during cleanup: %v", objectName, err)
				continue
			}
			if err := images.backend.Delete(cleanupCtx, objectName, attrs.Generation); err != nil {
				t.Errorf("delete GCS integration object %q generation %d: %v", objectName, attrs.Generation, err)
			}
		}
	})
}

func uploadSignedPost(ctx context.Context, grant *ImageUploadGrant, data []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range grant.FormFields {
		if err := writer.WriteField(name, value); err != nil {
			return err
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="pet.png"`)
	header.Set("Content-Type", grant.ContentType)
	file, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, grant.UploadURL, &body)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("status %d: %s", response.StatusCode, responseBody)
	}
	return nil
}
