package petmatcher

import (
	"context"
	"math"
	"net/http"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/embedding"
)

func l2Normalize(vec []float32) []float32 {
	if len(vec) == 0 {
		return nil
	}
	var sumSq float64
	for _, v := range vec {
		sumSq += float64(v) * float64(v)
	}
	norm := math.Sqrt(sumSq)
	if norm == 0 {
		return append([]float32(nil), vec...)
	}
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = float32(float64(v) / norm)
	}
	return out
}

func averageVectors(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	var dim int
	for _, v := range vectors {
		if len(v) > 0 {
			dim = len(v)
			break
		}
	}
	if dim == 0 {
		return nil
	}
	sum := make([]float64, dim)
	count := 0
	for _, v := range vectors {
		if len(v) != dim {
			continue
		}
		for i := 0; i < dim; i++ {
			sum[i] += float64(v[i])
		}
		count++
	}
	if count == 0 {
		return nil
	}
	avg := make([]float32, dim)
	for i := 0; i < dim; i++ {
		avg[i] = float32(sum[i] / float64(count))
	}
	return l2Normalize(avg)
}

// computeMultiPhotoEmbeddings computes individual embeddings for all images in the list
// and returns the updated images alongside their L2-normalized composite embedding.
func computeMultiPhotoEmbeddings(
	ctx context.Context,
	embedder embedding.Embedder,
	imagesStore blob.ImageStore,
	images []domain.PetImage,
	fallbackObject string,
	cachedBytes []byte,
	description string,
) ([]domain.PetImage, []float32) {
	if embedder == nil {
		return domain.NormalizePetImages(images), nil
	}

	resultImages := domain.NormalizePetImages(images)
	if len(resultImages) == 0 && fallbackObject != "" {
		resultImages = []domain.PetImage{
			{Object: fallbackObject, Tag: domain.PetImageTagPrimary},
		}
	}

	cleanDesc := strings.TrimSpace(description)
	var vectors [][]float32

	for i := range resultImages {
		if len(resultImages[i].Embedding) > 0 {
			vectors = append(vectors, resultImages[i].Embedding)
			continue
		}
		var imgBytes []byte
		if resultImages[i].Object == fallbackObject && len(cachedBytes) > 0 {
			imgBytes = cachedBytes
		} else if imagesStore != nil && resultImages[i].Object != "" {
			if readBytes, err := imagesStore.ReadFinalizedImage(ctx, resultImages[i].Object); err == nil {
				imgBytes = readBytes
			}
		}
		if len(imgBytes) > 0 {
			mimeType := http.DetectContentType(imgBytes)
			if mimeType == "application/octet-stream" || !strings.HasPrefix(mimeType, "image/") {
				mimeType = "image/jpeg"
			}
			if emb, err := embedder.EmbedMultimodal(ctx, imgBytes, mimeType, cleanDesc); err == nil && len(emb) > 0 {
				resultImages[i].Embedding = emb
				vectors = append(vectors, emb)
			}
		}
	}

	composite := averageVectors(vectors)
	return resultImages, composite
}
