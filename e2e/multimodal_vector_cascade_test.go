package e2e_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/internal/app/petmatcher"
	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/embedding"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMultimodalVectorCascadeAndBackfillJourney(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io/images")
	embedder := embedding.NewMockEmbedder()

	// 1. Subscribe to matchFound event to verify event dispatching
	matchFoundChan := make(chan domain.MatchResult, 1)
	if err := ps.Subscribe("matchFound", func(_ context.Context, data []byte) error {
		var res domain.MatchResult
		if _, err := domain.DecodeEventPayload(data, domain.EventTypeMatchFound, &res); err != nil {
			return err
		}
		matchFoundChan <- res
		return nil
	}); err != nil {
		t.Fatalf("subscribe to matchFound: %v", err)
	}

	// 2. Setup Ollama deterministic client for Gemma 4 visual trait extraction
	deterministicClient := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:      ollama.Gemma4Model,
		Done:       true,
		Response:   `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"Cream","distinctiveMarkings":["White chest patch"],"eyeColor":"Brown"}`,
		Provenance: ollama.DefaultGemma4Provenance(),
	}, nil)

	// 3. Setup PetMatcher Worker with Multimodal Embedder
	matcher := petmatcher.NewWorkerWithImageStore(st, ps, deterministicClient, bs)
	matcher.SetEmbedder(embedder)
	if err := matcher.Start(ctx); err != nil {
		t.Fatalf("start matcher: %v", err)
	}

	// 4. Setup Services
	lostReports := lostpet.NewServiceWithImageStore(st, ps, bs)
	foundReports := foundpet.NewService(st, ps, bs)

	dummyImageBytes := e2eLostPetImage()
	now := time.Now().UTC()
	locationPoint := &domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200}

	imageGrant := beginLostPetImageUpload(t, ctx, lostReports, bs)
	lostPrimaryObj := imageGrant.ObjectName

	// 5. Submit Multi-Photo Lost Pet Report
	embPrimary, _ := embedder.EmbedText(ctx, "Golden Retriever Dog Primary")
	embCoat, _ := embedder.EmbedText(ctx, "Golden Retriever Dog Coat")
	lostCoatObj := "images/lost-pets/" + imageGrant.ReportID + "/coat.png"
	_, _ = bs.UploadImage(ctx, lostCoatObj, dummyImageBytes)
	lostImages := []domain.PetImage{
		{Object: lostPrimaryObj, Tag: domain.PetImageTagPrimary, URL: "https://storage.petspotr.io/images/" + lostPrimaryObj, Embedding: embPrimary},
		{Object: lostCoatObj, Tag: domain.PetImageTagCoat, URL: "https://storage.petspotr.io/images/" + lostCoatObj, Embedding: embCoat},
	}

	lostReport := domain.LostPetReport{
		PetName:         "Rusty",
		Species:         "Dog",
		Breed:           "Golden Retriever",
		PrimaryColor:    "Golden",
		Description:     "White chest patch with red collar",
		ReporterEmail:   "owner@example.com",
		ReportedAt:      now.Add(-2 * time.Hour),
		Location:        "Capitol Hill, Seattle, WA",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     locationPoint,
		Images:          lostImages,
	}

	lostRecorder := reportLostPetWithImage(t, lostReports, imageGrant, lostReport)
	if lostRecorder.Code != 201 {
		t.Fatalf("report lost pet status = %d, want 201; body = %s", lostRecorder.Code, lostRecorder.Body.String())
	}

	// 6. Submit Multi-Photo Found Pet Report
	foundGrant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("begin found image upload: %v", err)
	}
	if _, err := bs.UploadImage(ctx, foundGrant.ObjectName, dummyImageBytes); err != nil {
		t.Fatalf("upload dummy found image: %v", err)
	}
	finalizedFound, err := bs.FinalizeImageForPurpose(ctx, blob.ImagePurposeFoundPet, foundGrant.ReportID, foundGrant.ObjectName, foundGrant.FinalizeToken)
	if err != nil {
		t.Fatalf("finalize found image: %v", err)
	}
	foundPetID := foundGrant.ReportID
	foundPrimaryObj := finalizedFound.ObjectName
	foundCoatObj := "images/found-pets/" + foundPetID + "/coat.png"
	_, _ = bs.UploadImage(ctx, foundCoatObj, dummyImageBytes)

	foundImages := []domain.PetImage{
		{Object: foundPrimaryObj, Tag: domain.PetImageTagPrimary, URL: "https://storage.petspotr.io/images/" + foundPrimaryObj, Embedding: embPrimary},
		{Object: foundCoatObj, Tag: domain.PetImageTagCoat, URL: "https://storage.petspotr.io/images/" + foundCoatObj, Embedding: embCoat},
	}

	foundCmd := foundpet.ReportCommand{
		PetID:               foundPetID,
		ImageObject:         foundPrimaryObj,
		Images:              foundImages,
		FoundAt:             now.Add(-10 * time.Minute),
		Location:            "Capitol Hill, Seattle, WA",
		GeocodingStatus:     domain.GeocodingVerified,
		Coordinates:         locationPoint,
		FinderEmail:         "finder@example.com",
		Species:             "Dog",
		Breed:               "Golden Retriever",
		PrimaryColor:        "Golden",
		SecondaryColor:      "Cream",
		DistinctiveMarkings: []string{"White chest patch"},
		CustodyStatus:       domain.CustodyFinderHome,
	}

	if _, err := foundReports.ReportFoundPet(ctx, foundCmd, foundpet.ReportMetadata{}); err != nil {
		t.Fatalf("report found pet: %v", err)
	}

	// 7. Verify Match Event and Recorded Result
	var receivedMatch domain.MatchResult
	select {
	case receivedMatch = <-matchFoundChan:
		t.Logf("Received matchFound event: MatchID=%s, Score=%.2f", receivedMatch.MatchID, receivedMatch.Score)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for matchFound event from matcher")
	}

	if receivedMatch.Scores.Vector <= 0 {
		t.Errorf("receivedMatch.Scores.Vector = %f, want > 0", receivedMatch.Scores.Vector)
	}
	if receivedMatch.ThresholdVersion != scoring.HybridThresholdVersion {
		t.Errorf("receivedMatch.ThresholdVersion = %q, want %q", receivedMatch.ThresholdVersion, scoring.HybridThresholdVersion)
	}

	// 8. Verify Match Record Stored in Store with Images Preserved
	matchRecords, err := st.ListState(ctx, store.MatchesCollection)
	if err != nil {
		t.Fatalf("list matches: %v", err)
	}
	if len(matchRecords) == 0 {
		t.Fatal("expected at least 1 match record in store")
	}

	var matchRecord domain.MatchRecord
	foundMatchingRecord := false
	for _, raw := range matchRecords {
		var mr domain.MatchRecord
		if err := json.Unmarshal(raw, &mr); err == nil && mr.MatchID == receivedMatch.MatchID {
			matchRecord = mr
			foundMatchingRecord = true
			break
		}
	}
	if !foundMatchingRecord {
		t.Fatalf("match record %s not found in store", receivedMatch.MatchID)
	}

	if matchRecord.Scores.Vector <= 0 {
		t.Errorf("matchRecord.Scores.Vector = %f, want > 0", matchRecord.Scores.Vector)
	}
	if len(matchRecord.FoundPet.Images) < 2 {
		t.Errorf("matchRecord.FoundPet.Images length = %d, want >= 2", len(matchRecord.FoundPet.Images))
	}

	// 9. Verify Legacy Record Vector Backfill
	// Seed legacy lost and found records missing vector embeddings
	legacyLostGrant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeLostPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("begin legacy lost image upload: %v", err)
	}
	if _, err := bs.UploadImage(ctx, legacyLostGrant.ObjectName, dummyImageBytes); err != nil {
		t.Fatalf("upload legacy lost image: %v", err)
	}
	finalizedLegacyLost, err := bs.FinalizeImageForPurpose(ctx, blob.ImagePurposeLostPet, legacyLostGrant.ReportID, legacyLostGrant.ObjectName, legacyLostGrant.FinalizeToken)
	if err != nil {
		t.Fatalf("finalize legacy lost image: %v", err)
	}
	legacyLostID := legacyLostGrant.ReportID
	legacyLostObj := finalizedLegacyLost.ObjectName

	legacyLostRecord := domain.LostPetRecord{
		PetID:           legacyLostID,
		Species:         "Dog",
		Breed:           "Labrador",
		Description:     "Yellow Lab friendly",
		Status:          domain.LostPetStatusLost,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     locationPoint,
		ReportedAt:      now.Add(-24 * time.Hour),
		ImageObject:     legacyLostObj,
		Embedding:       nil, // Missing embedding
	}
	legacyLostBytes, _ := json.Marshal(legacyLostRecord)
	if err := st.SaveState(ctx, store.LostPetsCollection, legacyLostID, legacyLostBytes); err != nil {
		t.Fatalf("seed legacy lost record: %v", err)
	}

	legacyFoundGrant, err := bs.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("begin legacy found image upload: %v", err)
	}
	if _, err := bs.UploadImage(ctx, legacyFoundGrant.ObjectName, dummyImageBytes); err != nil {
		t.Fatalf("upload legacy found image: %v", err)
	}
	finalizedLegacyFound, err := bs.FinalizeImageForPurpose(ctx, blob.ImagePurposeFoundPet, legacyFoundGrant.ReportID, legacyFoundGrant.ObjectName, legacyFoundGrant.FinalizeToken)
	if err != nil {
		t.Fatalf("finalize legacy found image: %v", err)
	}
	legacyFoundID := legacyFoundGrant.ReportID
	legacyFoundObj := finalizedLegacyFound.ObjectName

	legacyFoundRecord := domain.FoundPetRecord{
		PetID:           legacyFoundID,
		Species:         "Dog",
		Breed:           "Labrador",
		Status:          domain.FoundPetStatusFound,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     locationPoint,
		FoundAt:         now.Add(-20 * time.Hour),
		ImageObject:     legacyFoundObj,
		Embedding:       nil, // Missing embedding
	}
	legacyFoundBytes, _ := json.Marshal(legacyFoundRecord)
	if err := st.SaveState(ctx, store.FoundPetsCollection, legacyFoundID, legacyFoundBytes); err != nil {
		t.Fatalf("seed legacy found record: %v", err)
	}

	// Run backfill job
	processed, complete, err := matcher.BackfillEmbeddings(ctx, 10)
	if err != nil {
		t.Fatalf("BackfillEmbeddings() error = %v", err)
	}
	if processed != 2 {
		t.Errorf("BackfillEmbeddings() processed = %d, want 2", processed)
	}
	if !complete {
		t.Errorf("BackfillEmbeddings() complete = %v, want true", complete)
	}

	// Verify legacy lost record now has embedding
	updatedLostData, err := st.GetState(ctx, store.LostPetsCollection, legacyLostID)
	if err != nil {
		t.Fatalf("get backfilled lost record: %v", err)
	}
	var backfilledLost domain.LostPetRecord
	if err := json.Unmarshal(updatedLostData, &backfilledLost); err != nil {
		t.Fatalf("unmarshal backfilled lost record: %v", err)
	}
	if len(backfilledLost.Embedding) != embedder.Dimension() {
		t.Errorf("backfilled lost embedding dimension = %d, want %d", len(backfilledLost.Embedding), embedder.Dimension())
	}

	// Verify legacy found record now has embedding
	updatedFoundData, err := st.GetState(ctx, store.FoundPetsCollection, legacyFoundID)
	if err != nil {
		t.Fatalf("get backfilled found record: %v", err)
	}
	var backfilledFound domain.FoundPetRecord
	if err := json.Unmarshal(updatedFoundData, &backfilledFound); err != nil {
		t.Fatalf("unmarshal backfilled found record: %v", err)
	}
	if len(backfilledFound.Embedding) != embedder.Dimension() {
		t.Errorf("backfilled found embedding dimension = %d, want %d", len(backfilledFound.Embedding), embedder.Dimension())
	}
}
