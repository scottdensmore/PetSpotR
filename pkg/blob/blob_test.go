package blob_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
)

func TestMemoryBlobStore(t *testing.T) {
	ctx := context.Background()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io/images")

	t.Run("upload image and get URL", func(t *testing.T) {
		url, err := bs.UploadImage(ctx, "buddy.jpg", []byte("image-data"))
		if err != nil {
			t.Fatalf("failed to upload image: %v", err)
		}

		expected := "https://storage.petspotr.io/images/buddy.jpg"
		if url != expected {
			t.Errorf("got URL %s, want %s", url, expected)
		}

		data, err := bs.GetImage(ctx, "buddy.jpg")
		if err != nil {
			t.Fatalf("failed to get image: %v", err)
		}

		if string(data) != "image-data" {
			t.Errorf("got data %s, want image-data", string(data))
		}
	})

	t.Run("get non-existent image returns ErrNotFound", func(t *testing.T) {
		_, err := bs.GetImage(ctx, "missing.jpg")
		if !errors.Is(err, blob.ErrNotFound) {
			t.Fatalf("expected blob.ErrNotFound, got %v", err)
		}
	})

	t.Run("byte slice isolation on upload", func(t *testing.T) {
		input := []byte("original")
		_, err := bs.UploadImage(ctx, "isolated.jpg", input)
		if err != nil {
			t.Fatalf("upload failed: %v", err)
		}

		input[0] = 'X' // Mutate original slice

		retrieved, err := bs.GetImage(ctx, "isolated.jpg")
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}

		if string(retrieved) != "original" {
			t.Errorf("stored slice was mutated: got %s, want original", string(retrieved))
		}
	})

	t.Run("concurrent upload and read", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				name := fmt.Sprintf("img-%d.jpg", id)
				_, _ = bs.UploadImage(ctx, name, []byte(name))
				_, _ = bs.GetImage(ctx, name)
			}(i)
		}
		wg.Wait()
	})

	t.Run("context cancellation", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := bs.UploadImage(canceledCtx, "canceled.jpg", []byte("data"))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("UploadImage: got %v, want %v", err, context.Canceled)
		}

		_, err = bs.GetImage(canceledCtx, "canceled.jpg")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("GetImage: got %v, want %v", err, context.Canceled)
		}
	})

	t.Run("generate presigned upload URL", func(t *testing.T) {
		res, err := bs.GeneratePresignedUploadURL(ctx, "photo.jpg", "image/jpeg", 15*20)
		if err != nil {
			t.Fatalf("failed to generate presigned upload URL: %v", err)
		}

		if res == nil {
			t.Fatal("expected non-nil PresignedURLResponse")
		}

		if res.UploadURL == "" || res.PublicURL == "" {
			t.Errorf("expected non-empty upload and public URLs, got %+v", res)
		}

		if res.FileName != "photo.jpg" {
			t.Errorf("expected fileName photo.jpg, got %s", res.FileName)
		}
	})
}

func TestMemoryBlobStoreSecureImageLifecycle(t *testing.T) {
	ctx := context.Background()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io")

	grant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose:     blob.ImagePurposeFoundPet,
		FileName:    "../../someone-elses/report.jpg",
		ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("BeginImageUpload() error = %v", err)
	}
	if grant.ReportID == "" {
		t.Fatal("BeginImageUpload().ReportID is empty")
	}
	if grant.FinalizeToken == "" {
		t.Fatal("BeginImageUpload().FinalizeToken is empty")
	}
	wantPrefix := "uploads/found-pets/" + grant.ReportID + "/"
	if !strings.HasPrefix(grant.ObjectName, wantPrefix) {
		t.Fatalf("ObjectName = %q, want prefix %q", grant.ObjectName, wantPrefix)
	}
	if strings.Contains(grant.ObjectName, "someone-elses") || strings.Contains(grant.ObjectName, "..") {
		t.Fatalf("ObjectName trusted caller path: %q", grant.ObjectName)
	}
	if grant.MaxBytes != blob.MaxImageBytes {
		t.Fatalf("MaxBytes = %d, want %d", grant.MaxBytes, blob.MaxImageBytes)
	}
	if grant.FormFields["content-type"] != "image/jpeg" {
		t.Fatalf("content-type form field = %q, want image/jpeg", grant.FormFields["content-type"])
	}
	if grant.ExpiresAt.Before(time.Now().UTC()) || grant.ExpiresAt.After(time.Now().UTC().Add(16*time.Minute)) {
		t.Fatalf("ExpiresAt = %v, want a short-lived grant", grant.ExpiresAt)
	}

	imageBytes := encodedImage(t, "jpeg")
	if _, err := bs.UploadImage(ctx, grant.ObjectName, imageBytes); err != nil {
		t.Fatalf("UploadImage() error = %v", err)
	}
	finalized, err := bs.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil {
		t.Fatalf("FinalizeImage() error = %v", err)
	}
	wantFinalName := "images/found-pets/" + grant.ReportID + "/image.jpg"
	if finalized.ObjectName != wantFinalName {
		t.Fatalf("FinalizeImage().ObjectName = %q, want %q", finalized.ObjectName, wantFinalName)
	}
	if finalized.ContentType != "image/jpeg" || finalized.Size != int64(len(imageBytes)) {
		t.Fatalf("FinalizeImage() = %#v", finalized)
	}
	if finalized.Width != 2 || finalized.Height != 3 {
		t.Fatalf("FinalizeImage() dimensions = %dx%d, want 2x3", finalized.Width, finalized.Height)
	}
	if _, err := bs.GetImage(ctx, grant.ObjectName); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("temporary object still exists: %v", err)
	}
	if _, err := bs.GetImage(ctx, finalized.ObjectName); err != nil {
		t.Fatalf("final object is unavailable: %v", err)
	}

	readURL, err := bs.GenerateImageReadURL(ctx, finalized.ObjectName, 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateImageReadURL() error = %v", err)
	}
	parsed, err := url.Parse(readURL)
	if err != nil {
		t.Fatalf("parse read URL: %v", err)
	}
	if parsed.Query().Get("token") == "" || strings.Contains(readURL, "mock-sig") {
		t.Fatalf("read URL does not contain an opaque token: %q", readURL)
	}

	again, err := bs.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil || again.ObjectName != finalized.ObjectName {
		t.Fatalf("idempotent FinalizeImage() = %#v, %v", again, err)
	}
}

func TestMemoryBlobStoreScopesLostPetImageLifecycle(t *testing.T) {
	ctx := context.Background()
	images := blob.NewMemoryBlobStore("https://storage.petspotr.io")
	grant, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeLostPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("BeginImageUpload() error = %v", err)
	}
	wantUploadPrefix := "uploads/lost-pets/" + grant.ReportID + "/"
	if !strings.HasPrefix(grant.ReportID, "lost-") || !strings.HasPrefix(grant.ObjectName, wantUploadPrefix) {
		t.Fatalf("lost-pet grant = %#v, want prefix %q", grant, wantUploadPrefix)
	}
	if got := grant.FormFields["x-goog-meta-petspotr-purpose"]; got != string(blob.ImagePurposeLostPet) {
		t.Fatalf("purpose metadata = %q, want %q", got, blob.ImagePurposeLostPet)
	}
	data := encodedImage(t, "png")
	if _, err := images.UploadImage(ctx, grant.ObjectName, data); err != nil {
		t.Fatal(err)
	}
	finalized, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil {
		t.Fatalf("FinalizeImage() error = %v", err)
	}
	wantFinal := "images/lost-pets/" + grant.ReportID + "/image.png"
	if finalized.ObjectName != wantFinal {
		t.Fatalf("finalized object = %q, want %q", finalized.ObjectName, wantFinal)
	}
	if got, err := images.ReadFinalizedImage(ctx, finalized.ObjectName); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("ReadFinalizedImage() = %d bytes, %v", len(got), err)
	}
}

func TestMemoryBlobStoreRejectsCrossPurposeFinalization(t *testing.T) {
	ctx := context.Background()
	images := blob.NewMemoryBlobStore("https://storage.petspotr.io")
	grant, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := images.UploadImage(ctx, grant.ObjectName, encodedImage(t, "jpeg")); err != nil {
		t.Fatal(err)
	}
	if _, err := images.FinalizeImageForPurpose(
		ctx, blob.ImagePurposeLostPet, grant.ReportID, grant.ObjectName, grant.FinalizeToken,
	); !errors.Is(err, blob.ErrUploadMismatch) {
		t.Fatalf("FinalizeImageForPurpose() error = %v, want ErrUploadMismatch", err)
	}
	if _, err := images.GetImage(ctx, grant.ObjectName); err != nil {
		t.Fatalf("cross-purpose rejection mutated temporary object: %v", err)
	}
}

func TestMemoryBlobStoreRejectsUnsafeImageUploads(t *testing.T) {
	ctx := context.Background()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io")

	for _, contentType := range []string{"", "image/gif", "image/svg+xml", "application/octet-stream"} {
		t.Run("content type "+contentType, func(t *testing.T) {
			_, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
				Purpose:     blob.ImagePurposeFoundPet,
				FileName:    "pet",
				ContentType: contentType,
			})
			if !errors.Is(err, blob.ErrInvalidImage) {
				t.Fatalf("BeginImageUpload() error = %v, want ErrInvalidImage", err)
			}
		})
	}

	t.Run("declared type must match decoded bytes", func(t *testing.T) {
		grant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
			Purpose:     blob.ImagePurposeFoundPet,
			FileName:    "pet.jpg",
			ContentType: "image/jpeg",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bs.UploadImage(ctx, grant.ObjectName, encodedImage(t, "png")); err != nil {
			t.Fatal(err)
		}
		if _, err := bs.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken); !errors.Is(err, blob.ErrInvalidImage) {
			t.Fatalf("FinalizeImage() error = %v, want ErrInvalidImage", err)
		}
	})

	t.Run("invalid bytes are rejected", func(t *testing.T) {
		grant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
			Purpose:     blob.ImagePurposeFoundPet,
			FileName:    "pet.png",
			ContentType: "image/png",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bs.UploadImage(ctx, grant.ObjectName, []byte("not an image")); err != nil {
			t.Fatal(err)
		}
		if _, err := bs.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken); !errors.Is(err, blob.ErrInvalidImage) {
			t.Fatalf("FinalizeImage() error = %v, want ErrInvalidImage", err)
		}
	})

	t.Run("report and object binding is enforced", func(t *testing.T) {
		grant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
			Purpose:     blob.ImagePurposeFoundPet,
			FileName:    "pet.jpg",
			ContentType: "image/jpeg",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bs.UploadImage(ctx, grant.ObjectName, encodedImage(t, "jpeg")); err != nil {
			t.Fatal(err)
		}
		if _, err := bs.FinalizeImage(ctx, "different-report", grant.ObjectName, grant.FinalizeToken); !errors.Is(err, blob.ErrUploadMismatch) {
			t.Fatalf("FinalizeImage() error = %v, want ErrUploadMismatch", err)
		}
	})

	t.Run("finalize capability is required", func(t *testing.T) {
		grant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
			Purpose: blob.ImagePurposeFoundPet, ContentType: "image/jpeg",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bs.UploadImage(ctx, grant.ObjectName, encodedImage(t, "jpeg")); err != nil {
			t.Fatal(err)
		}
		if _, err := bs.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, "wrong-token"); !errors.Is(err, blob.ErrUploadMismatch) {
			t.Fatalf("FinalizeImage() error = %v, want ErrUploadMismatch", err)
		}
	})

	t.Run("unfinalized objects have no read capability", func(t *testing.T) {
		grant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
			Purpose:     blob.ImagePurposeFoundPet,
			FileName:    "pet.jpg",
			ContentType: "image/jpeg",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bs.GenerateImageReadURL(ctx, grant.ObjectName, time.Minute); !errors.Is(err, blob.ErrNotFinalized) {
			t.Fatalf("GenerateImageReadURL() error = %v, want ErrNotFinalized", err)
		}
	})
}

func encodedImage(t *testing.T, format string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var output strings.Builder
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
	return []byte(output.String())
}
