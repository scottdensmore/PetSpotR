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

// ValidateEmbedding verifies that an embedding vector has the correct dimension (768)
// and is Euclidean unit-normalized (norm within [0.95, 1.05]).
func ValidateEmbedding(emb []float32) error {
	if len(emb) != EmbeddingDimension {
		return fmt.Errorf("domain: embedding dimension must be %d, got %d", EmbeddingDimension, len(emb))
	}
	var sumSq float64
	for _, v := range emb {
		sumSq += float64(v) * float64(v)
	}
	norm := math.Sqrt(sumSq)
	if math.Abs(norm-1.0) > 0.05 {
		return fmt.Errorf("domain: embedding must be unit-normalized (norm: %.4f)", norm)
	}
	return nil
}

// PrimaryPetImage returns the image designated as primary, or the first image if none is explicitly tagged.
func PrimaryPetImage(images []PetImage) (PetImage, bool) {
	if len(images) == 0 {
		return PetImage{}, false
	}
	for _, img := range images {
		if img.Tag == PetImageTagPrimary {
			return img, true
		}
	}
	return images[0], true
}

// NormalizePetImages canonicalizes image fields, caps at 3 images, and ensures exactly one primary tag.
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
			if hasPrimary {
				tag = ""
			} else {
				hasPrimary = true
			}
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
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// ValidatePetImages verifies image counts, path lengths, tags, primary count, and vector dimensions.
func ValidatePetImages(images []PetImage) error {
	if len(images) > 3 {
		return errors.New("domain: pet report supports a maximum of 3 images")
	}
	primaryCount := 0
	for i, img := range images {
		obj := strings.TrimSpace(img.Object)
		if obj == "" {
			return fmt.Errorf("domain: image[%d] object path is required", i)
		}
		if utf8.RuneCountInString(obj) > 1024 {
			return fmt.Errorf("domain: image[%d] object path exceeds 1024 characters", i)
		}
		switch img.Tag {
		case PetImageTagPrimary:
			primaryCount++
		case PetImageTagFace, PetImageTagCoat, PetImageTagCollar, "":
		default:
			return fmt.Errorf("domain: image[%d] has unsupported tag %q", i, img.Tag)
		}
		if len(img.Embedding) > 0 {
			if err := ValidateEmbedding(img.Embedding); err != nil {
				return fmt.Errorf("domain: image[%d]: %w", i, err)
			}
		}
	}
	if primaryCount > 1 {
		return fmt.Errorf("domain: pet report supports at most 1 primary image, got %d", primaryCount)
	}
	return nil
}

func clonePetImages(images []PetImage) []PetImage {
	if len(images) == 0 {
		return nil
	}
	cloned := make([]PetImage, len(images))
	for i, img := range images {
		cloned[i] = img
		if len(img.Embedding) > 0 {
			cloned[i].Embedding = append([]float32(nil), img.Embedding...)
		}
	}
	return cloned
}
