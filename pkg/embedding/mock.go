package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand"
)

// MockEmbedder computes deterministic, unit-normalized 768-dimensional float32
// vectors derived from SHA-256 hashes of inputs. Zero network dependencies.
type MockEmbedder struct {
	dimension int
}

// NewMockEmbedder constructs a MockEmbedder with dimension 768.
func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{dimension: 768}
}

// Dimension returns the vector dimensionality.
func (m *MockEmbedder) Dimension() int {
	return m.dimension
}

// EmbedImage computes an embedding vector from raw image bytes and MIME type.
func (m *MockEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(mimeType))
	_, _ = h.Write(imageBytes)
	return m.hashToVector(h.Sum(nil)), nil
}

// EmbedText computes an embedding vector from descriptive text.
func (m *MockEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(text))
	return m.hashToVector(h[:]), nil
}

// EmbedMultimodal combines image bytes and contextual text into a single normalized vector.
func (m *MockEmbedder) EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(mimeType))
	_, _ = h.Write(imageBytes)
	_, _ = h.Write([]byte(text))
	return m.hashToVector(h.Sum(nil)), nil
}

func (m *MockEmbedder) hashToVector(digest []byte) []float32 {
	seed := int64(binary.BigEndian.Uint64(digest[:8]))
	/* #nosec G404 - weak random number generator is acceptable for deterministic mock embeddings */
	rng := rand.New(rand.NewSource(seed))
	vec := make([]float32, m.dimension)
	var sumSq float64
	for i := 0; i < m.dimension; i++ {
		val := rng.Float32()*2.0 - 1.0
		vec[i] = val
		sumSq += float64(val) * float64(val)
	}
	norm := float32(math.Sqrt(sumSq))
	if norm > 0 {
		for i := 0; i < m.dimension; i++ {
			vec[i] /= norm
		}
	}
	return vec
}
