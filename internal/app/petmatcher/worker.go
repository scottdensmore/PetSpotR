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
	"strings"
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
	store.LostPetCandidateStore
}

type matcherResultRecord struct {
	InputEventID string `json:"inputEventId"`
	OutboxID     string `json:"outboxId"`
	MatchID      string `json:"matchId,omitempty"`
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

// Start registers the foundPet matching and lostPet image-analysis subscriptions.
func (w *Worker) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	broker, ok := w.broker.(pubsub.Broker)
	if !ok {
		return fmt.Errorf("pet-matcher: publisher does not support in-process subscriptions")
	}
	if err := broker.Subscribe("foundPet", func(handlerCtx context.Context, data []byte) error {
		return w.ProcessFoundPet(handlerCtx, data)
	}); err != nil {
		return err
	}
	return broker.Subscribe("lostPet", func(handlerCtx context.Context, data []byte) error {
		return w.ProcessLostPet(handlerCtx, data)
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
	candidates, err := w.eligibleLostPetCandidates(ctx, foundEvt)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
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

	// 2. Score every eligible candidate and choose a deterministic winner.
	var winner *rankedCandidate
	for index := range candidates {
		candidate := candidates[index]
		result := scoring.ComparePetsAtDistance(
			candidate.record.PetID,
			foundEvt.PetID,
			candidate.distanceMiles,
			candidate.traits,
			foundTraits,
		)
		if result == nil || !result.IsMatch {
			continue
		}
		challenger := rankedCandidate{candidate: candidate, result: result}
		if winner == nil || outranks(challenger, *winner) {
			winner = &challenger
		}
	}
	if winner != nil {
		matchResult := winner.result
		lostRecord := winner.candidate.record
		lostTraits := winner.candidate.traits
		lostPetID := lostRecord.PetID
		matchResult.SourceEventID = inputEventID
		matchResult.Model = genResp.Model
		if strings.TrimSpace(matchResult.Model) == "" {
			matchResult.Model = w.modelName
		}
		matchID, err := domain.StableMatchID(inputEventID, foundEvt.PetID, lostPetID)
		if err != nil {
			return fmt.Errorf("pet-matcher: derive match ID: %w", err)
		}
		matchResult.MatchID = matchID
		if matchResult.Scores == nil {
			return errors.New("pet-matcher: scoring result omitted score components")
		}
		matchedAt := w.now().UTC()
		lostBreed := lostRecord.Breed
		if lostBreed == "" {
			lostBreed = lostTraits.Breed
		}
		foundBreed := foundEvt.Breed
		if foundBreed == "" {
			foundBreed = foundTraits.Breed
		}
		matchRecord := domain.MatchRecord{
			MatchID:          matchID,
			FoundPetID:       foundEvt.PetID,
			MatchedPetID:     lostPetID,
			Score:            matchResult.Score,
			Status:           domain.MatchStatusPendingReview,
			MatchedAt:        matchedAt,
			Scores:           *matchResult.Scores,
			SourceEventID:    inputEventID,
			Model:            matchResult.Model,
			ThresholdVersion: matchResult.ThresholdVersion,
			Explanation:      matchResult.Details,
			LostPet: domain.MatchPetDetail{
				PetID:    lostPetID,
				PetName:  lostRecord.PetName,
				Breed:    lostBreed,
				Location: lostRecord.Location,
			},
			FoundPet: domain.MatchPetDetail{
				PetID:    foundEvt.PetID,
				Breed:    foundBreed,
				ImageURL: foundEvt.ImageURL,
				Location: foundEvt.Location,
			},
		}
		if err := matchRecord.Validate(); err != nil {
			return fmt.Errorf("pet-matcher: validate match record: %w", err)
		}
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
			OccurredAt:       matchedAt,
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
		resultRecord, err := w.persistMatcherResult(ctx, inputEventID, matchRecord, matchEnvelope.ID, envelopeBytes)
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
	match domain.MatchRecord,
	outboxID string,
	payload []byte,
) (matcherResultRecord, error) {
	record := matcherResultRecord{InputEventID: inputEventID, OutboxID: outboxID, MatchID: match.MatchID}
	stateData, err := json.Marshal(record)
	if err != nil {
		return matcherResultRecord{}, fmt.Errorf("pet-matcher: marshal matcher result: %w", err)
	}
	outboxRecord := outbox.NewRecord(outboxID, matcherDeliveryChannel, payload, w.now().UTC())
	outboxData, err := outbox.MarshalRecord(outboxRecord)
	if err != nil {
		return matcherResultRecord{}, fmt.Errorf("pet-matcher: marshal matchFound outbox: %w", err)
	}
	matchData, err := json.Marshal(match)
	if err != nil {
		return matcherResultRecord{}, fmt.Errorf("pet-matcher: marshal match record: %w", err)
	}
	_, err = w.store.CreateStatesAndOutbox(
		ctx,
		[]store.StateWrite{
			{StoreName: store.MatcherResultsCollection, Key: inputEventID, Data: stateData},
			{StoreName: store.MatchesCollection, Key: match.MatchID, Data: matchData},
		},
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
