package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestAcousticVoiceprint_Serialization(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		VectorDimensions: 32,
		Features:         make([]float64, 32),
		DominantPitchHz:  320.5,
		PitchVarianceHz:  15.2,
		HarmonicRatio:    0.72,
		SpectralCentroid: 1450.0,
		DurationSeconds:  2.5,
		CreatedAt:        time.Now().UTC(),
	}
	vp.Features[0] = 0.5
	vp.Features[31] = 0.85

	profile := domain.AudioProfile{
		AudioID:         "audio-test-1",
		PetID:           "pet-test-101",
		Vocalization:    domain.VocalizationCanineBark,
		ConfidenceScore: 0.88,
		Voiceprint:      vp,
		SpectrogramBins: [][]float64{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
		},
		AudioDataURI: "data:audio/wav;base64,UklGRg==",
		RecordedAt:   time.Now().UTC(),
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("failed to marshal AudioProfile: %v", err)
	}

	var decoded domain.AudioProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal AudioProfile: %v", err)
	}

	if decoded.AudioID != profile.AudioID || decoded.Vocalization != domain.VocalizationCanineBark {
		t.Errorf("expected AudioID %s and Vocalization %s, got %s and %s",
			profile.AudioID, profile.Vocalization, decoded.AudioID, decoded.Vocalization)
	}
	if len(decoded.Voiceprint.Features) != 32 {
		t.Errorf("expected 32 features, got %d", len(decoded.Voiceprint.Features))
	}
}

func TestStoreCollectionConstants(t *testing.T) {
	if store.CollectionAudioProfiles != "audio_profiles" {
		t.Errorf("unexpected CollectionAudioProfiles: %s", store.CollectionAudioProfiles)
	}
	if store.CollectionAcousticMatches != "acoustic_matches" {
		t.Errorf("unexpected CollectionAcousticMatches: %s", store.CollectionAcousticMatches)
	}
}

func TestAcousticMatchResult_Serialization(t *testing.T) {
	match := domain.AcousticMatchResult{
		ReferenceAudioID: "ref-1",
		CandidateAudioID: "cand-2",
		PetID:            "pet-101",
		SimilarityScore:  0.89,
		IsProbableMatch:  true,
		Vocalization:     domain.VocalizationCanineBark,
		MatchedAt:        time.Now().UTC(),
	}

	data, err := json.Marshal(match)
	if err != nil {
		t.Fatalf("failed to marshal AcousticMatchResult: %v", err)
	}

	var decoded domain.AcousticMatchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal AcousticMatchResult: %v", err)
	}

	if decoded.ReferenceAudioID != match.ReferenceAudioID || !decoded.IsProbableMatch {
		t.Errorf("decoded match mismatch: got %+v, want %+v", decoded, match)
	}
}

func TestSighting_AudioProfileIntegration(t *testing.T) {
	sighting := domain.Sighting{
		ID:    "sighting-101",
		PetID: "pet-101",
		AudioProfile: &domain.AudioProfile{
			AudioID:      "audio-cand-1",
			PetID:        "pet-101",
			Vocalization: domain.VocalizationCanineBark,
		},
		AcousticMatch: &domain.AcousticMatchResult{
			ReferenceAudioID: "audio-ref-1",
			CandidateAudioID: "audio-cand-1",
			PetID:            "pet-101",
			SimilarityScore:  0.88,
			IsProbableMatch:  true,
		},
	}

	data, err := json.Marshal(sighting)
	if err != nil {
		t.Fatalf("failed to marshal Sighting: %v", err)
	}

	var decoded domain.Sighting
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal Sighting: %v", err)
	}

	if decoded.AudioProfile == nil || decoded.AudioProfile.AudioID != "audio-cand-1" {
		t.Errorf("expected AudioProfile attached to Sighting, got %+v", decoded.AudioProfile)
	}
	if decoded.AcousticMatch == nil || !decoded.AcousticMatch.IsProbableMatch {
		t.Errorf("expected AcousticMatch attached to Sighting, got %+v", decoded.AcousticMatch)
	}
}

func TestPet_ReferenceAudioProfileIntegration(t *testing.T) {
	pet := domain.Pet{
		ID:         "pet-101",
		Name:       "Rex",
		OwnerEmail: "rex@example.com",
		ReferenceAudioProfile: &domain.AudioProfile{
			AudioID:      "audio-ref-1",
			PetID:        "pet-101",
			Vocalization: domain.VocalizationCanineBark,
		},
	}

	data, err := json.Marshal(pet)
	if err != nil {
		t.Fatalf("failed to marshal Pet: %v", err)
	}

	var decoded domain.Pet
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal Pet: %v", err)
	}

	if decoded.ReferenceAudioProfile == nil || decoded.ReferenceAudioProfile.AudioID != "audio-ref-1" {
		t.Errorf("expected ReferenceAudioProfile attached to Pet, got %+v", decoded.ReferenceAudioProfile)
	}
}

func TestNormalizeLostPetReport_ReferenceAudioProfileDeepCopy(t *testing.T) {
	origFeatures := []float64{0.1, 0.2, 0.3}
	origBins := [][]float64{{0.4, 0.5}, {0.6, 0.7}}

	orig := domain.LostPetReport{
		PetID:         "pet-deepcopy-1",
		ReporterEmail: "owner@example.com",
		ReferenceAudioProfile: &domain.AudioProfile{
			AudioID: "audio-prof-1",
			Voiceprint: domain.AcousticVoiceprint{
				Features: origFeatures,
			},
			SpectrogramBins: origBins,
		},
	}

	norm := domain.NormalizeLostPetReport(orig)
	if norm.ReferenceAudioProfile == nil {
		t.Fatal("expected non-nil ReferenceAudioProfile on normalized report")
	}

	// Mutate original slices
	origFeatures[0] = 999.0
	origBins[0][0] = 888.0

	if norm.ReferenceAudioProfile.Voiceprint.Features[0] == 999.0 {
		t.Errorf("Features slice was not deep-copied in NormalizeLostPetReport")
	}
	if norm.ReferenceAudioProfile.SpectrogramBins[0][0] == 888.0 {
		t.Errorf("SpectrogramBins slice was not deep-copied in NormalizeLostPetReport")
	}

	// Also verify NormalizeLostPetRecord deep copy
	rec, _ := norm.Persisted()
	normRec := domain.NormalizeLostPetRecord(rec)
	if normRec.ReferenceAudioProfile == nil {
		t.Fatal("expected non-nil ReferenceAudioProfile on normalized record")
	}

	// Mutate rec slices
	rec.ReferenceAudioProfile.Voiceprint.Features[0] = 777.0
	rec.ReferenceAudioProfile.SpectrogramBins[0][0] = 666.0

	if normRec.ReferenceAudioProfile.Voiceprint.Features[0] == 777.0 {
		t.Errorf("Features slice was not deep-copied in NormalizeLostPetRecord")
	}
	if normRec.ReferenceAudioProfile.SpectrogramBins[0][0] == 666.0 {
		t.Errorf("SpectrogramBins slice was not deep-copied in NormalizeLostPetRecord")
	}
}
