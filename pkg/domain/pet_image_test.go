package domain_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func makeUnitEmbedding(dim int) []float32 {
	emb := make([]float32, dim)
	val := float32(1.0 / math.Sqrt(float64(dim)))
	for i := range emb {
		emb[i] = val
	}
	return emb
}

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

	t.Run("multiple primary tags rejected in ValidatePetImages", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTagPrimary},
			{Object: "images/lost/1/2.jpg", Tag: domain.PetImageTagPrimary},
		}
		if err := domain.ValidatePetImages(images); err == nil {
			t.Fatal("ValidatePetImages() expected error for multiple primary tags, got nil")
		}
	})

	t.Run("multiple primary tags demoted in NormalizePetImages", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTagPrimary},
			{Object: "images/lost/1/2.jpg", Tag: domain.PetImageTagPrimary},
		}
		normalized := domain.NormalizePetImages(images)
		if len(normalized) != 2 {
			t.Fatalf("expected 2 images, got %d", len(normalized))
		}
		if normalized[0].Tag != domain.PetImageTagPrimary {
			t.Errorf("expected first image tag %q, got %q", domain.PetImageTagPrimary, normalized[0].Tag)
		}
		if normalized[1].Tag != "" {
			t.Errorf("expected second image tag to be demoted to empty, got %q", normalized[1].Tag)
		}
		if err := domain.ValidatePetImages(normalized); err != nil {
			t.Fatalf("ValidatePetImages() on normalized error = %v", err)
		}
	})

	t.Run("single non-first primary preserved in NormalizePetImages", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "images/lost/1/1.jpg", Tag: domain.PetImageTagFace},
			{Object: "images/lost/1/2.jpg", Tag: domain.PetImageTagPrimary},
		}
		normalized := domain.NormalizePetImages(images)
		if normalized[0].Tag != domain.PetImageTagFace {
			t.Errorf("expected first image tag to remain %q, got %q", domain.PetImageTagFace, normalized[0].Tag)
		}
		if normalized[1].Tag != domain.PetImageTagPrimary {
			t.Errorf("expected second image tag to remain %q, got %q", domain.PetImageTagPrimary, normalized[1].Tag)
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
		emb := makeUnitEmbedding(domain.EmbeddingDimension)
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
		dim := domain.EmbeddingDimension
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

func TestPrimaryPetImage(t *testing.T) {
	t.Run("empty slice returns false", func(t *testing.T) {
		_, ok := domain.PrimaryPetImage(nil)
		if ok {
			t.Fatal("expected false for nil slice")
		}
	})

	t.Run("finds primary tag at non-first index", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "img-0.jpg", Tag: domain.PetImageTagFace},
			{Object: "img-1.jpg", Tag: domain.PetImageTagPrimary},
			{Object: "img-2.jpg", Tag: domain.PetImageTagCoat},
		}
		primary, ok := domain.PrimaryPetImage(images)
		if !ok || primary.Object != "img-1.jpg" {
			t.Fatalf("expected img-1.jpg as primary, got %#v (ok=%v)", primary, ok)
		}
	})

	t.Run("falls back to first image if none tagged primary", func(t *testing.T) {
		images := []domain.PetImage{
			{Object: "img-0.jpg", Tag: domain.PetImageTagFace},
			{Object: "img-1.jpg", Tag: domain.PetImageTagCoat},
		}
		primary, ok := domain.PrimaryPetImage(images)
		if !ok || primary.Object != "img-0.jpg" {
			t.Fatalf("expected img-0.jpg as fallback, got %#v (ok=%v)", primary, ok)
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

	t.Run("lost pet report ImageObject and Embedding synced from primary image at index 1", func(t *testing.T) {
		emb := makeUnitEmbedding(domain.EmbeddingDimension)
		report := domain.NormalizeLostPetReport(domain.LostPetReport{
			PetID: "lost-789",
			Images: []domain.PetImage{
				{Object: "images/lost/789/face.jpg", Tag: domain.PetImageTagFace},
				{Object: "images/lost/789/primary.jpg", Tag: domain.PetImageTagPrimary, Embedding: emb},
			},
		})
		if report.ImageObject != "images/lost/789/primary.jpg" {
			t.Errorf("expected ImageObject synced from primary at index 1 %q, got %q", "images/lost/789/primary.jpg", report.ImageObject)
		}
		persisted, _ := report.Persisted()
		if len(persisted.Embedding) != domain.EmbeddingDimension {
			t.Fatalf("expected persisted embedding dimension %d, got %d", domain.EmbeddingDimension, len(persisted.Embedding))
		}
		event := report.ReportedEvent()
		if len(event.Embedding) != domain.EmbeddingDimension {
			t.Fatalf("expected event embedding dimension %d, got %d", domain.EmbeddingDimension, len(event.Embedding))
		}
	})

	t.Run("found pet report ImageObject and Embedding synced from primary image at index 1", func(t *testing.T) {
		emb := makeUnitEmbedding(domain.EmbeddingDimension)
		report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
			PetID: "found-789",
			Images: []domain.PetImage{
				{Object: "images/found/789/face.jpg", Tag: domain.PetImageTagFace},
				{Object: "images/found/789/primary.jpg", Tag: domain.PetImageTagPrimary, Embedding: emb},
			},
		})
		if report.ImageObject != "images/found/789/primary.jpg" {
			t.Errorf("expected ImageObject synced from primary at index 1 %q, got %q", "images/found/789/primary.jpg", report.ImageObject)
		}
		persisted, _ := report.Persisted()
		if len(persisted.Embedding) != domain.EmbeddingDimension {
			t.Fatalf("expected persisted embedding dimension %d, got %d", domain.EmbeddingDimension, len(persisted.Embedding))
		}
		event := report.ReportedEvent()
		if len(event.Embedding) != domain.EmbeddingDimension {
			t.Fatalf("expected event embedding dimension %d, got %d", domain.EmbeddingDimension, len(event.Embedding))
		}
	})
}

func TestAggregateEmbeddingValidationOnEvents(t *testing.T) {
	now := time.Now().UTC()

	t.Run("LostPetReportedV4 rejects invalid embedding dimension", func(t *testing.T) {
		event := domain.LostPetReportedV4{
			PetID:       "lost-emb",
			ReportedAt:  now,
			Location:    "Seattle, WA",
			ImageObject: "images/lost/emb.jpg",
			Embedding:   []float32{0.1, 0.2},
		}
		if err := event.Validate(); err == nil {
			t.Fatal("expected error for non-768 dimension embedding, got nil")
		}
	})

	t.Run("LostPetReportedV4 rejects unnormalized embedding", func(t *testing.T) {
		emb := make([]float32, domain.EmbeddingDimension)
		for i := range emb {
			emb[i] = 1.0
		}
		event := domain.LostPetReportedV4{
			PetID:       "lost-emb",
			ReportedAt:  now,
			Location:    "Seattle, WA",
			ImageObject: "images/lost/emb.jpg",
			Embedding:   emb,
		}
		if err := event.Validate(); err == nil {
			t.Fatal("expected error for unnormalized embedding, got nil")
		}
	})

	t.Run("FoundPetReportedV2 rejects invalid embedding dimension", func(t *testing.T) {
		event := domain.FoundPetReportedV2{
			PetID:       "found-emb",
			FoundAt:     now,
			Location:    "Seattle, WA",
			ImageObject: "images/found/emb.jpg",
			Embedding:   []float32{0.1, 0.2},
		}
		if err := event.Validate(); err == nil {
			t.Fatal("expected error for non-768 dimension embedding, got nil")
		}
	})

	t.Run("FoundPetReportedV2 rejects unnormalized embedding", func(t *testing.T) {
		emb := make([]float32, domain.EmbeddingDimension)
		for i := range emb {
			emb[i] = 1.0
		}
		event := domain.FoundPetReportedV2{
			PetID:       "found-emb",
			FoundAt:     now,
			Location:    "Seattle, WA",
			ImageObject: "images/found/emb.jpg",
			Embedding:   emb,
		}
		if err := event.Validate(); err == nil {
			t.Fatal("expected error for unnormalized embedding, got nil")
		}
	})

	t.Run("LostPetReportedV4 and FoundPetReportedV2 accept valid unit embedding", func(t *testing.T) {
		emb := makeUnitEmbedding(domain.EmbeddingDimension)
		lostEvent := domain.LostPetReportedV4{
			PetID:           "lost-emb",
			ReportedAt:      now,
			Location:        "Seattle, WA",
			ImageObject:     "images/lost/emb.jpg",
			GeocodingStatus: domain.GeocodingPending,
			Status:          domain.LostPetStatusLost,
			Embedding:       emb,
		}
		if err := lostEvent.Validate(); err != nil {
			t.Fatalf("LostPetReportedV4.Validate() error = %v", err)
		}
		foundEvent := domain.FoundPetReportedV2{
			PetID:           "found-emb",
			FoundAt:         now,
			Location:        "Seattle, WA",
			ImageObject:     "images/found/emb.jpg",
			GeocodingStatus: domain.GeocodingPending,
			Status:          domain.FoundPetStatusFound,
			CustodyStatus:   domain.CustodyFinderHome,
			Embedding:       emb,
		}
		if err := foundEvent.Validate(); err != nil {
			t.Fatalf("FoundPetReportedV2.Validate() error = %v", err)
		}
	})
}
