package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestImageTraitAnalysisValidationAndPrivacy(t *testing.T) {
	verifiedAt := time.Date(2026, time.August, 17, 20, 0, 0, 0, time.FixedZone("offset", -7*60*60))
	analysis := domain.NormalizeImageTraitAnalysis(&domain.ImageTraitAnalysis{
		Status: domain.ImageTraitsVerified,
		Traits: domain.PetImageTraits{
			Breed: " Golden Retriever ", PrimaryColor: " Golden ",
			DistinctiveMarkings: []string{" White chest patch ", ""},
		},
		Model: " gemma4:e2b ", AnalysisVersion: " pet-image-traits-v1 ",
		SourceEventID: " evt-lost ", SourceImageObject: " images/lost-pets/lost-1/image.jpg ",
		VerifiedAt: verifiedAt,
	})
	if err := analysis.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if analysis.Traits.Breed != "Golden Retriever" || len(analysis.Traits.DistinctiveMarkings) != 1 ||
		analysis.Model != "gemma4:e2b" || analysis.VerifiedAt.Location() != time.UTC {
		t.Fatalf("normalized analysis = %#v", analysis)
	}

	record := domain.NormalizeLostPetRecord(domain.LostPetRecord{
		PetID: "lost-1", OwnerIdentityRef: "identity-lost-1", ReportedAt: verifiedAt.Add(-time.Hour),
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		Status: domain.LostPetStatusLost, ImageAnalysis: analysis,
	})
	if record.ImageAnalysis == analysis || record.ImageAnalysis == nil {
		t.Fatal("NormalizeLostPetRecord() did not isolate image analysis")
	}
	publicData, err := json.Marshal(record.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicData), "imageAnalysis") || strings.Contains(string(publicData), "sourceImageObject") {
		t.Fatalf("public report exposed private analysis: %s", publicData)
	}

	foundRecord := domain.NormalizeFoundPetRecord(domain.FoundPetRecord{
		PetID: "found-1", ImageObject: "images/found-pets/found-1/image.jpg",
		FoundAt: verifiedAt, Location: "Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		FinderIdentityRef: "identity-found-1", Status: domain.FoundPetStatusFound,
		ImageAnalysis: analysis,
	})
	if foundRecord.ImageAnalysis == analysis || foundRecord.ImageAnalysis == nil {
		t.Fatal("NormalizeFoundPetRecord() did not isolate image analysis")
	}
	publicData, err = json.Marshal(foundRecord.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicData), "imageAnalysis") || strings.Contains(string(publicData), "sourceImageObject") {
		t.Fatalf("public found report exposed private analysis: %s", publicData)
	}
}

func TestImageTraitAnalysisRejectsIncompleteOrOversizedResults(t *testing.T) {
	valid := domain.ImageTraitAnalysis{
		Status: domain.ImageTraitsVerified,
		Traits: domain.PetImageTraits{PrimaryColor: "Black"},
		Model:  "gemma4:e2b", AnalysisVersion: "pet-image-traits-v1",
		SourceEventID: "evt-lost", SourceImageObject: "images/lost-pets/lost-1/image.jpg",
		VerifiedAt: time.Now().UTC(),
	}
	tests := []struct {
		name   string
		mutate func(*domain.ImageTraitAnalysis)
	}{
		{name: "missing traits", mutate: func(a *domain.ImageTraitAnalysis) { a.Traits = domain.PetImageTraits{} }},
		{name: "missing provenance", mutate: func(a *domain.ImageTraitAnalysis) { a.SourceEventID = "" }},
		{name: "oversized trait", mutate: func(a *domain.ImageTraitAnalysis) { a.Traits.Breed = strings.Repeat("x", 201) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := valid
			test.mutate(&analysis)
			if err := analysis.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
		})
	}
}
