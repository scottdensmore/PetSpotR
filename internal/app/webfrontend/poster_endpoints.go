package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/qrcode"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type lostPetDTO struct {
	PetID            string               `json:"petId"`
	PetName          string               `json:"petName"`
	Species          string               `json:"species"`
	Breed            string               `json:"breed"`
	PrimaryColor     string               `json:"primaryColor"`
	Description      string               `json:"description"`
	Location         string               `json:"location"`
	LastSeenLocation any                  `json:"lastSeenLocation"`
	ReportedAt       time.Time            `json:"reportedAt"`
	ImageObject      string               `json:"imageObject"`
	ImageURL         string               `json:"imageUrl"`
	Images           []domain.PetImage    `json:"images"`
	Status           domain.LostPetStatus `json:"status"`
}

func (s *Server) getLostPetRecord(ctx context.Context, petID string) (*lostPetDTO, error) {
	if s.stateStore == nil {
		return nil, errors.New("state store uninitialized")
	}
	data, err := s.stateStore.GetState(ctx, store.LostPetsCollection, petID)
	if err != nil {
		return nil, err
	}
	var dto lostPetDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, err
	}
	return &dto, nil
}

func extractLocationString(dto *lostPetDTO) string {
	if loc := strings.TrimSpace(dto.Location); loc != "" {
		return loc
	}
	if dto.LastSeenLocation != nil {
		switch v := dto.LastSeenLocation.(type) {
		case string:
			return strings.TrimSpace(v)
		case map[string]any:
			if addr, ok := v["address"].(string); ok && strings.TrimSpace(addr) != "" {
				return strings.TrimSpace(addr)
			}
		}
	}
	return ""
}

func determineRequestBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
			scheme = xfp
		} else if strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") {
			scheme = "http"
		}
	}
	host := r.Host
	if host == "" {
		host = "petspotr.io"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func (s *Server) handleApiPetQR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	_, err := s.getLostPetRecord(r.Context(), petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load pet record", http.StatusInternalServerError)
		return
	}

	size := 256
	if sVal := r.URL.Query().Get("size"); sVal != "" {
		if parsed, err := strconv.Atoi(sVal); err == nil && parsed > 0 && parsed <= 4096 {
			size = parsed
		}
	}

	margin := 2
	if mVal := r.URL.Query().Get("margin"); mVal != "" {
		if parsed, err := strconv.Atoi(mVal); err == nil && parsed >= 0 && parsed <= 32 {
			margin = parsed
		}
	}

	targetURL := fmt.Sprintf("%s/p/%s", determineRequestBaseURL(r), petID)
	svgBytes, err := qrcode.GenerateSVG(targetURL, size, margin)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(svgBytes)
}

func wrapSVGDescription(desc string, maxChars int) []string {
	words := strings.Fields(desc)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() == 0 {
			cur.WriteString(w)
		} else if cur.Len()+1+len(w) <= maxChars {
			cur.WriteByte(' ')
			cur.WriteString(w)
		} else {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
			if len(lines) >= 3 {
				break
			}
		}
	}
	if cur.Len() > 0 && len(lines) < 3 {
		lines = append(lines, cur.String())
	}
	return lines
}

func (s *Server) handleApiPetShareCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	pet, err := s.getLostPetRecord(r.Context(), petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load pet record", http.StatusInternalServerError)
		return
	}

	petName := strings.TrimSpace(pet.PetName)
	if petName == "" {
		petName = "Missing Pet"
	}

	breed := strings.TrimSpace(pet.Breed)
	species := strings.TrimSpace(pet.Species)
	location := extractLocationString(pet)

	var subtitleParts []string
	if breed != "" {
		subtitleParts = append(subtitleParts, breed)
	} else if species != "" {
		subtitleParts = append(subtitleParts, species)
	}
	if location != "" {
		subtitleParts = append(subtitleParts, location)
	}
	subtitle := strings.Join(subtitleParts, " • ")
	if subtitle == "" {
		subtitle = "PetSpotR Community Alert"
	}

	rewardText := "💰 $500 REWARD"
	if rwd := strings.TrimSpace(r.URL.Query().Get("reward")); rwd != "" {
		if strings.Contains(strings.ToUpper(rwd), "REWARD") {
			rewardText = rwd
		} else {
			rewardText = rwd + " REWARD"
		}
	}

	desc := strings.TrimSpace(pet.Description)
	if desc == "" {
		desc = fmt.Sprintf("Missing pet in %s. Please help bring %s home safely. Tap or scan to view details or report a sighting.", location, petName)
	}

	escapedName := html.EscapeString(petName)
	escapedSubtitle := html.EscapeString(subtitle)
	escapedReward := html.EscapeString(rewardText)

	descLines := wrapSVGDescription(desc, 56)
	var descBuilder strings.Builder
	yOffset := 325
	for _, line := range descLines {
		fmt.Fprintf(&descBuilder, `<text x="510" y="%d" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="20" font-weight="400" fill="#cbd5e1">%s</text>
`, yOffset, html.EscapeString(line))
		yOffset += 28
	}

	imageSrc := strings.TrimSpace(pet.ImageURL)
	if imageSrc == "" {
		if strings.TrimSpace(pet.ImageObject) != "" {
			imageSrc = pet.ImageObject
		} else if len(pet.Images) > 0 && strings.TrimSpace(pet.Images[0].Object) != "" {
			imageSrc = pet.Images[0].Object
		}
	}

	var photoElement string
	if imageSrc != "" {
		photoElement = fmt.Sprintf(`<image href="%s" x="80" y="80" width="380" height="470" preserveAspectRatio="xMidYMid slice" clip-path="url(#photo-clip)"/>`, html.EscapeString(imageSrc))
	} else {
		photoElement = `<g transform="translate(230, 275)" opacity="0.3">
<circle cx="40" cy="40" r="40" fill="#64748b"/>
<path d="M25 40 Q40 15 55 40 Q40 60 25 40 Z" fill="#94a3b8"/>
</g>`
	}

	targetURL := fmt.Sprintf("%s/p/%s", determineRequestBaseURL(r), petID)
	var qrSVGGroup string
	if qrMatrix, err := qrcode.GenerateMatrix(targetURL); err == nil {
		var pathBuilder strings.Builder
		matrixSize := len(qrMatrix)
		moduleSize := 88.0 / float64(matrixSize)
		for y := 0; y < matrixSize; y++ {
			for x := 0; x < matrixSize; {
				if !qrMatrix[y][x] {
					x++
					continue
				}
				start := x
				for x < matrixSize && qrMatrix[y][x] {
					x++
				}
				run := x - start
				fmt.Fprintf(&pathBuilder, "M%.2f,%.2f h%.2f v%.2f h-%.2f z ",
					float64(start)*moduleSize, float64(y)*moduleSize,
					float64(run)*moduleSize, moduleSize, float64(run)*moduleSize)
			}
		}
		qrSVGGroup = fmt.Sprintf(`<g transform="translate(1010, 440)">
<rect width="110" height="110" rx="8" fill="#ffffff"/>
<g transform="translate(11, 11)">
<path d="%s" fill="#0f172a"/>
</g>
</g>`, pathBuilder.String())
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630" width="1200" height="630">
<defs>
<linearGradient id="bg-gradient" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
<stop offset="0%%" stop-color="#0f172a"/>
<stop offset="100%%" stop-color="#1e293b"/>
</linearGradient>
<clipPath id="photo-clip">
<rect x="80" y="80" width="380" height="470" rx="20"/>
</clipPath>
</defs>

<!-- Background -->
<rect width="1200" height="630" fill="url(#bg-gradient)"/>
<rect x="40" y="40" width="1120" height="550" rx="24" fill="#1e293b" fill-opacity="0.6" stroke="#334155" stroke-width="2"/>

<!-- Left Column: Photo -->
<rect x="80" y="80" width="380" height="470" rx="20" fill="#0f172a" stroke="#334155" stroke-width="2"/>
%s

<!-- Photo Urgent LOST Badge -->
<rect x="104" y="104" width="84" height="32" rx="8" fill="#ef4444"/>
<text x="146" y="125" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="14" font-weight="800" fill="#ffffff" text-anchor="middle" letter-spacing="1.5">LOST</text>

<!-- Right Column: Content -->
<!-- Header Pill -->
<rect x="510" y="80" width="170" height="32" rx="16" fill="#ef4444"/>
<text x="595" y="101" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="13" font-weight="800" fill="#ffffff" text-anchor="middle" letter-spacing="1.5">LOST PET ALERT</text>

<!-- Pet Name -->
<text x="510" y="165" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="52" font-weight="800" fill="#ffffff">%s</text>

<!-- Subtitle: Breed • Location -->
<text x="510" y="210" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="24" font-weight="600" fill="#94a3b8">%s</text>

<!-- Reward Badge -->
<g transform="translate(510, 240)">
<rect width="280" height="44" rx="10" fill="#064e3b" stroke="#10b981" stroke-width="1.5"/>
<text x="140" y="28" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="18" font-weight="800" fill="#34d399" text-anchor="middle">%s</text>
</g>

<!-- Description -->
%s

<!-- Footer Info & Shortlink -->
<text x="510" y="475" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="18" font-weight="700" fill="#38bdf8">petspotr.io/p/%s</text>
<text x="510" y="502" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="15" font-weight="500" fill="#94a3b8">Scan QR code or visit shortlink to report sightings directly to owner.</text>

<!-- Brand Footer -->
<g transform="translate(510, 542)">
<circle cx="8" cy="-5" r="7" fill="#f97316"/>
<text x="24" y="0" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="16" font-weight="700" fill="#64748b">PetSpotR <tspan font-weight="400">• Community AI Pet Recovery</tspan></text>
</g>

<!-- Mini QR Code -->
%s
</svg>
`, photoElement, escapedName, escapedSubtitle, escapedReward, descBuilder.String(), html.EscapeString(petID), qrSVGGroup)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(svg))
}
