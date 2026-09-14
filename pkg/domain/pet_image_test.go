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

	t.Run("unsupported tag rejected", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTag("tail")},
		}
		if err := domain.ValidatePetImages(images); err == nil {
			t.Fatal("ValidatePetImages() expected error for unsupported tag, got nil")
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

	t.Run("unnormalized embedding rejected", func(t *testing.T) {
		dim := 768
		emb := make([]float32, dim)
		for i := range emb {
			emb[i] = 1.0 // norm will be sqrt(768) ~= 27.7
		}
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTagPrimary, Embedding: emb},
		}
		if err := domain.ValidatePetImages(images); err == nil {
			t.Fatal("ValidatePetImages() expected error for unnormalized embedding, got nil")
		}
	})
}

func TestBackwardCompatibility_SynthesisFromImageObject(t *testing.T) {
	t.Run("lost pet synthesis from ImageObject", func(t *testing.T) {
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
	})

	t.Run("found pet synthesis from ImageObject", func(t *testing.T) {
		record := domain.FoundPetRecord{
			PetID:       "found-456",
			ImageObject: "images/found/456/primary.jpg",
		}
		normalized := domain.NormalizeFoundPetRecord(record)
		if len(normalized.Images) != 1 {
			t.Fatalf("expected 1 synthesized image, got %d", len(normalized.Images))
		}
		if normalized.Images[0].Object != "images/found/456/primary.jpg" {
			t.Errorf("expected synthesized image object %q, got %q", "images/found/456/primary.jpg", normalized.Images[0].Object)
		}
		if normalized.Images[0].Tag != domain.PetImageTagPrimary {
			t.Errorf("expected synthesized image tag %q, got %q", domain.PetImageTagPrimary, normalized.Images[0].Tag)
		}
		if normalized.ImageObject != "images/found/456/primary.jpg" {
			t.Errorf("expected ImageObject preserved, got %q", normalized.ImageObject)
		}
	})

	t.Run("lost pet report ImageObject synced from Images[0]", func(t *testing.T) {
		report := domain.NormalizeLostPetReport(domain.LostPetReport{
			PetID: "lost-789",
			Images: []domain.PetImage{
				{Object: "images/lost/789/face.jpg", Tag: domain.PetImageTagFace},
			},
		})
		if report.ImageObject != "images/lost/789/face.jpg" {
			t.Errorf("expected ImageObject synced to %q, got %q", "images/lost/789/face.jpg", report.ImageObject)
		}
	})

	t.Run("found pet report ImageObject synced from Images[0]", func(t *testing.T) {
		report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
			PetID: "found-789",
			Images: []domain.PetImage{
				{Object: "images/found/789/face.jpg", Tag: domain.PetImageTagFace},
			},
		})
		if report.ImageObject != "images/found/789/face.jpg" {
			t.Errorf("expected ImageObject synced to %q, got %q", "images/found/789/face.jpg", report.ImageObject)
		}
	})
}
