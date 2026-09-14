package petmatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// BackfillEmbeddings queries for missing embeddings across active lost and found records
// in batches, computes the missing vector data via Ollama/embedder, and updates the durable store.
func (w *Worker) BackfillEmbeddings(ctx context.Context, batchSize int) (processed int, complete bool, err error) {
	if batchSize < 1 {
		batchSize = 50
	}

	if w.embedder == nil {
		return 0, false, errors.New("petmatcher: embedder is required for backfill")
	}

	collections := []string{store.LostPetsCollection, store.FoundPetsCollection}
	totalProcessed := 0
	anyRemaining := false

	for _, collection := range collections {
		cursor := ""
		for {
			dataMap, nextCursor, err := w.store.ScanActiveReportsMissingEmbeddings(ctx, collection, batchSize, cursor)
			if err != nil {
				return totalProcessed, false, fmt.Errorf("petmatcher: scan missing embeddings for %s: %w", collection, err)
			}

			if len(dataMap) == 0 {
				break
			}

			for key, data := range dataMap {
				if err := ctx.Err(); err != nil {
					return totalProcessed, false, err
				}

				if collection == store.LostPetsCollection {
					var record domain.LostPetRecord
					if err := json.Unmarshal(data, &record); err != nil {
						continue
					}
					if len(record.Embedding) > 0 {
						continue
					}

					textDesc := strings.TrimSpace(record.Species + " " + record.Breed)
					if record.Description != "" {
						if textDesc != "" {
							textDesc += " " + strings.TrimSpace(record.Description)
						} else {
							textDesc = strings.TrimSpace(record.Description)
						}
					}

					imagesToProcess := record.Images
					objectName := record.ImageObject
					if objectName == "" {
						if primary, ok := domain.PrimaryPetImage(imagesToProcess); ok {
							objectName = primary.Object
						}
					}

					processedImages, composite := computeMultiPhotoEmbeddings(ctx, w.embedder, w.images, imagesToProcess, objectName, nil, textDesc)
					if len(composite) == 0 {
						continue
					}

					_ = w.store.UpdateState(ctx, collection, key, func(current []byte) ([]byte, error) {
						var updated domain.LostPetRecord
						if err := json.Unmarshal(current, &updated); err != nil {
							return nil, err
						}
						if len(updated.Embedding) > 0 {
							return current, nil
						}
						updated.Embedding = append([]float32(nil), composite...)
						if len(processedImages) > 0 {
							updated.Images = processedImages
						}
						return json.Marshal(updated)
					})
					totalProcessed++

				} else {
					var record domain.FoundPetRecord
					if err := json.Unmarshal(data, &record); err != nil {
						continue
					}
					if len(record.Embedding) > 0 {
						continue
					}

					textDesc := strings.TrimSpace(record.Species + " " + record.Breed)

					imagesToProcess := record.Images
					objectName := record.ImageObject
					if objectName == "" {
						if primary, ok := domain.PrimaryPetImage(imagesToProcess); ok {
							objectName = primary.Object
						}
					}

					processedImages, composite := computeMultiPhotoEmbeddings(ctx, w.embedder, w.images, imagesToProcess, objectName, nil, textDesc)
					if len(composite) == 0 {
						continue
					}

					_ = w.store.UpdateState(ctx, collection, key, func(current []byte) ([]byte, error) {
						var updated domain.FoundPetRecord
						if err := json.Unmarshal(current, &updated); err != nil {
							return nil, err
						}
						if len(updated.Embedding) > 0 {
							return current, nil
						}
						updated.Embedding = append([]float32(nil), composite...)
						if len(processedImages) > 0 {
							updated.Images = processedImages
						}
						return json.Marshal(updated)
					})
					totalProcessed++
				}
			}

			if nextCursor == "" || len(dataMap) < batchSize {
				break
			}
			cursor = nextCursor
			anyRemaining = true
			if totalProcessed >= batchSize {
				return totalProcessed, false, nil
			}
		}
	}

	return totalProcessed, !anyRemaining, nil
}
