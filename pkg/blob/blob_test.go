package blob_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

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
