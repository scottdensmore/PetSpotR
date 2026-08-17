package petmatcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func (w *Worker) foundImageTraits(
	ctx context.Context,
	inputEventID string,
	foundEvent domain.FoundPetReportedV2,
) (*scoring.PetTraits, string, error) {
	if foundEvent.ImageObject == "" {
		return w.generateFoundImageTraits(ctx, foundEvent.ImageURL)
	}
	if !blob.IsFinalizedImageForPurpose(blob.ImagePurposeFoundPet, foundEvent.PetID, foundEvent.ImageObject) {
		return nil, "", errors.New("pet-matcher: found-pet event image is outside the found-pet namespace")
	}
	record, err := w.loadFoundPetRecord(ctx, foundEvent.PetID)
	if err != nil {
		return nil, "", err
	}
	if record.ImageObject != foundEvent.ImageObject {
		return nil, "", errors.New("pet-matcher: found-pet event image does not match durable state")
	}
	if record.ImageAnalysis != nil {
		return verifiedFoundTraits(record.ImageAnalysis, inputEventID, foundEvent.ImageObject)
	}
	if w.images == nil {
		return nil, "", errors.New("pet-matcher: private image store is not configured")
	}
	imageBytes, err := w.images.ReadFinalizedImage(ctx, foundEvent.ImageObject)
	if err != nil {
		return nil, "", fmt.Errorf("pet-matcher: read private found-pet image: %w", err)
	}
	traits, model, err := w.generateFoundImageTraits(ctx, base64.StdEncoding.EncodeToString(imageBytes))
	if err != nil {
		return nil, "", err
	}
	analysis := domain.NormalizeImageTraitAnalysis(&domain.ImageTraitAnalysis{
		Status: domain.ImageTraitsVerified,
		Traits: domain.PetImageTraits{
			Breed: traits.Breed, PrimaryColor: traits.PrimaryColor,
			SecondaryColor:      traits.SecondaryColor,
			DistinctiveMarkings: append([]string(nil), traits.DistinctiveMarkings...),
			EyeColor:            traits.EyeColor,
		},
		Model: model, AnalysisVersion: imageTraitAnalysisVersion,
		SourceEventID: inputEventID, SourceImageObject: foundEvent.ImageObject,
		VerifiedAt: w.now().UTC(),
	})
	if err := analysis.Validate(); err != nil {
		return nil, "", fmt.Errorf("pet-matcher: validate found image analysis: %w", err)
	}
	err = w.store.UpdateState(ctx, store.FoundPetsCollection, foundEvent.PetID, func(current []byte) ([]byte, error) {
		var updated domain.FoundPetRecord
		if err := json.Unmarshal(current, &updated); err != nil {
			return nil, fmt.Errorf("pet-matcher: decode durable found-pet state: %w", err)
		}
		updated = domain.NormalizeFoundPetRecord(updated)
		if updated.PetID != foundEvent.PetID || updated.ImageObject != foundEvent.ImageObject {
			return nil, errors.New("pet-matcher: found-pet image changed before analysis persistence")
		}
		if updated.ImageAnalysis != nil {
			if _, _, err := verifiedFoundTraits(updated.ImageAnalysis, inputEventID, foundEvent.ImageObject); err != nil {
				return nil, err
			}
			return current, nil
		}
		updated.ImageAnalysis = analysis
		return json.Marshal(updated)
	})
	if err != nil {
		return nil, "", fmt.Errorf("pet-matcher: persist found image analysis: %w", err)
	}
	committed, err := w.loadFoundPetRecord(ctx, foundEvent.PetID)
	if err != nil {
		return nil, "", err
	}
	return verifiedFoundTraits(committed.ImageAnalysis, inputEventID, foundEvent.ImageObject)
}

func (w *Worker) generateFoundImageTraits(ctx context.Context, image string) (*scoring.PetTraits, string, error) {
	if w.ollamaClient == nil {
		return nil, "", errors.New("pet-matcher: Ollama client is not configured")
	}
	response, err := w.ollamaClient.Generate(ctx, &ollama.GenerateRequest{
		Model: w.modelName, Prompt: scoring.BuildGemmaPrompt("Pet", ""), Images: []string{image},
	})
	if err != nil {
		return nil, "", fmt.Errorf("pet-matcher: Ollama generation failed: %w", err)
	}
	traits, err := scoring.ParseGemmaResponse(response.Response)
	if err != nil {
		return nil, "", fmt.Errorf("pet-matcher: failed to parse traits from Gemma response: %w", err)
	}
	model := strings.TrimSpace(response.Model)
	if model == "" {
		model = w.modelName
	}
	return traits, model, nil
}

func verifiedFoundTraits(
	analysis *domain.ImageTraitAnalysis,
	inputEventID string,
	imageObject string,
) (*scoring.PetTraits, string, error) {
	analysis = domain.NormalizeImageTraitAnalysis(analysis)
	if analysis == nil || analysis.SourceEventID != inputEventID || analysis.SourceImageObject != imageObject {
		return nil, "", errors.New("pet-matcher: persisted found image analysis has mismatched provenance")
	}
	if err := analysis.Validate(); err != nil {
		return nil, "", fmt.Errorf("pet-matcher: validate persisted found image analysis: %w", err)
	}
	return &scoring.PetTraits{
		Breed: analysis.Traits.Breed, PrimaryColor: analysis.Traits.PrimaryColor,
		SecondaryColor:      analysis.Traits.SecondaryColor,
		DistinctiveMarkings: append([]string(nil), analysis.Traits.DistinctiveMarkings...),
		EyeColor:            analysis.Traits.EyeColor,
	}, analysis.Model, nil
}

func (w *Worker) loadFoundPetRecord(ctx context.Context, petID string) (domain.FoundPetRecord, error) {
	data, err := w.store.GetState(ctx, store.FoundPetsCollection, petID)
	if err != nil {
		return domain.FoundPetRecord{}, fmt.Errorf("pet-matcher: load durable found-pet state: %w", err)
	}
	var record domain.FoundPetRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return domain.FoundPetRecord{}, fmt.Errorf("pet-matcher: decode durable found-pet state: %w", err)
	}
	record = domain.NormalizeFoundPetRecord(record)
	if record.PetID != petID {
		return domain.FoundPetRecord{}, errors.New("pet-matcher: durable found-pet identity does not match event")
	}
	return record, nil
}
