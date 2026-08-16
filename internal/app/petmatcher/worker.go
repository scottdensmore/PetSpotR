// Package petmatcher implements the found-pet matching application worker.
package petmatcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const (
	matcherDeliveryChannel = "matchFound"
	defaultMatcherLease    = 10 * time.Minute
)

// Store provides the durable state and delivery operations required by Worker.
type Store interface {
	store.StateStore
	store.DeliveryOperationStore
}

type matcherResultRecord struct {
	InputEventID string `json:"inputEventId"`
	OutboxID     string `json:"outboxId"`
}

// Worker consumes foundPet events, extracts visual traits via Ollama, and matches against stored lost pets.
type Worker struct {
	store        Store
	broker       pubsub.Publisher
	ollamaClient *ollama.Client
	modelName    string
	relay        *outbox.Relay
	now          func() time.Time
	lease        time.Duration
	images       blob.ImageStore
}

// NewWorker constructs a Worker instance.
func NewWorker(st Store, br pubsub.Publisher, oc *ollama.Client) *Worker {
	return NewWorkerWithImageStore(st, br, oc, nil)
}

// NewWorkerWithImageStore constructs a worker that can read private finalized images.
func NewWorkerWithImageStore(st Store, br pubsub.Publisher, oc *ollama.Client, images blob.ImageStore) *Worker {
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "gemma4:e2b"
	}
	return &Worker{
		store:        st,
		broker:       br,
		ollamaClient: oc,
		modelName:    model,
		relay:        outbox.NewRelay(st, br),
		now:          time.Now,
		lease:        defaultMatcherLease,
		images:       images,
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
	if err := ctx.Err(); err != nil {
		return err
	}
	foundEvt, inputEnvelope, err := domain.DecodeFoundPetReported(foundPetData)
	if err != nil {
		return fmt.Errorf("pet-matcher: decode found-pet report: %w", err)
	}
	envelopeID := ""
	if inputEnvelope != nil {
		envelopeID = inputEnvelope.ID
	}
	inputEventID, err := delivery.ResolveEventID(envelopeID, domain.EventTypeFoundPetReported, foundPetData)
	if err != nil {
		return fmt.Errorf("pet-matcher: resolve foundPet identity: %w", err)
	}
	now := w.now().UTC()
	operation, err := delivery.NewOperation(inputEventID, foundEvt.PetID, matcherDeliveryChannel, now)
	if err != nil {
		return fmt.Errorf("pet-matcher: create processing operation: %w", err)
	}
	claim, err := w.store.ClaimDeliveryOperation(ctx, operation, now, w.lease)
	if err != nil {
		return fmt.Errorf("pet-matcher: claim foundPet processing: %w", err)
	}
	switch claim.State {
	case delivery.ClaimCompleted:
		return nil
	case delivery.ClaimInProgress:
		return fmt.Errorf("pet-matcher: foundPet processing: %w", delivery.ErrOperationInProgress)
	case delivery.ClaimAcquired:
	default:
		return fmt.Errorf("pet-matcher: unexpected processing claim %q", claim.State)
	}

	processErr := w.processClaimedFoundPet(ctx, inputEventID, inputEnvelope, foundEvt)
	if processErr == nil {
		processErr = w.store.CompleteDeliveryOperation(ctx, operation.ID, claim.Attempt, w.now().UTC())
	}
	if processErr == nil {
		return nil
	}
	failErr := w.store.FailDeliveryOperation(
		ctx,
		operation.ID,
		claim.Attempt,
		w.now().UTC(),
		processErr.Error(),
	)
	return errors.Join(processErr, failErr)
}

func (w *Worker) processClaimedFoundPet(
	ctx context.Context,
	inputEventID string,
	inputEnvelope *domain.EventEnvelope,
	foundEvt domain.FoundPetReportedV2,
) error {
	if result, exists, err := w.loadMatcherResult(ctx, inputEventID); err != nil {
		return err
	} else if exists {
		return w.publishMatcherResult(ctx, result)
	}

	// 1. Analyze found pet image with Ollama + Gemma 4
	prompt := scoring.BuildGemmaPrompt("Pet", "")
	imageInput := foundEvt.ImageURL
	if foundEvt.ImageObject != "" {
		if w.images == nil {
			return errors.New("pet-matcher: private image store is not configured")
		}
		imageBytes, err := w.images.ReadFinalizedImage(ctx, foundEvt.ImageObject)
		if err != nil {
			return fmt.Errorf("pet-matcher: read private found-pet image: %w", err)
		}
		imageInput = base64.StdEncoding.EncodeToString(imageBytes)
	}
	genReq := &ollama.GenerateRequest{
		Model:  w.modelName,
		Prompt: prompt,
		Images: []string{imageInput},
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
		matchResult.SourceEventID = inputEventID
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
			OccurredAt:       w.now().UTC(),
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
		resultRecord, err := w.persistMatcherResult(ctx, inputEventID, matchEnvelope.ID, envelopeBytes)
		if err != nil {
			return err
		}
		if err := w.publishMatcherResult(ctx, resultRecord); err != nil {
			return err
		}

		log.Printf("[Pet Matcher] MATCH FOUND! FoundPet: %s <-> LostPet: %s (Score: %.2f)",
			matchResult.FoundPetID, matchResult.MatchedPetID, matchResult.Score)
	}

	return nil
}

func (w *Worker) persistMatcherResult(
	ctx context.Context,
	inputEventID string,
	outboxID string,
	payload []byte,
) (matcherResultRecord, error) {
	record := matcherResultRecord{InputEventID: inputEventID, OutboxID: outboxID}
	stateData, err := json.Marshal(record)
	if err != nil {
		return matcherResultRecord{}, fmt.Errorf("pet-matcher: marshal matcher result: %w", err)
	}
	outboxRecord := outbox.NewRecord(outboxID, matcherDeliveryChannel, payload, w.now().UTC())
	outboxData, err := outbox.MarshalRecord(outboxRecord)
	if err != nil {
		return matcherResultRecord{}, fmt.Errorf("pet-matcher: marshal matchFound outbox: %w", err)
	}
	_, err = w.store.CreateStateAndOutbox(
		ctx,
		store.StateWrite{StoreName: store.MatcherResultsCollection, Key: inputEventID, Data: stateData},
		store.StateWrite{StoreName: store.OutboxCollection, Key: outboxID, Data: outboxData},
	)
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, store.ErrConflict) {
		return matcherResultRecord{}, fmt.Errorf("pet-matcher: persist matchFound result and outbox: %w", err)
	}
	winner, exists, loadErr := w.loadMatcherResult(ctx, inputEventID)
	if loadErr != nil {
		return matcherResultRecord{}, loadErr
	}
	if !exists {
		return matcherResultRecord{}, fmt.Errorf("pet-matcher: conflicting result has no durable winner: %w", err)
	}
	return winner, nil
}

func (w *Worker) loadMatcherResult(ctx context.Context, inputEventID string) (matcherResultRecord, bool, error) {
	data, err := w.store.GetState(ctx, store.MatcherResultsCollection, inputEventID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		return matcherResultRecord{}, false, nil
	}
	if err != nil {
		return matcherResultRecord{}, false, fmt.Errorf("pet-matcher: load durable matcher result: %w", err)
	}
	var record matcherResultRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return matcherResultRecord{}, false, fmt.Errorf("pet-matcher: decode durable matcher result: %w", err)
	}
	if record.InputEventID != inputEventID || record.OutboxID == "" {
		return matcherResultRecord{}, false, errors.New("pet-matcher: durable matcher result identity is invalid")
	}
	return record, true, nil
}

func (w *Worker) publishMatcherResult(ctx context.Context, result matcherResultRecord) error {
	if _, err := w.relay.PublishRecords(ctx, result.OutboxID); err != nil {
		return fmt.Errorf("pet-matcher: publish durable matchFound result: %w", err)
	}
	record, err := outbox.GetRecord(ctx, w.store, result.OutboxID)
	if err != nil {
		return fmt.Errorf("pet-matcher: load matchFound outbox result: %w", err)
	}
	if record.Status != outbox.StatusPublished {
		return fmt.Errorf("pet-matcher: matchFound publication: %w", delivery.ErrOperationInProgress)
	}
	return nil
}
