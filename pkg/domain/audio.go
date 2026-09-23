package domain

import "time"

// VocalizationType categorizes detected animal acoustic sounds.
type VocalizationType string

const (
	VocalizationCanineBark   VocalizationType = "CANINE_BARK"
	VocalizationFelineMeow   VocalizationType = "FELINE_MEOW"
	VocalizationDistressYowl VocalizationType = "ANIMAL_DISTRESS"
	VocalizationAmbientNoise VocalizationType = "AMBIENT_NOISE"
	VocalizationUnknown      VocalizationType = "UNKNOWN"
)

// AcousticVoiceprint is a 32-dimensional normalized biometric signature.
type AcousticVoiceprint struct {
	VectorDimensions int       `json:"vectorDimensions"` // Always 32
	Features         []float64 `json:"features"`         // 32-dim normalized vector
	DominantPitchHz  float64   `json:"dominantPitchHz"`  // Fundamental frequency F0
	PitchVarianceHz  float64   `json:"pitchVarianceHz"`
	HarmonicRatio    float64   `json:"harmonicRatio"`    // 0.0 - 1.0 (tonality vs noise)
	SpectralCentroid float64   `json:"spectralCentroid"` // Brightness of sound in Hz
	DurationSeconds  float64   `json:"durationSeconds"`
	CreatedAt        time.Time `json:"createdAt"`
}

// AudioProfile stores reference audio for a lost pet or field sighting.
type AudioProfile struct {
	AudioID          string             `json:"audioId"`
	PetID            string             `json:"petId,omitempty"`
	SightingID       string             `json:"sightingId,omitempty"`
	Vocalization     VocalizationType   `json:"vocalization"`
	ConfidenceScore  float64            `json:"confidenceScore"` // 0.0 - 1.0
	Voiceprint       AcousticVoiceprint `json:"voiceprint"`
	SpectrogramBins  [][]float64        `json:"spectrogramBins"` // Compressed 32-time x 32-freq Mel matrix
	AudioDataURI     string             `json:"audioDataUri"`    // base64 data URI (audio/wav or audio/webm)
	OriginalFileName string             `json:"originalFileName,omitempty"`
	RecordedAt       time.Time          `json:"recordedAt"`
}

// AcousticMatchResult represents the bilateral similarity score between reference and field audio.
type AcousticMatchResult struct {
	ReferenceAudioID string           `json:"referenceAudioId"`
	CandidateAudioID string           `json:"candidateAudioId"`
	PetID            string           `json:"petId"`
	SimilarityScore  float64          `json:"similarityScore"` // Cosine similarity: 0.0 - 1.0
	IsProbableMatch  bool             `json:"isProbableMatch"` // True if score >= threshold (default: 0.75)
	Vocalization     VocalizationType `json:"vocalization"`
	MatchedAt        time.Time        `json:"matchedAt"`
}
