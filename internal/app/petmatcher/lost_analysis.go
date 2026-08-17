package petmatcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const (
	lostImageAnalysisChannel = "lostPetImageAnalysis"
	lostImageAnalysisVersion = "pet-image-traits-v1"
)

// ProcessLostPet analyzes a finalized lost-pet image asynchronously and
// persists verified model-derived traits on the existing lost report.
func (w *Worker) ProcessLostPet(ctx context.Context, lostPetData []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	lostEvent, inputEnvelope, err := domain.DecodeLostPetReported(lostPetData)
	if err != nil {
		return fmt.Errorf("pet-matcher: decode lost-pet report: %w", err)
	}
	envelopeID := ""
	if inputEnvelope != nil {
		envelopeID = inputEnvelope.ID
	}
	inputEventID, err := delivery.ResolveEventID(envelopeID, domain.EventTypeLostPetReported, lostPetData)
	if err != nil {
		return fmt.Errorf("pet-matcher: resolve lostPet identity: %w", err)
	}
	now := w.now().UTC()
	operation, err := delivery.NewOperation(inputEventID, lostEvent.PetID, lostImageAnalysisChannel, now)
	if err != nil {
		return fmt.Errorf("pet-matcher: create lost image analysis operation: %w", err)
	}
	claim, err := w.store.ClaimDeliveryOperation(ctx, operation, now, w.lease)
	if err != nil {
		return fmt.Errorf("pet-matcher: claim lost image analysis: %w", err)
	}
	switch claim.State {
	case delivery.ClaimCompleted:
		return nil
	case delivery.ClaimInProgress:
		return fmt.Errorf("pet-matcher: lost image analysis: %w", delivery.ErrOperationInProgress)
	case delivery.ClaimAcquired:
	default:
		return fmt.Errorf("pet-matcher: unexpected lost image analysis claim %q", claim.State)
	}

	processErr := w.processClaimedLostPet(ctx, inputEventID, lostEvent)
	if processErr == nil {
		processErr = w.store.CompleteDeliveryOperation(ctx, operation.ID, claim.Attempt, w.now().UTC())
	}
	if processErr == nil {
		return nil
	}
	failErr := w.store.FailDeliveryOperation(ctx, operation.ID, claim.Attempt, w.now().UTC(), processErr.Error())
	return errors.Join(processErr, failErr)
}

func (w *Worker) processClaimedLostPet(
	ctx context.Context,
	inputEventID string,
	lostEvent domain.LostPetReportedV4,
) error {
	if lostEvent.ImageObject == "" {
		return nil
	}
	if !blob.IsFinalizedImageForPurpose(blob.ImagePurposeLostPet, lostEvent.PetID, lostEvent.ImageObject) {
		return errors.New("pet-matcher: lost-pet event image is outside the lost-pet namespace")
	}
	record, err := w.loadLostPetRecord(ctx, lostEvent.PetID)
	if err != nil {
		return err
	}
	if record.ImageObject != lostEvent.ImageObject {
		return errors.New("pet-matcher: lost-pet event image does not match durable state")
	}
	if record.ImageAnalysis != nil && record.ImageAnalysis.SourceEventID == inputEventID {
		if record.ImageAnalysis.SourceImageObject != lostEvent.ImageObject {
			return errors.New("pet-matcher: completed lost image analysis has mismatched provenance")
		}
		return record.ImageAnalysis.Validate()
	}
	if w.images == nil {
		return errors.New("pet-matcher: private image store is not configured")
	}
	if w.ollamaClient == nil {
		return errors.New("pet-matcher: Ollama client is not configured")
	}
	imageBytes, err := w.images.ReadFinalizedImage(ctx, lostEvent.ImageObject)
	if err != nil {
		return fmt.Errorf("pet-matcher: read private lost-pet image: %w", err)
	}
	response, err := w.ollamaClient.Generate(ctx, &ollama.GenerateRequest{
		Model:  w.modelName,
		Prompt: scoring.BuildGemmaPrompt(lostEvent.Species, lostEvent.Breed),
		Images: []string{base64.StdEncoding.EncodeToString(imageBytes)},
	})
	if err != nil {
		return fmt.Errorf("pet-matcher: lost image Ollama generation failed: %w", err)
	}
	traits, err := scoring.ParseGemmaResponse(response.Response)
	if err != nil {
		return fmt.Errorf("pet-matcher: parse lost image traits: %w", err)
	}
	model := strings.TrimSpace(response.Model)
	if model == "" {
		model = w.modelName
	}
	analysis := domain.NormalizeImageTraitAnalysis(&domain.ImageTraitAnalysis{
		Status: domain.ImageTraitsVerified,
		Traits: domain.PetImageTraits{
			Breed: traits.Breed, PrimaryColor: traits.PrimaryColor,
			SecondaryColor:      traits.SecondaryColor,
			DistinctiveMarkings: append([]string(nil), traits.DistinctiveMarkings...),
			EyeColor:            traits.EyeColor,
		},
		Model: model, AnalysisVersion: lostImageAnalysisVersion,
		SourceEventID: inputEventID, SourceImageObject: lostEvent.ImageObject,
		VerifiedAt: w.now().UTC(),
	})
	if err := analysis.Validate(); err != nil {
		return fmt.Errorf("pet-matcher: validate lost image analysis: %w", err)
	}
	err = w.store.UpdateState(ctx, store.LostPetsCollection, lostEvent.PetID, func(current []byte) ([]byte, error) {
		var updated domain.LostPetRecord
		if err := json.Unmarshal(current, &updated); err != nil {
			return nil, fmt.Errorf("pet-matcher: decode durable lost-pet state: %w", err)
		}
		updated = domain.NormalizeLostPetRecord(updated)
		if updated.PetID != lostEvent.PetID || updated.ImageObject != lostEvent.ImageObject {
			return nil, errors.New("pet-matcher: lost-pet image changed before analysis persistence")
		}
		if updated.ImageAnalysis != nil {
			if updated.ImageAnalysis.SourceEventID == inputEventID {
				if err := updated.ImageAnalysis.Validate(); err != nil {
					return nil, err
				}
				return current, nil
			}
			return nil, fmt.Errorf("%w: lost-pet image already has analysis provenance", store.ErrConflict)
		}
		updated.ImageAnalysis = analysis
		return json.Marshal(updated)
	})
	if err != nil {
		return fmt.Errorf("pet-matcher: persist lost image analysis: %w", err)
	}
	return nil
}

func (w *Worker) loadLostPetRecord(ctx context.Context, petID string) (domain.LostPetRecord, error) {
	data, err := w.store.GetState(ctx, store.LostPetsCollection, petID)
	if err != nil {
		return domain.LostPetRecord{}, fmt.Errorf("pet-matcher: load durable lost-pet state: %w", err)
	}
	var record domain.LostPetRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return domain.LostPetRecord{}, fmt.Errorf("pet-matcher: decode durable lost-pet state: %w", err)
	}
	record = domain.NormalizeLostPetRecord(record)
	if record.PetID != petID {
		return domain.LostPetRecord{}, errors.New("pet-matcher: durable lost-pet identity does not match event")
	}
	return record, nil
}
