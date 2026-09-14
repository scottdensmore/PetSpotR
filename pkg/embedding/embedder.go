package embedding

import (
	"context"
)

// Embedder generates fixed-dimension vector embeddings from visual and textual inputs.
type Embedder interface {
	// EmbedImage computes an embedding vector from raw image bytes and MIME type.
	EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error)

	// EmbedText computes an embedding vector from descriptive text.
	EmbedText(ctx context.Context, text string) ([]float32, error)

	// EmbedMultimodal combines image bytes and contextual text into a single normalized vector.
	EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error)

	// Dimension returns the vector dimensionality (768).
	Dimension() int
}
