# Design Specification: Milestone 5.3 Hybrid Multimodal AI & Semantic Vector Search

- **Date:** 2026-09-13
- **Status:** Approved
- **Milestone:** 5.3 (Hybrid Multimodal AI & Semantic Vector Search)
- **Author:** Antigravity Engineering

---

## 1. Executive Summary & Goals

Milestone 5.3 elevates PetSpotR's matching intelligence by introducing multimodal AI embeddings, semantic vector similarity, and multi-photo ingestion. While previous milestones relied on discrete trait extraction (breed, colors, markings) and spatial proximity, Milestone 5.3 enables PetSpotR to perceive visual nuances (subtle fur textures, posture, facial proportions, distinctive scars, ear notches) directly through dense multimodal vector embeddings.

### Core Goals:
1. **Multi-Photo Ingestion**: Allow pet owners and community finders to upload up to 3 photos per pet report (e.g. primary facial profile, coat pattern, identifying collar/tags).
2. **Pluggable Multimodal Embedder**: Provide a clean abstraction (`pkg/embedding`) supporting Google Cloud Vertex AI Multimodal Embeddings in production, private Ollama GPU instances, and a deterministic offline `mock` embedder for unit tests and zero-dependency `make verify`.
3. **Tri-Factor Hybrid Similarity Ranking**: Fuse Semantic Vector Cosine Similarity (40%), Deterministic Trait Matching (35%), and Spatial Proximity (25%), governed by strict species-level invariants.
4. **Firestore Vector Search Compatibility**: Store 768-dimensional normalized embeddings in Firestore with native `FindNearest` vector index querying, accompanied by an in-memory cosine fallback for local development and unit tests.
5. **Multi-Angle Visual Match Comparison**: Upgrade the frontend match comparison modal to display multi-photo thumbnail strips and clear breakdowns of vector confidence alongside discrete trait scores.

---

## 2. Multi-Photo Domain Models (`pkg/domain`)

### 2.1 Domain Structs & Schema Evolution

```go
package domain

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
```

### 2.2 Aggregate Integration (`LostPetRecord` & `FoundPetRecord`)

Each report record maintains:
- `Images []PetImage`: An ordered list of up to 3 photos.
- `ImageObject string`: Canonical reference to the primary photo (`Images[0].Object`), preserving backward compatibility with legacy endpoints and events.
- `Embedding []float32`: Composite multimodal document embedding representing the pet report as a whole.

### 2.3 Validation & Normalization Rules
- **Photo Cap**: 1 to 3 images per report.
- **Primary Guarantee**: Exactly one image must have `Tag: PetImageTagPrimary` (or `Images[0]` is treated as primary).
- **Object Path Bounds**: Each `PetImage.Object` is trimmed, non-empty, and limited to 1024 UTF-8 characters.
- **Backward Compatibility**: If a legacy payload arrives with only `ImageObject` and empty `Images`, normalization synthesizes:
  ```go
  Images: []PetImage{{Object: ImageObject, Tag: PetImageTagPrimary}}
  ```
- **Embedding Invariant**: If `Embedding` is populated, it must have dimension equal to the configured model dimension (768) and be unit-normalized ($||\mathbf{v}||_2 \approx 1.0$).

---

## 3. Multimodal Embeddings Architecture (`pkg/embedding`)

### 3.1 Interface Specification

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

### 3.2 Implementations
1. **`MockEmbedder`**:
   - Computes deterministic, unit-normalized 768-dimensional float32 vectors derived from SHA-256 hashes of input bytes/strings.
   - Guaranteed identical outputs across test runs with zero network dependencies, enabling fast, isolated unit and integration testing under `make verify`.
2. **`VertexAIEmbedder`**:
   - Calls Google Cloud Vertex AI Multimodal Embedding API (`multimodalembedding@001`).
   - Authenticates using GCP Application Default Credentials (ADC).
   - Generates 768-dimensional normalized vectors.
3. **`OllamaEmbedder`**:
   - Calls private Ollama embedding endpoints hosting multimodal vision models (e.g. CLIP / Gemma embeddings).
   - Compatible with private Cloud Run GPU instances provisioned in ADR-0004.

### 3.3 Runtime Configuration Integration (`pkg/runtimeconfig`)
```env
EMBEDDING_PROVIDER=mock          # "mock", "vertex", "ollama"
EMBEDDING_DIMENSION=768
VERTEX_PROJECT_ID=petspotr-prod
VERTEX_LOCATION=us-central1
OLLAMA_EMBED_URL=http://localhost:11434
OLLAMA_EMBED_MODEL=nomic-embed-text
```

---

## 4. Hybrid Similarity Ranking & Matching Algorithm (`pkg/scoring`)

### 4.1 Tri-Factor Scoring Formula

The hybrid match score blends three distinct signals:
$$\text{CombinedScore} = (0.40 \times \text{VectorScore}) + (0.35 \times \text{TraitScore}) + (0.25 \times \text{SpatialScore})$$

1. **Semantic Vector Score (40%)**:
   - Cosine similarity between the composite multimodal embeddings of the lost and found pet reports:
     $$\text{CosineSimilarity}(\mathbf{u}, \mathbf{v}) = \frac{\sum_{i=1}^D u_i v_i}{\|\mathbf{u}\|_2 \|\mathbf{v}\|_2}$$
   - Normalized and clamped to $[0.0, 1.0]$. If either vector is nil or empty, vector score defaults to `0.0` with weight redistribution.
2. **Deterministic Trait Score (35%)**:
   - Rule-based attribute matching:
     - Breed match: 0.40
     - Primary Color match: 0.20
     - Secondary Color match: 0.10
     - Distinctive Markings match: 0.20
     - Eye Color match: 0.10
3. **Spatial Proximity Score (25%)**:
   - Linear decay based on Haversine distance up to 15 miles:
     $$\text{SpatialScore} = \max\left(0.0, 1.0 - \frac{\text{DistanceMiles}}{15.0}\right)$$

### 4.2 Species Invariant (Hard Constraint)
If `lost.Species != found.Species` (e.g. Dog vs. Cat), the match score is immediately set to `0.0` and `IsMatch = false`, regardless of vector similarity or visual features.

### 4.3 Match Threshold & Backward Compatibility Fallback
- **Threshold**: $\text{CombinedScore} \ge 0.70$.
- **Fallback for Legacy Reports**: If either report lacks an embedding vector, the engine falls back gracefully to the dual-weighted formula:
  $$\text{CombinedScore} = (0.70 \times \text{TraitScore}) + (0.30 \times \text{SpatialScore})$$

---

## 5. Event Ingestion & Matcher Cascade (`internal/app/petmatcher`)

### 5.1 Ingestion Flow
When `petmatcher` receives a `LostPetReported` or `FoundPetReported` event:
1. **Blob Retrieval**: Downloads raw image bytes for all items in `report.Images` from private Cloud Storage.
2. **Vector Generation**:
   - Calls `Embedder.EmbedMultimodal` for each image alongside the pet's text description.
   - Computes the average normalized vector across all images to produce the pet aggregate's composite `Embedding`.
3. **State Persistence**: Saves `Images` (with individual embeddings) and the composite `Embedding` into Firestore `lostPets` or `foundPets` collection.
4. **Candidate Evaluation**:
   - Queries candidates within geographic radius and 30-day window.
   - For each candidate, computes `VectorScore`, `TraitScore`, and `SpatialScore`.
   - If $\text{CombinedScore} \ge 0.70$, registers match in `matches` collection, writes bilateral `matchParticipants` records, and queues owner notifications.

---

## 6. Frontend Multi-Photo UI & Match Dashboard (`webfrontend`)

### 6.1 Multi-Photo Upload Dropzone (`pets/lost`, `pets/found`)
- Dropzone accepts up to 3 files.
- Each photo is assigned a preview tile with:
  - Thumbnail preview.
  - Tag selector dropdown or badge (`Primary / Face`, `Coat Pattern`, `Collar & Tags`).
  - Delete (`✕`) action button.
- Presigned URLs requested per staged file via `POST /api/v1/uploads/presigned-url`.
- Direct GCS upload via `PUT` with CSRF protection headers.

### 6.2 Match Comparison Dashboard Modal
- Side-by-side modal enhanced with:
  - **Multi-Photo Carousel / Strip**: Thumbnail carousel allowing volunteers and owners to cycle through all uploaded angles for both pets.
  - **Visual Score Metrics**:
    - AI Vector Match percentage bar (e.g. `88% Multimodal Match`).
    - Discrete trait breakdown (Breed, Colors, Markings, Eyes).
    - Spatial distance in miles.

### 6.3 Security & Content Security Policy (CSP)
- Zero inline styles (`style="..."`) or inline script tags.
- All styles defined via CSS classes in `styles.css`.
- Accessible ARIA labels (`aria-label="Delete uploaded coat photo"`, `role="region"` for photo preview).

---

## 7. Verification & Testing Strategy

1. **Unit Tests**:
   - `pkg/embedding`: `MockEmbedder` vector consistency and dimension tests.
   - `pkg/domain`: Multi-photo normalization, backward-compatibility synthesis, and validation edge cases.
   - `pkg/scoring`: Cosine similarity math, species veto enforcement, and hybrid weight calculations.
2. **Integration Tests**:
   - `petmatcher`: Ingestion cascade generating embeddings and executing hybrid candidate ranking against Firestore state store.
   - `webfrontend`: Presigned URL multi-file upload and multi-photo form submission.
3. **Full System Verification**:
   - `make verify` passing with 0 linter errors, 0 vet issues, and 100% test pass rate under `-race`.
