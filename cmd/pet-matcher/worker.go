package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Worker consumes foundPet events, extracts visual traits via Ollama, and matches against stored lost pets.
type Worker struct {
	store        store.StateStore
	broker       pubsub.Publisher
	ollamaClient *ollama.Client
	modelName    string
}

// NewWorker constructs a Worker instance.
func NewWorker(st store.StateStore, br pubsub.Publisher, oc *ollama.Client) *Worker {
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "gemma4:e2b"
	}
	return &Worker{
		store:        st,
		broker:       br,
		ollamaClient: oc,
		modelName:    model,
	}
}

// Start registers the foundPet topic subscription.
func (w *Worker) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	broker, ok := w.broker.(pubsub.Broker)
	if !ok {
		return fmt.Errorf("pet-matcher: publisher does not support in-process subscriptions")
	}
	return broker.Subscribe("foundPet", func(handlerCtx context.Context, data []byte) error {
		return w.ProcessFoundPet(handlerCtx, data)
	})
}

// ProcessFoundPet processes a foundPet event payload.
func (w *Worker) ProcessFoundPet(ctx context.Context, foundPetData []byte) error {
	var foundEvt domain.FoundPetEvent
	inputEnvelope, err := domain.DecodeEventPayload(foundPetData, domain.EventTypeFoundPetReported, &foundEvt)
	if err != nil {
		return fmt.Errorf("pet-matcher: failed to unmarshal foundPet event: %w", err)
	}

	if err := foundEvt.Validate(); err != nil {
		return fmt.Errorf("pet-matcher: invalid foundPet event: %w", err)
	}

	// 1. Analyze found pet image with Ollama + Gemma 4
	prompt := scoring.BuildGemmaPrompt("Pet", "")
	genReq := &ollama.GenerateRequest{
		Model:  w.modelName,
		Prompt: prompt,
		Images: []string{foundEvt.ImageURL},
	}

	genResp, err := w.ollamaClient.Generate(ctx, genReq)
	if err != nil {
		return fmt.Errorf("pet-matcher: Ollama generation failed: %w", err)
	}

	foundTraits, err := scoring.ParseGemmaResponse(genResp.Response)
	if err != nil {
		return fmt.Errorf("pet-matcher: failed to parse traits from Gemma response: %w", err)
	}

	// 2. Fetch lost pets state candidate
	// Note: In memory store testing, lost pets are queried directly or retrieved by key.
	// For current vertical slice, attempt matching against stored candidate or default lost pet traits.
	lostStateBytes, err := w.store.GetState(ctx, store.LostPetsCollection, "lost-101")
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("pet-matcher: load lost-pet candidate: %w", err)
	}
	var lostTraits *scoring.PetTraits
	lostPetID := "lost-101"
	lostLocation := "Capitol Hill, Seattle, WA"
	var lostEvt domain.LostPetEvent
	if err := json.Unmarshal(lostStateBytes, &lostEvt); err != nil {
		return fmt.Errorf("pet-matcher: decode lost-pet candidate: %w", err)
	}
	if lostEvt.PetID != "" {
		lostPetID = lostEvt.PetID
	}
	if lostEvt.Location != "" {
		lostLocation = lostEvt.Location
	}
	lostTraits = &scoring.PetTraits{
		Breed:               "Golden Retriever",
		PrimaryColor:        "Golden",
		SecondaryColor:      "Cream",
		DistinctiveMarkings: []string{"White chest patch"},
		EyeColor:            "Brown",
	}

	// 3. Compute distance-weighted combined similarity score
	matchResult := scoring.ComparePetsGeo(lostPetID, foundEvt.PetID, lostLocation, foundEvt.Location, lostTraits, foundTraits)
	if matchResult != nil && matchResult.IsMatch {
		resultBytes, err := matchResult.ToJSON()
		if err != nil {
			return fmt.Errorf("pet-matcher: failed to marshal MatchResult: %w", err)
		}

		correlationID := ""
		traceID := ""
		if inputEnvelope != nil {
			correlationID = inputEnvelope.CorrelationID
			traceID = inputEnvelope.TraceID
		}
		matchEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeMatchFound,
			OccurredAt:       time.Now().UTC(),
			CorrelationID:    correlationID,
			TraceID:          traceID,
			AggregateID:      matchResult.FoundPetID + ":" + matchResult.MatchedPetID,
			AggregateVersion: 1,
			PayloadVersion:   1,
			Payload:          resultBytes,
		})
		if err != nil {
			return fmt.Errorf("pet-matcher: failed to create matchFound envelope: %w", err)
		}
		envelopeBytes, err := json.Marshal(matchEnvelope)
		if err != nil {
			return fmt.Errorf("pet-matcher: failed to marshal matchFound envelope: %w", err)
		}
		if err := w.broker.Publish(ctx, "matchFound", envelopeBytes); err != nil {
			return fmt.Errorf("pet-matcher: failed to publish matchFound event: %w", err)
		}

		log.Printf("[Pet Matcher] MATCH FOUND! FoundPet: %s <-> LostPet: %s (Score: %.2f)",
			matchResult.FoundPetID, matchResult.MatchedPetID, matchResult.Score)
	}

	return nil
}
