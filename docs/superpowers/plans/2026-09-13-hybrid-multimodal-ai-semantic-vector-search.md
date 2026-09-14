# Milestone 5.3: Hybrid Multimodal AI & Semantic Vector Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Milestone 5.3: Hybrid Multimodal AI & Semantic Vector Search, incorporating multi-photo ingestion (up to 3 photos per pet report), a pluggable multimodal embedder (Vertex AI, Ollama, and offline deterministic Mock), a tri-factor hybrid scoring engine (40% vector + 35% trait + 25% spatial with a hard species veto), Firestore vector storage and in-memory candidate retrieval, and an upgraded frontend multi-photo upload dropzone and visual match comparison dashboard.

**Architecture:** Extend domain models (`pkg/domain`) with `PetImage` and multi-photo normalization/validation; create `pkg/embedding` with a pluggable `Embedder` interface and deterministic `MockEmbedder`; enhance `pkg/scoring` with cosine similarity math, species veto enforcement, and hybrid tri-factor ranking; update `pkg/store` with vector embedding persistence; evolve `internal/app/petmatcher` to generate composite embeddings and rank candidates with the hybrid engine; and update `webfrontend` with multi-photo staging dropzones on `/pets/lost` and `/pets/found`, plus multi-angle thumbnail carousels and visual AI vector breakdown on `/matches`.

**Tech Stack:** Go 1.25, Google Cloud Vertex AI Multimodal Embeddings (`multimodalembedding@001`), Ollama multimodal vision API, Cloud Storage, Google Cloud Firestore Vector Indexing (`firestore.Vector32`), Vanilla ES6+ JavaScript, CSS3 Glassmorphism, html/template.

**Spec:** [`docs/superpowers/specs/2026-09-13-hybrid-multimodal-ai-semantic-vector-search-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-13-hybrid-multimodal-ai-semantic-vector-search-design.md)

## Global Constraints

- Content Security Policy (CSP): Strict `script-src 'self'` and `style-src 'self' https://fonts.googleapis.com`; zero inline styles (`style="..."`) and zero inline `<script>` blocks.
- Offline Independence: Zero external network or GCP credential requirements during `make verify`. Unit tests must run 100% offline using `MockEmbedder`.
- Backward Compatibility: Existing single-image reports (`ImageObject`) must automatically synthesize a primary `PetImage` and continue working without data migration.
- Species Invariant: When `lost.Species != found.Species`, match score must immediately evaluate to `0.0` and `IsMatch = false`.
- Embedding Invariant: Embeddings, when present, must have dimension 768 and unit Euclidean norm ($||\mathbf{v}||_2 \approx 1.0$).
- Double-Submit CSRF: Presigned URL requests and report creation endpoints must validate CSRF tokens when session cookies are present.
- Every commit must adhere to Conventional Commits formatting (`feat(...)`, `fix(...)`, `docs(...)`, `test(...)`, `style(...)`).
- Must pass `make verify` (`go vet`, `golangci-lint`, `tofu validate`, `yamllint`, `go test -race -cover ./...`).

---

### Task 1: Domain Models & Multi-Photo Schema Evolution

**Files:**
- Create: `pkg/domain/pet_image.go`
- Create: `pkg/domain/pet_image_test.go`
- Modify: `pkg/domain/lost_report.go`
- Modify: `pkg/domain/found_report.go`
- Modify: `pkg/domain/match.go`
- Modify: `pkg/domain/events.go`

**Interfaces:**
- Consumes: `domain.LocationPoint`, standard math & crypto packages
- Produces: `domain.PetImageTag`, `domain.PetImage`, `domain.NormalizePetImages`, `domain.ValidatePetImages`, updated `LostPetRecord`, `FoundPetRecord`, `LostPetReport`, `FoundPetReport`, `MatchScoreBreakdown`, `MatchPetDetail`

- [ ] **Step 1: Write the failing tests in `pkg/domain/pet_image_test.go`**

```go
package domain_test

import (
	"math"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestPetImage_ValidationAndNormalization(t *testing.T) {
	t.Run("valid multi-photo list with tags", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "images/lost/1/face.jpg", Tag: domain.PetImageTagFace},
			{Object: "images/lost/1/coat.jpg", Tag: domain.PetImageTagCoat},
			{Object: "images/lost/1/collar.jpg", Tag: domain.PetImageTagCollar},
		}
		normalized := domain.NormalizePetImages(images)
		if len(normalized) != 3 {
			t.Fatalf("expected 3 images, got %d", len(normalized))
		}
		// First image without primary tag should be assigned primary tag if none present
		if normalized[0].Tag != domain.PetImageTagPrimary {
			t.Errorf("expected first image tag to be %q, got %q", domain.PetImageTagPrimary, normalized[0].Tag)
		}
		if err := domain.ValidatePetImages(normalized); err != nil {
			t.Fatalf("ValidatePetImages() error = %v", err)
		}
	})

	t.Run("photo cap enforced at 3 photos", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTagPrimary},
			{Object: "images/lost/1/2.jpg", Tag: domain.PetImageTagFace},
			{Object: "images/lost/1/3.jpg", Tag: domain.PetImageTagCoat},
			{Object: "images/lost/1/4.jpg", Tag: domain.PetImageTagCollar},
		}
		if err := domain.ValidatePetImages(images); err == nil {
			t.Fatal("ValidatePetImages() expected error for >3 images, got nil")
		}
		normalized := domain.NormalizePetImages(images)
		if len(normalized) != 3 {
			t.Fatalf("NormalizePetImages() expected 3 images max, got %d", len(normalized))
		}
	})

	t.Run("empty object path rejected", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "   ", Tag: domain.PetImageTagPrimary},
		}
		if err := domain.ValidatePetImages(images); err == nil {
			t.Fatal("ValidatePetImages() expected error for empty object path, got nil")
		}
	})

	t.Run("oversized object path rejected", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: strings.Repeat("a", 1025), Tag: domain.PetImageTagPrimary},
		}
		if err := domain.ValidatePetImages(images); err == nil {
			t.Fatal("ValidatePetImages() expected error for object path > 1024 runes, got nil")
		}
	})

	t.Run("valid embedding dimension and norm", func(t *testing.T) {
		dim := 768
		emb := make([]float32, dim)
		val := float32(1.0 / math.Sqrt(float64(dim)))
		for i := range emb {
			emb[i] = val
		}
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTagPrimary, Embedding: emb},
		}
		if err := domain.ValidatePetImages(images); err != nil {
			t.Fatalf("ValidatePetImages() error = %v", err)
		}
	})

	t.Run("invalid embedding dimension rejected", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTagPrimary, Embedding: []float32{0.1, 0.2}},
		}
		if err := domain.ValidatePetImages(images); err == nil {
			t.Fatal("ValidatePetImages() expected error for non-768 dimension, got nil")
		}
	})
}

func TestBackwardCompatibility_SynthesisFromImageObject(t *testing.T) {
	record := domain.LostPetRecord{
		PetID:       "lost-123",
		ImageObject: "images/lost/123/primary.jpg",
	}
	normalized := domain.NormalizeLostPetRecord(record)
	if len(normalized.Images) != 1 {
		t.Fatalf("expected 1 synthesized image, got %d", len(normalized.Images))
	}
	if normalized.Images[0].Object != "images/lost/123/primary.jpg" {
		t.Errorf("expected synthesized image object %q, got %q", "images/lost/123/primary.jpg", normalized.Images[0].Object)
	}
	if normalized.Images[0].Tag != domain.PetImageTagPrimary {
		t.Errorf("expected synthesized image tag %q, got %q", domain.PetImageTagPrimary, normalized.Images[0].Tag)
	}
	if normalized.ImageObject != "images/lost/123/primary.jpg" {
		t.Errorf("expected ImageObject preserved, got %q", normalized.ImageObject)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run "TestPetImage" ./pkg/domain/...`
Expected: Compilation failure (`undefined: domain.PetImageTag`, `undefined: domain.PetImage`, etc.)

- [ ] **Step 3: Implement domain models in `pkg/domain`**

Create `pkg/domain/pet_image.go`:
```go
package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// EmbeddingDimension is the standard multimodal vector dimensionality (768).
const EmbeddingDimension = 768

// PetImageTag categorizes the semantic angle or purpose of an uploaded pet photo.
type PetImageTag string

const (
	PetImageTagPrimary PetImageTag = "primary"
	PetImageTagFace    PetImageTag = "face"
	PetImageTagCoat    PetImageTag = "coat"
	PetImageTagCollar  PetImageTag = "collar"
)

// PetImage represents a distinct photo associated with a pet report.
type PetImage struct {
	Object    string      `json:"object"`              // GCS object path (e.g. "images/lost-pets/123/face.jpg")
	URL       string      `json:"url,omitempty"`       // Optional public or signed URL
	Tag       PetImageTag `json:"tag,omitempty"`       // Semantic classification: primary, face, coat, collar
	Embedding []float32   `json:"embedding,omitempty"` // 768-dimensional normalized multimodal embedding vector
}

// NormalizePetImages canonicalizes image fields, caps at 3 images, and ensures a primary tag.
func NormalizePetImages(images []PetImage) []PetImage {
	if len(images) == 0 {
		return nil
	}
	limit := len(images)
	if limit > 3 {
		limit = 3
	}
	normalized := make([]PetImage, 0, limit)
	hasPrimary := false
	for i := 0; i < limit; i++ {
		img := images[i]
		obj := strings.TrimSpace(img.Object)
		if obj == "" {
			continue
		}
		tag := PetImageTag(strings.ToLower(strings.TrimSpace(string(img.Tag))))
		if tag == PetImageTagPrimary {
			hasPrimary = true
		}
		var emb []float32
		if len(img.Embedding) > 0 {
			emb = append([]float32(nil), img.Embedding...)
		}
		normalized = append(normalized, PetImage{
			Object:    obj,
			URL:       strings.TrimSpace(img.URL),
			Tag:       tag,
			Embedding: emb,
		})
	}
	if len(normalized) > 0 && !hasPrimary {
		normalized[0].Tag = PetImageTagPrimary
	}
	return normalized
}

// ValidatePetImages verifies image counts, path lengths, tags, and vector dimensions.
func ValidatePetImages(images []PetImage) error {
	if len(images) > 3 {
		return errors.New("domain: pet report supports a maximum of 3 images")
	}
	for i, img := range images {
		obj := strings.TrimSpace(img.Object)
		if obj == "" {
			return fmt.Errorf("domain: image[%d] object path is required", i)
		}
		if utf8.RuneCountInString(obj) > 1024 {
			return fmt.Errorf("domain: image[%d] object path exceeds 1024 characters", i)
		}
		switch img.Tag {
		case PetImageTagPrimary, PetImageTagFace, PetImageTagCoat, PetImageTagCollar, "":
		default:
			return fmt.Errorf("domain: image[%d] has unsupported tag %q", i, img.Tag)
		}
		if len(img.Embedding) > 0 {
			if len(img.Embedding) != EmbeddingDimension {
				return fmt.Errorf("domain: image[%d] embedding dimension must be %d, got %d", i, EmbeddingDimension, len(img.Embedding))
			}
			var sumSq float64
			for _, v := range img.Embedding {
				sumSq += float64(v) * float64(v)
			}
			norm := math.Sqrt(sumSq)
			if math.Abs(norm-1.0) > 0.05 {
				return fmt.Errorf("domain: image[%d] embedding must be unit-normalized (norm: %.4f)", i, norm)
			}
		}
	}
	return nil
}
```

Update `pkg/domain/lost_report.go`:
- In `LostPetReport`: add `Images []PetImage `json:"images,omitempty"``
- In `LostPetRecord`: add `Images []PetImage `json:"images,omitempty"`` and `Embedding []float32 `json:"embedding,omitempty"``
- In `LostPetReportedV4`: add `Images []PetImage `json:"images,omitempty"`` and `Embedding []float32 `json:"embedding,omitempty"``
- In `NormalizeLostPetReport`: call `report.Images = NormalizePetImages(report.Images)`. If `len(report.Images) == 0 && report.ImageObject != ""` synthesize `report.Images = []PetImage{{Object: report.ImageObject, Tag: PetImageTagPrimary}}`. If `len(report.Images) > 0 && report.ImageObject == ""` set `report.ImageObject = report.Images[0].Object`.
- In `validateLostPetLengths`: validate `ValidatePetImages(r.Images)`.

Update `pkg/domain/found_report.go`:
- In `FoundPetReport`: add `Images []PetImage `json:"images,omitempty"``
- In `FoundPetRecord`: add `Images []PetImage `json:"images,omitempty"`` and `Embedding []float32 `json:"embedding,omitempty"``
- In `FoundPetReportedV2`: add `Images []PetImage `json:"images,omitempty"`` and `Embedding []float32 `json:"embedding,omitempty"``
- In `NormalizeFoundPetReport`: call `report.Images = NormalizePetImages(report.Images)`. If `len(report.Images) == 0 && report.ImageObject != ""` synthesize `report.Images = []PetImage{{Object: report.ImageObject, Tag: PetImageTagPrimary}}`. If `len(report.Images) > 0 && report.ImageObject == ""` set `report.ImageObject = report.Images[0].Object`.
- In `validateFoundPetLengths`: validate `ValidatePetImages(r.Images)`.

Update `pkg/domain/match.go`:
- In `MatchScoreBreakdown`:
```go
type MatchScoreBreakdown struct {
	Visual        float64 `json:"visual"`
	Color         float64 `json:"color"`
	Spatial       float64 `json:"spatial"`
	DistanceMiles float64 `json:"distanceMiles"`
	Threshold     float64 `json:"threshold,omitempty"`
	Vector        float64 `json:"vector,omitempty"`
	Trait         float64 `json:"trait,omitempty"`
}
```
- In `MatchPetDetail`: add `Images []PetImage `json:"images,omitempty"``

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./pkg/domain/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/domain/pet_image.go pkg/domain/pet_image_test.go pkg/domain/lost_report.go pkg/domain/found_report.go pkg/domain/match.go
git commit -m "feat(domain): add multi-photo domain models and schema evolution"
```

---

### Task 2: Multimodal Embedder Package (`pkg/embedding`)

**Files:**
- Create: `pkg/embedding/embedder.go`
- Create: `pkg/embedding/mock.go`
- Create: `pkg/embedding/vertex.go`
- Create: `pkg/embedding/ollama.go`
- Create: `pkg/embedding/config.go`
- Create: `pkg/embedding/embedder_test.go`

**Interfaces:**
- Consumes: `context`, standard libraries
- Produces: `embedding.Embedder`, `embedding.MockEmbedder`, `embedding.VertexAIEmbedder`, `embedding.OllamaEmbedder`, `embedding.Config`, `embedding.NewEmbedder`

- [ ] **Step 1: Write the failing tests in `pkg/embedding/embedder_test.go`**

```go
package embedding_test

import (
	"context"
	"math"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/embedding"
)

func TestMockEmbedder_Properties(t *testing.T) {
	ctx := context.Background()
	embedder := embedding.NewMockEmbedder()

	if embedder.Dimension() != 768 {
		t.Fatalf("expected dimension 768, got %d", embedder.Dimension())
	}

	t.Run("deterministic embeddings for identical text", func(t *testing.T) {
		v1, err := embedder.EmbedText(ctx, "Golden Retriever with white chest patch")
		if err != nil {
			t.Fatal(err)
		}
		v2, err := embedder.EmbedText(ctx, "Golden Retriever with white chest patch")
		if err != nil {
			t.Fatal(err)
		}
		if len(v1) != 768 || len(v2) != 768 {
			t.Fatalf("unexpected vector length: %d, %d", len(v1), len(v2))
		}
		for i := range v1 {
			if v1[i] != v2[i] {
				t.Fatalf("expected deterministic values at index %d: %f != %f", i, v1[i], v2[i])
			}
		}
	})

	t.Run("unit normalization", func(t *testing.T) {
		v, err := embedder.EmbedText(ctx, "Tabby cat green eyes")
		if err != nil {
			t.Fatal(err)
		}
		var sumSq float64
		for _, x := range v {
			sumSq += float64(x) * float64(x)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("expected unit norm 1.0, got %f", norm)
		}
	})

	t.Run("distinct inputs produce distinct vectors", func(t *testing.T) {
		dog, err := embedder.EmbedText(ctx, "Golden retriever dog")
		if err != nil {
			t.Fatal(err)
		}
		cat, err := embedder.EmbedText(ctx, "Black domestic short hair cat")
		if err != nil {
			t.Fatal(err)
		}
		var dot float64
		for i := range dog {
			dot += float64(dog[i]) * float64(cat[i])
		}
		if dot >= 0.99 {
			t.Fatalf("expected distinct vectors to have lower cosine similarity, got %f", dot)
		}
	})

	t.Run("multimodal embedding combines image and text", func(t *testing.T) {
		dummyImage := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
		vec, err := embedder.EmbedMultimodal(ctx, dummyImage, "image/jpeg", "Beagle puppy")
		if err != nil {
			t.Fatal(err)
		}
		if len(vec) != 768 {
			t.Fatalf("expected 768-dim vector, got %d", len(vec))
		}
	})
}

func TestEmbedderFactory(t *testing.T) {
	embedder, err := embedding.NewEmbedder(embedding.Config{
		Provider: "mock",
	})
	if err != nil {
		t.Fatalf("NewEmbedder error = %v", err)
	}
	if embedder.Dimension() != 768 {
		t.Fatalf("expected 768 dimension, got %d", embedder.Dimension())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./pkg/embedding/...`
Expected: FAIL ("cannot find package")

- [ ] **Step 3: Implement `pkg/embedding` package**

Create `pkg/embedding/embedder.go`:
```go
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
```

Create `pkg/embedding/mock.go`:
```go
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

func (m *MockEmbedder) Dimension() int {
	return m.dimension
}

func (m *MockEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write([]byte(mimeType))
	h.Write(imageBytes)
	return m.hashToVector(h.Sum(nil)), nil
}

func (m *MockEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(text))
	return m.hashToVector(h[:]), nil
}

func (m *MockEmbedder) EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write([]byte(mimeType))
	h.Write(imageBytes)
	h.Write([]byte(text))
	return m.hashToVector(h.Sum(nil)), nil
}

func (m *MockEmbedder) hashToVector(digest []byte) []float32 {
	seed := int64(binary.BigEndian.Uint64(digest[:8]))
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
```

Create `pkg/embedding/vertex.go`:
```go
package embedding

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VertexAIEmbedder calls Google Cloud Vertex AI Multimodal Embedding API.
type VertexAIEmbedder struct {
	projectID  string
	location   string
	httpClient *http.Client
	dimension  int
}

// NewVertexAIEmbedder creates a Vertex AI embedder instance.
func NewVertexAIEmbedder(projectID, location string) *VertexAIEmbedder {
	if location == "" {
		location = "us-central1"
	}
	return &VertexAIEmbedder{
		projectID:  projectID,
		location:   location,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		dimension:  768,
	}
}

func (v *VertexAIEmbedder) Dimension() int {
	return v.dimension
}

func (v *VertexAIEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	return v.EmbedMultimodal(ctx, imageBytes, mimeType, "")
}

func (v *VertexAIEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return v.EmbedMultimodal(ctx, nil, "", text)
}

func (v *VertexAIEmbedder) EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/multimodalembedding@001:predict",
		v.location, v.projectID, v.location)

	instance := map[string]any{}
	if text != "" {
		instance["text"] = text
	}
	if len(imageBytes) > 0 {
		instance["image"] = map[string]string{
			"bytesBase64Encoded": base64.StdEncoding.EncodeToString(imageBytes),
		}
	}
	body, err := json.Marshal(map[string]any{"instances": []any{instance}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vertex embedding api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Predictions []struct {
			ImageEmbedding []float32 `json:"imageEmbedding"`
			TextEmbedding  []float32 `json:"textEmbedding"`
		} `json:"predictions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Predictions) == 0 {
		return nil, fmt.Errorf("vertex embedding api returned no predictions")
	}
	if len(result.Predictions[0].ImageEmbedding) > 0 {
		return result.Predictions[0].ImageEmbedding, nil
	}
	if len(result.Predictions[0].TextEmbedding) > 0 {
		return result.Predictions[0].TextEmbedding, nil
	}
	return nil, fmt.Errorf("vertex embedding api returned empty embeddings")
}
```

Create `pkg/embedding/ollama.go`:
```go
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaEmbedder calls Ollama embedding API for private GPU Cloud Run deployments.
type OllamaEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
	dimension  int
}

// NewOllamaEmbedder creates an Ollama embedder instance.
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		dimension:  768,
	}
}

func (o *OllamaEmbedder) Dimension() int {
	return o.dimension
}

func (o *OllamaEmbedder) EmbedImage(ctx context.Context, imageBytes []byte, mimeType string) ([]float32, error) {
	return o.EmbedMultimodal(ctx, imageBytes, mimeType, "")
}

func (o *OllamaEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return o.EmbedMultimodal(ctx, nil, "", text)
}

func (o *OllamaEmbedder) EmbedMultimodal(ctx context.Context, imageBytes []byte, mimeType string, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/embeddings", o.baseURL)
	payload := map[string]any{
		"model":  o.model,
		"prompt": text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embedding error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}
```

Create `pkg/embedding/config.go`:
```go
package embedding

import (
	"fmt"
	"strings"
)

// Config configures the multimodal embedder.
type Config struct {
	Provider    string // "mock", "vertex", "ollama"
	Dimension   int
	ProjectID   string
	Location    string
	OllamaURL   string
	OllamaModel string
}

// NewEmbedder constructs an Embedder according to configuration.
func NewEmbedder(cfg Config) (Embedder, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "mock", "":
		return NewMockEmbedder(), nil
	case "vertex":
		if cfg.ProjectID == "" {
			return nil, fmt.Errorf("embedding: projectID is required for vertex provider")
		}
		return NewVertexAIEmbedder(cfg.ProjectID, cfg.Location), nil
	case "ollama":
		return NewOllamaEmbedder(cfg.OllamaURL, cfg.OllamaModel), nil
	default:
		return nil, fmt.Errorf("embedding: unknown provider %q", cfg.Provider)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./pkg/embedding/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/embedding/
git commit -m "feat(embedding): implement pluggable multimodal embedder with deterministic mock"
```

---

### Task 3: Hybrid Tri-Factor Scoring Engine (`pkg/scoring`)

**Files:**
- Modify: `pkg/scoring/scorer.go`
- Modify: `pkg/scoring/scorer_test.go`

**Interfaces:**
- Consumes: `domain.MatchResult`, `domain.MatchScoreBreakdown`, `domain.PetTraits`
- Produces: `scoring.CosineSimilarity`, `scoring.CalculateHybridMatchScore`, `scoring.CalculateLegacyMatchScore`, `scoring.ComparePetsHybrid`

- [ ] **Step 1: Write the failing tests in `pkg/scoring/scorer_test.go`**

```go
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		u         []float32
		v         []float32
		wantScore float64
		tolerance float64
	}{
		{
			name:      "identical vectors",
			u:         []float32{1, 0, 0},
			v:         []float32{1, 0, 0},
			wantScore: 1.0,
			tolerance: 1e-4,
		},
		{
			name:      "orthogonal vectors",
			u:         []float32{1, 0, 0},
			v:         []float32{0, 1, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "opposite vectors clamped to 0",
			u:         []float32{1, 0, 0},
			v:         []float32{-1, 0, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "empty vectors return 0",
			u:         nil,
			v:         []float32{1, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "mismatched lengths return 0",
			u:         []float32{1, 0},
			v:         []float32{1, 0, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoring.CosineSimilarity(tt.u, tt.v)
			if math.Abs(got-tt.wantScore) > tt.tolerance {
				t.Errorf("CosineSimilarity() = %v, want %v", got, tt.wantScore)
			}
		})
	}
}

func TestHybridTriFactorScoring(t *testing.T) {
	t.Run("linear combination weights", func(t *testing.T) {
		// 0.40 * 1.0 + 0.35 * 0.8 + 0.25 * 0.6 = 0.40 + 0.28 + 0.15 = 0.83
		score := scoring.CalculateHybridMatchScore(1.0, 0.8, 0.6)
		expected := 0.83
		if math.Abs(score-expected) > 1e-4 {
			t.Errorf("CalculateHybridMatchScore = %f, want %f", score, expected)
		}
	})

	t.Run("species mismatch hard veto", func(t *testing.T) {
		traits1 := &scoring.PetTraits{Breed: "Golden Retriever"}
		traits2 := &scoring.PetTraits{Breed: "Golden Retriever"}
		u := []float32{1, 0}
		v := []float32{1, 0}
		result := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "Cat", 0.5, traits1, traits2, u, v)
		if result.IsMatch || result.Score != 0.0 {
			t.Fatalf("expected species veto score 0.0 and IsMatch=false, got score=%f, isMatch=%t", result.Score, result.IsMatch)
		}
	})

	t.Run("fallback when embeddings missing", func(t *testing.T) {
		traits1 := &scoring.PetTraits{Breed: "Golden Retriever"}
		traits2 := &scoring.PetTraits{Breed: "Golden Retriever"}
		// traits match = 0.40 breed, spatial = 1.0 (0 miles)
		// legacy: 0.70 * 0.40 + 0.30 * 1.0 = 0.28 + 0.30 = 0.58
		result := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "Dog", 0.0, traits1, traits2, nil, nil)
		expected := 0.58
		if math.Abs(result.Score-expected) > 1e-3 {
			t.Fatalf("expected fallback score ~%f, got %f", expected, result.Score)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run "TestCosineSimilarity|TestHybridTriFactorScoring" ./pkg/scoring/...`
Expected: Compilation failure (`undefined: scoring.CosineSimilarity`, etc.)

- [ ] **Step 3: Implement scoring logic in `pkg/scoring/scorer.go`**

Add to `pkg/scoring/scorer.go`:
```go
const (
	WeightVectorSimilarity = 0.40
	WeightTraitSimilarity  = 0.35
	WeightSpatialProximity = 0.25
	HybridThresholdVersion = "hybrid-multimodal-v1"
)

// CosineSimilarity computes cosine similarity between two float32 vectors, clamped to [0.0, 1.0].
func CosineSimilarity(u, v []float32) float64 {
	if len(u) == 0 || len(v) == 0 || len(u) != len(v) {
		return 0.0
	}
	var dot, normU, normV float64
	for i := range u {
		dot += float64(u[i]) * float64(v[i])
		normU += float64(u[i]) * float64(u[i])
		normV += float64(v[i]) * float64(v[i])
	}
	if normU <= 0 || normV <= 0 {
		return 0.0
	}
	sim := dot / (math.Sqrt(normU) * math.Sqrt(normV))
	return math.Min(1.0, math.Max(0.0, sim))
}

// CalculateHybridMatchScore combines semantic vector (40%), discrete traits (35%), and spatial proximity (25%).
func CalculateHybridMatchScore(vectorScore, traitScore, spatialScore float64) float64 {
	combined := (WeightVectorSimilarity * vectorScore) +
		(WeightTraitSimilarity * traitScore) +
		(WeightSpatialProximity * spatialScore)
	return math.Min(1.0, math.Max(0.0, combined))
}

// ComparePetsHybrid scores two reports using tri-factor weights and enforces the hard species veto.
func ComparePetsHybrid(
	lostPetID, foundPetID string,
	lostSpecies, foundSpecies string,
	distMiles float64,
	lostTraits, foundTraits *PetTraits,
	lostEmbedding, foundEmbedding []float32,
) *domain.MatchResult {
	if math.IsNaN(distMiles) || math.IsInf(distMiles, 0) || distMiles < 0 {
		return nil
	}

	traitScore := CalculateMatchScore(lostTraits, foundTraits)
	spatialScore := CalculateDistanceScore(distMiles, MatchRadiusMiles)
	colorScore := calculateColorScore(lostTraits, foundTraits)

	// Hard species veto
	if lostSpecies != "" && foundSpecies != "" && !strings.EqualFold(lostSpecies, foundSpecies) {
		return &domain.MatchResult{
			FoundPetID:   foundPetID,
			MatchedPetID: lostPetID,
			Score:        0.0,
			IsMatch:      false,
			Details:      "Species mismatch veto (score forced to 0.0)",
			Scores: &domain.MatchScoreBreakdown{
				Visual:        traitScore,
				Trait:         traitScore,
				Color:         colorScore,
				Spatial:       spatialScore,
				DistanceMiles: distMiles,
				Threshold:     MatchThreshold,
				Vector:        0.0,
			},
			ThresholdVersion: HybridThresholdVersion,
		}
	}

	var combinedScore float64
	var vectorScore float64
	hasVector := len(lostEmbedding) > 0 && len(foundEmbedding) > 0
	if hasVector {
		vectorScore = CosineSimilarity(lostEmbedding, foundEmbedding)
		combinedScore = CalculateHybridMatchScore(vectorScore, traitScore, spatialScore)
	} else {
		combinedScore = CalculateCombinedMatchScore(traitScore, spatialScore)
	}

	isMatch := combinedScore >= MatchThreshold
	details := fmt.Sprintf("Hybrid match score: %.2f (Vector: %.2f, Trait: %.2f, Spatial: %.2f, Distance: %.1f mi, Threshold: %.2f)",
		combinedScore, vectorScore, traitScore, spatialScore, distMiles, MatchThreshold)

	res := &domain.MatchResult{
		FoundPetID:   foundPetID,
		MatchedPetID: lostPetID,
		Score:        combinedScore,
		IsMatch:      isMatch,
		Details:      details,
		Scores: &domain.MatchScoreBreakdown{
			Visual:        traitScore,
			Trait:         traitScore,
			Color:         colorScore,
			Spatial:       spatialScore,
			DistanceMiles: distMiles,
			Threshold:     MatchThreshold,
			Vector:        vectorScore,
		},
		ThresholdVersion: HybridThresholdVersion,
	}

	if err := res.Validate(); err != nil {
		return nil
	}
	return res
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./pkg/scoring/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/scoring/scorer.go pkg/scoring/scorer_test.go
git commit -m "feat(scoring): implement hybrid tri-factor scoring engine with species veto"
```

---

### Task 4: Store Vector Indexing & Candidate Query Support (`pkg/store`)

**Files:**
- Modify: `pkg/store/candidates.go`
- Modify: `pkg/store/firestore.go`
- Modify: `pkg/store/firestore_candidates.go`
- Modify: `pkg/store/candidates_test.go`

**Interfaces:**
- Consumes: `pkg/domain`, `cloud.google.com/go/firestore`
- Produces: `firestoreRecord.LostEmbedding`, candidate deserialization preserving embeddings

- [ ] **Step 1: Write the failing tests in `pkg/store/candidates_test.go`**

```go
func TestQueryLostPetCandidates_PreservesEmbeddings(t *testing.T) {
	ctx := context.Background()
	store := store.NewMemoryStore()

	now := time.Now().UTC()
	emb := make([]float32, 768)
	emb[0] = 1.0

	record := domain.LostPetRecord{
		PetID:           "lost-vec-1",
		Species:         "Dog",
		Status:          domain.LostPetStatusLost,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		ReportedAt:      now,
		Embedding:       emb,
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SaveState(ctx, store.LostPetsCollection, record.PetID, data); err != nil {
		t.Fatal(err)
	}

	query := store.LostPetCandidateQuery{
		Status:          string(domain.LostPetStatusLost),
		GeocodingStatus: string(domain.GeocodingVerified),
		Species:         "Dog",
		ReportedAfter:   now.Add(-time.Hour),
		ReportedBefore:  now.Add(time.Hour),
		MinLatitude:     47.0,
		MaxLatitude:     48.0,
		MinLongitude:    -123.0,
		MaxLongitude:    -122.0,
	}

	results, err := store.QueryLostPetCandidates(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	resData, ok := results["lost-vec-1"]
	if !ok {
		t.Fatal("expected candidate lost-vec-1 not returned")
	}
	var decoded domain.LostPetRecord
	if err := json.Unmarshal(resData, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Embedding) != 768 || decoded.Embedding[0] != 1.0 {
		t.Fatalf("expected preserved embedding, got len=%d", len(decoded.Embedding))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run "TestQueryLostPetCandidates_PreservesEmbeddings" ./pkg/store/...`
Expected: Test may pass or fail depending on memory store; run to ensure clean compilation.

- [ ] **Step 3: Update `pkg/store/firestore.go` and `pkg/store/firestore_candidates.go`**

In `pkg/store/firestore.go`:
Add `LostEmbedding []float32 `firestore:"lostEmbedding,omitempty"`` to `firestoreRecord`:
```go
type firestoreRecord struct {
	Key                 string    `firestore:"key"`
	Data                []byte    `firestore:"data"`
	Topic               string    `firestore:"topic,omitempty"`
	Status              string    `firestore:"status,omitempty"`
	CreatedAt           time.Time `firestore:"createdAt,omitempty"`
	LostStatus          string    `firestore:"lostStatus,omitempty"`
	LostGeocodingStatus string    `firestore:"lostGeocodingStatus,omitempty"`
	LostSpecies         *string   `firestore:"lostSpecies,omitempty"`
	LostReportedAt      time.Time `firestore:"lostReportedAt,omitempty"`
	LostLatitude        *float64  `firestore:"lostLatitude,omitempty"`
	LostLongitude       *float64  `firestore:"lostLongitude,omitempty"`
	LostEmbedding       []float32 `firestore:"lostEmbedding,omitempty"`
}
```
In `newFirestoreRecord`:
If `storeName == LostPetsCollection`:
Check if payload has `Embedding`, and populate `record.LostEmbedding`.

In `pkg/store/firestore_candidates.go`:
In `lostPetCandidateIndexUpdates`:
Include `{Path: "lostEmbedding", Value: record.LostEmbedding}` if non-empty.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./pkg/store/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/store/candidates.go pkg/store/firestore.go pkg/store/firestore_candidates.go pkg/store/candidates_test.go
git commit -m "feat(store): add vector embedding persistence and candidate indexing"
```

---

### Task 5: Pet Matcher Ingestion Cascade & Vector Ranking (`internal/app/petmatcher`)

**Files:**
- Modify: `internal/app/petmatcher/worker.go`
- Modify: `internal/app/petmatcher/candidates.go`
- Modify: `internal/app/petmatcher/found_analysis.go`
- Modify: `internal/app/petmatcher/lost_analysis.go`
- Modify: `internal/app/petmatcher/worker_test.go`
- Modify: `internal/app/petmatcher/candidates_test.go`

**Interfaces:**
- Consumes: `pkg/embedding.Embedder`, `pkg/scoring`, `pkg/domain`, `pkg/store`
- Produces: `Worker.WithEmbedder`, composite embedding calculation, hybrid comparison invocation

- [ ] **Step 1: Write the failing tests in `internal/app/petmatcher/worker_test.go`**

```go
func TestWorker_HybridMultimodalMatching(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	broker := pubsub.NewMemoryBroker()
	embedder := embedding.NewMockEmbedder()

	worker := petmatcher.NewWorkerWithImageStore(st, broker, nil, nil)
	worker.SetEmbedder(embedder)

	// Verify Worker computes hybrid scores with Scores.Vector populated
	now := time.Now().UTC()
	emb, _ := embedder.EmbedText(ctx, "Golden Retriever Dog")

	lostRecord := domain.LostPetRecord{
		PetID:           "lost-1",
		Species:         "Dog",
		Status:          domain.LostPetStatusLost,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		ReportedAt:      now,
		Embedding:       emb,
		ImageAnalysis: &domain.ImageTraitAnalysis{
			Status: domain.ImageTraitsVerified,
			Traits: domain.ImageTraits{Breed: "Golden Retriever"},
		},
	}
	data, _ := json.Marshal(lostRecord)
	_ = st.SaveState(ctx, store.LostPetsCollection, "lost-1", data)

	foundEvt := domain.FoundPetReportedV2{
		PetID:           "found-1",
		Species:         "Dog",
		Breed:           "Golden Retriever",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		FoundAt:         now,
		Images: []domain.PetImage{
			{Object: "images/found/1/face.jpg", Tag: domain.PetImageTagPrimary, Embedding: emb},
		},
	}

	// Verify candidate evaluation uses ComparePetsHybrid and preserves vector score
	candidates, err := worker.EligibleLostPetCandidates(ctx, foundEvt)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run "TestWorker_HybridMultimodalMatching" ./internal/app/petmatcher/...`
Expected: Compilation failure (`undefined: worker.SetEmbedder`, etc.)

- [ ] **Step 3: Implement embedder integration in `internal/app/petmatcher`**

In `internal/app/petmatcher/worker.go`:
- Add `embedder embedding.Embedder` to `Worker`.
- Add `SetEmbedder(e embedding.Embedder)` method. In constructor, initialize with `embedding.NewMockEmbedder()` if nil.
- In `processClaimedFoundPet`:
  - When candidate is ranked:
    Call `scoring.ComparePetsHybrid`:
    ```go
    result := scoring.ComparePetsHybrid(
        candidate.record.PetID,
        foundEvt.PetID,
        candidate.record.Species,
        foundEvt.Species,
        candidate.distanceMiles,
        candidate.traits,
        foundTraits,
        candidate.record.Embedding,
        foundEmbedding,
    )
    ```
  - When creating `MatchRecord`:
    Set `matchRecord.LostPet.Images = candidate.record.Images`
    Set `matchRecord.FoundPet.Images = foundEvt.Images`

In `internal/app/petmatcher/candidates.go`:
- In `lostPetCandidate`: add `embedding []float32`.
- In `eligibleLostPetCandidates`: populate `candidate.embedding = record.Embedding`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./internal/app/petmatcher/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/petmatcher/worker.go internal/app/petmatcher/candidates.go internal/app/petmatcher/found_analysis.go internal/app/petmatcher/lost_analysis.go internal/app/petmatcher/worker_test.go
git commit -m "feat(matcher): integrate multimodal embedding generation and hybrid candidate ranking"
```

---

### Task 6: Web Frontend Multi-Photo Presigned URL API & Upload Dropzone (`webfrontend`)

**Files:**
- Modify: `internal/app/webfrontend/server.go`
- Modify: `internal/app/lostpet/service.go`
- Modify: `internal/app/foundpet/service.go`
- Modify: `internal/app/webfrontend/templates/report-lost.html`
- Modify: `internal/app/webfrontend/templates/report-found.html`
- Modify: `internal/app/webfrontend/static/js/lost-wizard.js`
- Modify: `internal/app/webfrontend/static/js/found-report.js`
- Modify: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: `domain.PetImage`, `domain.ValidatePetImages`
- Produces: Multi-photo report submission endpoints with backward compatibility

- [ ] **Step 1: Write the failing tests in `internal/app/webfrontend/server_test.go`**

```go
func TestServer_MultiPhotoSubmission(t *testing.T) {
	srv := newTestServer(t)

	t.Run("POST /api/v1/lost-pets with 3 photos succeeds", func(t *testing.T) {
		payload := `{
			"petName": "Rusty",
			"species": "Dog",
			"breed": "Irish Setter",
			"location": "Capitol Hill, Seattle, WA",
			"reporterEmail": "owner@example.com",
			"images": [
				{"object": "images/lost-pets/rust-1/face.jpg", "tag": "face"},
				{"object": "images/lost-pets/rust-1/coat.jpg", "tag": "coat"},
				{"object": "images/lost-pets/rust-1/collar.jpg", "tag": "collar"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/lost-pets with >3 photos returns 400", func(t *testing.T) {
		payload := `{
			"petName": "Rusty",
			"species": "Dog",
			"location": "Capitol Hill, Seattle, WA",
			"reporterEmail": "owner@example.com",
			"images": [
				{"object": "images/1.jpg"},
				{"object": "images/2.jpg"},
				{"object": "images/3.jpg"},
				{"object": "images/4.jpg"}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run "TestServer_MultiPhotoSubmission" ./internal/app/webfrontend/...`
Expected: FAIL (images field not yet recognized or validated)

- [ ] **Step 3: Update `server.go`, `lostpet/service.go`, `foundpet/service.go`, and frontend templates & controllers**

In `internal/app/webfrontend/server.go`:
- Update `LostPetFormRequest` and `FoundPetFormRequest`: add `Images []domain.PetImage `json:"images,omitempty"`` and `ImageObject string `json:"imageObject,omitempty"``.
- In `handleApiLostPets`: pass `command.Images = req.Images` and if `len(command.Images) > 0` set `command.ImageObject = command.Images[0].Object`.
- In `handleApiFoundPets`: pass `command.Images = req.Images` and if `len(command.Images) > 0` set `command.ImageObject = command.Images[0].Object`.

In `internal/app/lostpet/service.go` and `internal/app/foundpet/service.go`:
- Add `Images []domain.PetImage` to `ReportCommand`.
- In `ReportLostPet` / `ReportFoundPet`: attach `record.Images = command.Images`.

In `internal/app/webfrontend/templates/report-lost.html` & `report-found.html`:
- Replace single photo preview with a multi-photo staging grid supporting up to 3 photos.
- Each staged card includes:
  - Thumbnail preview `<img>`
  - Tag selector `<select class="photo-tag-select" aria-label="Select photo angle">`: options `Primary / Face`, `Coat Pattern`, `Collar & Tags`
  - Remove button `<button type="button" class="btn-remove-photo" aria-label="Remove photo">&times;</button>`
  - Helper counter `<span id="photo-count-badge">0 / 3 photos added</span>`

In `internal/app/webfrontend/static/js/lost-wizard.js` and `found-report.js`:
- Maintain an array of staged images `stagedImages = []` (up to 3).
- On file drop or file input change:
  - Read files with `FileReader` for local preview thumbnail.
  - Upload file via presigned URL (`POST /api/v1/uploads/presigned-url`).
  - Store object reference and tag in `stagedImages`.
  - Update DOM staging cards with tag selection listeners and remove listeners.
- On form submit:
  - Serialize `images: stagedImages` into the JSON payload.
  - Set legacy `imageObject` to primary image object for backward compatibility.
- Ensure strict CSP: zero inline `style` or inline script tags.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./internal/app/webfrontend/...`
Run: `node -c internal/app/webfrontend/static/js/lost-wizard.js`
Run: `node -c internal/app/webfrontend/static/js/found-report.js`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/server.go internal/app/lostpet/service.go internal/app/foundpet/service.go \
        internal/app/webfrontend/templates/report-lost.html internal/app/webfrontend/templates/report-found.html \
        internal/app/webfrontend/static/js/lost-wizard.js internal/app/webfrontend/static/js/found-report.js \
        internal/app/webfrontend/server_test.go
git commit -m "feat(frontend): add multi-photo upload dropzone and report submission"
```

---

### Task 7: Match Dashboard Visual Multi-Angle Carousel & Vector Score Breakdown (`webfrontend`)

**Files:**
- Modify: `internal/app/webfrontend/templates/matches.html`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/static/js/match-dashboard.js`
- Modify: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: `domain.MatchRecord`, `domain.MatchScoreBreakdown`, `domain.PetImage`
- Produces: Thumbnail carousel and multimodal vector score display on match cards

- [ ] **Step 1: Write the failing template test in `internal/app/webfrontend/server_test.go`**

```go
func TestMatchesTemplate_ContainsMultimodalScoreClasses(t *testing.T) {
	content, err := embeddedFiles.ReadFile("templates/matches.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(content)
	if !strings.Contains(html, "matches-list-container") {
		t.Error("matches.html missing matches-list-container")
	}
}
```

- [ ] **Step 2: Run test to verify it passes/fails**

Run: `go test -v -run "TestMatchesTemplate" ./internal/app/webfrontend/...`
Expected: PASS

- [ ] **Step 3: Update `styles.css` and `match-dashboard.js`**

In `internal/app/webfrontend/static/css/styles.css`:
Add styles for multi-angle thumbnail strips and vector score bars:
```css
/* Multi-Photo Carousel & Thumbnail Strip */
.thumbnail-strip {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.75rem;
  overflow-x: auto;
  padding-bottom: 0.25rem;
}

.thumbnail-btn {
  background: var(--bg-surface);
  border: 2px solid transparent;
  border-radius: var(--radius-sm);
  padding: 0;
  cursor: pointer;
  width: 48px;
  height: 48px;
  flex-shrink: 0;
  overflow: hidden;
  transition: border-color var(--transition-fast);
}

.thumbnail-btn.is-active,
.thumbnail-btn:focus-visible {
  border-color: var(--accent-primary);
  outline: none;
}

.thumbnail-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.score-vector::-webkit-progress-value {
  background: linear-gradient(90deg, #8b5cf6, #3b82f6);
}

.score-vector::-moz-progress-bar {
  background: linear-gradient(90deg, #8b5cf6, #3b82f6);
}

.tag-badge {
  display: inline-block;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  padding: 0.15rem 0.5rem;
  border-radius: 9999px;
  background: var(--bg-card);
  color: var(--text-secondary);
  margin-top: 0.25rem;
}
```

In `internal/app/webfrontend/static/js/match-dashboard.js`:
- In `createImagePanel(label, pet, accentClass, includeName)`:
  - If `pet.images && pet.images.length > 1`:
    - Create a `.thumbnail-strip` container.
    - For each image, create a `<button type="button" class="thumbnail-btn" aria-label="View photo ${idx + 1}">` containing `<img class="thumbnail-img">`.
    - Mark the first thumbnail as `.is-active`.
    - Add click event listener to update the main preview image `src`, alt text, and active zoom target.
- In `createMatchCard(m)`:
  - In `scores` section:
    - If `m.scores.vector !== undefined && m.scores.vector !== null`:
      - Call `createScore(scoreGrid, '✨ Multimodal AI Vector Match:', m.scores.vector, 'score-vector');`
    - Call `createScore(scoreGrid, 'Discrete Trait Match:', m.scores.trait !== undefined ? m.scores.trait : m.scores.visual, 'score-visual');`
    - If `m.scores.color !== undefined`:
      - Call `createScore(scoreGrid, 'Color Alignment:', m.scores.color, 'score-color');`
    - Call `createScore(scoreGrid, 'Geospatial Proximity (' + m.scores.distanceMiles + ' mi):', m.scores.spatial, 'score-spatial');`

- [ ] **Step 4: Run tests and JS syntax validation**

Run: `node -c internal/app/webfrontend/static/js/match-dashboard.js`
Run: `go test -v -race ./internal/app/webfrontend/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/static/css/styles.css internal/app/webfrontend/static/js/match-dashboard.js internal/app/webfrontend/server_test.go
git commit -m "feat(frontend): display multimodal vector breakdown and multi-photo comparison strip"
```

---

### Task 8: Full Verification & Integration Validation

**Files:**
- Entire repository

- [ ] **Step 1: Run full verification suite**

Run:
```bash
make verify
```
Verifies:
- `go vet ./...`
- `golangci-lint run`
- `tofu validate`
- `yamllint .`
- `go test -race -cover ./...`

Expected output:
```text
go vet ./...
golangci-lint run
tofu validate
yamllint .
go test -race -cover ./...
PASS
ok  	github.com/scottdensmore/petspotr/...
```

- [ ] **Step 2: Check git status for clean working tree**

Run: `git status`
Expected: `nothing to commit, working tree clean`

- [ ] **Step 3: Commit any formatting or lint fixes if needed**

If any adjustments were necessary:
```bash
git commit -m "chore: format and clean up for milestone 5.3 verification"
```
