package petmatcher

import (
	"context"
	"sync"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/embedding"
)

type recordingEmbedder struct {
	*embedding.MockEmbedder
	mu        sync.Mutex
	mimeTypes []string
}

func newRecordingEmbedder() *recordingEmbedder {
	return &recordingEmbedder{
		MockEmbedder: embedding.NewMockEmbedder(),
	}
}

func (r *recordingEmbedder) EmbedMultimodal(ctx context.Context, img []byte, mimeType string, text string) ([]float32, error) {
	r.mu.Lock()
	r.mimeTypes = append(r.mimeTypes, mimeType)
	r.mu.Unlock()
	return r.MockEmbedder.EmbedMultimodal(ctx, img, mimeType, text)
}

func (r *recordingEmbedder) getMimeTypes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.mimeTypes...)
}

func TestComputeMultiPhotoEmbeddings_DetectsMIMETypes(t *testing.T) {
	ctx := context.Background()

	// 1. PNG magic bytes: \x89PNG\r\n\x1a\n
	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13, 'I', 'H', 'D', 'R'}

	// 2. JPEG magic bytes: \xff\xd8\xff
	jpegBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 0, 16, 'J', 'F', 'I', 'F', 0}

	// 3. Unknown non-image bytes
	unknownBytes := []byte("plain text content which is not an image at all")

	tests := []struct {
		name         string
		imgBytes     []byte
		wantMimeType string
	}{
		{
			name:         "detects PNG image",
			imgBytes:     pngBytes,
			wantMimeType: "image/png",
		},
		{
			name:         "detects JPEG image",
			imgBytes:     jpegBytes,
			wantMimeType: "image/jpeg",
		},
		{
			name:         "falls back to image/jpeg for non-image data",
			imgBytes:     unknownBytes,
			wantMimeType: "image/jpeg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := newRecordingEmbedder()
			images := []domain.PetImage{
				{Object: "test-obj", Tag: domain.PetImageTagPrimary},
			}

			computeMultiPhotoEmbeddings(
				ctx,
				rec,
				nil,
				images,
				"test-obj",
				tc.imgBytes,
				"test description",
			)

			recorded := rec.getMimeTypes()
			if len(recorded) != 1 {
				t.Fatalf("expected 1 embedder call, got %d", len(recorded))
			}
			if recorded[0] != tc.wantMimeType {
				t.Errorf("detected MIME type = %q, want %q", recorded[0], tc.wantMimeType)
			}
		})
	}
}
