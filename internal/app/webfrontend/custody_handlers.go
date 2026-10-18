package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/custody"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastCustodyNano atomic.Int64
	custodySeedMu   sync.Mutex
)

func nextCustodyNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastCustodyNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastCustodyNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

func getCustodyLedgerSecretKey() []byte {
	if secret := strings.TrimSpace(os.Getenv("PETSPOTR_CUSTODY_LEDGER_SECRET")); secret != "" {
		return []byte(secret)
	}
	return []byte("petspotr-custody-chain-hmac-secret-2026")
}

// CustodyLedgerPageData represents template data passed to templates/custody_ledger.html.
type CustodyLedgerPageData struct {
	Chains []domain.PetCustodyChain
	Locale string
}

type createCustodyChainRequest struct {
	PetID          string              `json:"petId"`
	PetName        string              `json:"petName"`
	Species        string              `json:"species"`
	MicrochipID    string              `json:"microchipId"`
	InitialStage   domain.CustodyStage `json:"initialStage"`
	Stage          domain.CustodyStage `json:"stage"`
	FromCustodian  string              `json:"fromCustodian"`
	ToCustodian    string              `json:"toCustodian"`
	ToRole         string              `json:"toRole"`
	LocationLabel  string              `json:"locationLabel"`
	ConditionNotes string              `json:"conditionNotes"`
}

type appendCustodyTransferRequest struct {
	Stage          domain.CustodyStage `json:"stage"`
	FromCustodian  string              `json:"fromCustodian"`
	ToCustodian    string              `json:"toCustodian"`
	ToRole         string              `json:"toRole"`
	LocationLabel  string              `json:"locationLabel"`
	ConditionNotes string              `json:"conditionNotes"`
}

type completeReunionHandoverRequest struct {
	HandoverPinCode   string                      `json:"handoverPinCode"`
	VerifiedOwnerName string                      `json:"verifiedOwnerName"`
	OwnershipProof    domain.OwnershipProofMethod `json:"ownershipProof"`
	LocationLabel     string                      `json:"locationLabel"`
	WaiverAccepted    bool                        `json:"waiverAccepted"`
}

type custodyChainEnvelope struct {
	domain.PetCustodyChain
	Chain domain.PetCustodyChain `json:"chain"`
}

func defaultCustodyChains(now time.Time, secret []byte) []domain.PetCustodyChain {
	c1 := domain.PetCustodyChain{
		ChainID:         "chain-custody-101",
		PetID:           "pet-reunion-cooper",
		PetName:         "Cooper",
		Species:         "Dog",
		MicrochipID:     "985141009988771",
		HandoverPinCode: custody.GenerateHandoverPIN("chain-custody-101", "pet-reunion-cooper", secret),
		CreatedAt:       now.Add(-30 * time.Minute),
	}
	c1, _ = custody.AppendCustodyHop(c1, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageFinderIntake,
		FromCustodian:  "Field Recovery Point (Discovery Park)",
		ToCustodian:    "Hannah Abbott (Community Finder)",
		ToRole:         "COMMUNITY_FINDER",
		LocationLabel:  "Discovery Park South Trailhead",
		ConditionNotes: "Found dehydrated wearing blue reflective harness",
		TransferredAt:  now.Add(-30 * time.Minute),
	}, secret)
	c1, _ = custody.AppendCustodyHop(c1, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageFieldSAREscort,
		FromCustodian:  "Hannah Abbott (Community Finder)",
		ToCustodian:    "Maya Lin (EAGLE-1)",
		ToRole:         "SAR_FIELD_RESPONDER",
		LocationLabel:  "SAR Mobile Command Base Alpha",
		ConditionNotes: "Microchip scanned 985141009988771; transported in climate-controlled crate",
		TransferredAt:  now.Add(-15 * time.Minute),
	}, secret)
	c1, _ = custody.AppendCustodyHop(c1, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageOwnerEscrow,
		FromCustodian:  "Maya Lin (EAGLE-1)",
		ToCustodian:    "Emerald City 24/7 Veterinary ER Escrow",
		ToRole:         "ESCROW_CUSTODIAN",
		LocationLabel:  "Emerald City ER Reunification Desk",
		ConditionNotes: "Cleared by ER vet; awaiting owner PIN & waiver signature",
		TransferredAt:  now.Add(time.Millisecond),
	}, secret)

	c2 := domain.PetCustodyChain{
		ChainID:         "chain-custody-102",
		PetID:           "pet-reunion-milo",
		PetName:         "Milo",
		Species:         "Cat",
		MicrochipID:     "981020005544332",
		HandoverPinCode: custody.GenerateHandoverPIN("chain-custody-102", "pet-reunion-milo", secret),
		CreatedAt:       now.Add(-45 * time.Minute),
	}
	c2, _ = custody.AppendCustodyHop(c2, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageFinderIntake,
		FromCustodian:  "Ballard Residential Porch",
		ToCustodian:    "Carlos Rivera (K9-2)",
		ToRole:         "SAR_FIELD_RESPONDER",
		LocationLabel:  "Ballard NW 65th St",
		ConditionNotes: "Secured in humane carrier",
		TransferredAt:  now.Add(-45 * time.Minute),
	}, secret)
	c2, _ = custody.AppendCustodyHop(c2, domain.CustodyTransferHop{
		Stage:          domain.CustodyStageVetERHold,
		FromCustodian:  "Carlos Rivera (K9-2)",
		ToCustodian:    "Dr. Sarah Jenkins (VET-3)",
		ToRole:         "VET_ER_CLINICIAN",
		LocationLabel:  "Puget Sound Animal Triage Bay 2",
		ConditionNotes: "Fluid therapy & warming pad stabilization",
		TransferredAt:  now,
	}, secret)

	return []domain.PetCustodyChain{c1, c2}
}

func (s *Server) ensureDefaultCustodyChains(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	custodySeedMu.Lock()
	defer custodySeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionPetCustodyChains)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	secret := getCustodyLedgerSecretKey()
	for _, c := range defaultCustodyChains(now, secret) {
		if data, err := json.Marshal(c); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionPetCustodyChains, c.ChainID, data)
		}
	}
}

func (s *Server) loadCustodyChains(ctx context.Context, petIDFilter, stageFilter string) []domain.PetCustodyChain {
	chains := make([]domain.PetCustodyChain, 0)
	if s.stateStore == nil {
		return chains
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionPetCustodyChains)
	if err != nil {
		return chains
	}
	secret := getCustodyLedgerSecretKey()
	for _, b := range raw {
		var c domain.PetCustodyChain
		if err := json.Unmarshal(b, &c); err == nil {
			if petIDFilter != "" && !strings.EqualFold(c.PetID, petIDFilter) {
				continue
			}
			if stageFilter != "" && !strings.EqualFold(string(c.CurrentStage), stageFilter) {
				continue
			}
			if c.Hops == nil {
				c.Hops = []domain.CustodyTransferHop{}
			}
			intact, _, _ := custody.VerifyCustodyChainIntegrity(c, secret)
			c.ChainVerifiedIntact = intact
			chains = append(chains, c)
		}
	}
	sort.Slice(chains, func(i, j int) bool {
		if !chains[i].UpdatedAt.Equal(chains[j].UpdatedAt) {
			return chains[i].UpdatedAt.After(chains[j].UpdatedAt)
		}
		return chains[i].ChainID < chains[j].ChainID
	})
	return chains
}

func writeCustodyJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleCustodyLedgerPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultCustodyChains(r.Context())
	chains := s.loadCustodyChains(r.Context(), "", "")

	tmpl, err := template.New("custody_ledger.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/custody_ledger.html")
	if err != nil {
		http.Error(w, "Failed to load Custody Ledger template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := CustodyLedgerPageData{
		Chains: chains,
		Locale: locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListCustodyChains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultCustodyChains(r.Context())
	q := r.URL.Query()
	chains := s.loadCustodyChains(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("stage")),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"chains": chains,
	})
}

func (s *Server) handleCreateCustodyChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createCustodyChainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCustodyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultCustodyChains(r.Context())

	petID := strings.TrimSpace(req.PetID)
	petName := strings.TrimSpace(req.PetName)
	if petID == "" || petName == "" {
		writeCustodyJSONError(w, http.StatusBadRequest, "petId and petName are required")
		return
	}

	species := strings.TrimSpace(req.Species)
	if species == "" {
		species = "Dog"
	}

	initialStage := domain.CustodyStage(strings.ToUpper(strings.TrimSpace(string(req.InitialStage))))
	if initialStage == "" {
		initialStage = domain.CustodyStage(strings.ToUpper(strings.TrimSpace(string(req.Stage))))
	}
	if initialStage == "" {
		initialStage = domain.CustodyStageFinderIntake
	}

	fromCustodian := strings.TrimSpace(req.FromCustodian)
	if fromCustodian == "" {
		fromCustodian = "Field Recovery Point"
	}
	toCustodian := strings.TrimSpace(req.ToCustodian)
	toRole := strings.ToUpper(strings.TrimSpace(req.ToRole))
	if toRole == "" {
		toRole = "COMMUNITY_FINDER"
	}
	locationLabel := strings.TrimSpace(req.LocationLabel)

	seqNano := nextCustodyNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)
	chainID := fmt.Sprintf("chain-custody-%d", seqNano)
	secret := getCustodyLedgerSecretKey()

	chain := domain.PetCustodyChain{
		ChainID:         chainID,
		PetID:           petID,
		PetName:         petName,
		Species:         species,
		MicrochipID:     strings.TrimSpace(req.MicrochipID),
		HandoverPinCode: custody.GenerateHandoverPIN(chainID, petID, secret),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	genesisHop := domain.CustodyTransferHop{
		Stage:          initialStage,
		FromCustodian:  fromCustodian,
		ToCustodian:    toCustodian,
		ToRole:         toRole,
		LocationLabel:  locationLabel,
		ConditionNotes: strings.TrimSpace(req.ConditionNotes),
		TransferredAt:  now,
	}

	chain, err := custody.AppendCustodyHop(chain, genesisHop, secret)
	if err != nil {
		writeCustodyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := chain.Validate(); err != nil {
		writeCustodyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	chainBytes, err := json.Marshal(chain)
	if err != nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "Failed to encode custody chain")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionPetCustodyChains, chain.ChainID, chainBytes); err != nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "Failed to save custody chain")
		return
	}

	s.broadcastCustodyEvent("custody_chain_updated", map[string]any{
		"type":  "custody_chain_updated",
		"chain": chain,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(custodyChainEnvelope{
		PetCustodyChain: chain,
		Chain:           chain,
	})
}

func (s *Server) handleAppendCustodyTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	chainID := strings.TrimSpace(r.PathValue("id"))
	if chainID == "" {
		writeCustodyJSONError(w, http.StatusBadRequest, "chain id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req appendCustodyTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCustodyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultCustodyChains(r.Context())

	stage := domain.CustodyStage(strings.ToUpper(strings.TrimSpace(string(req.Stage))))
	if !stage.IsValid() {
		writeCustodyJSONError(w, http.StatusBadRequest, "invalid custody stage")
		return
	}

	seqNano := nextCustodyNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)
	secret := getCustodyLedgerSecretKey()

	var (
		updatedChain  domain.PetCustodyChain
		errAlready    = errors.New("custody chain already reunited")
		errValidation error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionPetCustodyChains, chainID, func(current []byte) ([]byte, error) {
		var c domain.PetCustodyChain
		if err := json.Unmarshal(current, &c); err != nil {
			return nil, fmt.Errorf("corrupt custody chain: %w", err)
		}
		if c.CurrentStage == domain.CustodyStageReunited {
			return nil, errAlready
		}
		fromCustodian := strings.TrimSpace(req.FromCustodian)
		if fromCustodian == "" {
			fromCustodian = c.CurrentCustodian
		}
		toRole := strings.ToUpper(strings.TrimSpace(req.ToRole))
		if toRole == "" {
			toRole = "SAR_CUSTODIAN"
		}
		hop := domain.CustodyTransferHop{
			Stage:          stage,
			FromCustodian:  fromCustodian,
			ToCustodian:    strings.TrimSpace(req.ToCustodian),
			ToRole:         toRole,
			LocationLabel:  strings.TrimSpace(req.LocationLabel),
			ConditionNotes: strings.TrimSpace(req.ConditionNotes),
			TransferredAt:  now,
		}
		nextChain, hopErr := custody.AppendCustodyHop(c, hop, secret)
		if hopErr != nil {
			errValidation = hopErr
			return nil, hopErr
		}
		updatedChain = nextChain
		return json.Marshal(nextChain)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCustodyJSONError(w, http.StatusNotFound, "Custody chain not found")
			return
		}
		if errors.Is(err, errAlready) {
			writeCustodyJSONError(w, http.StatusConflict, "Custody chain has already completed reunion handover")
			return
		}
		if errValidation != nil {
			writeCustodyJSONError(w, http.StatusBadRequest, errValidation.Error())
			return
		}
		writeCustodyJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update custody chain: %v", err))
		return
	}

	s.broadcastCustodyEvent("custody_chain_updated", map[string]any{
		"type":  "custody_chain_updated",
		"chain": updatedChain,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(custodyChainEnvelope{
		PetCustodyChain: updatedChain,
		Chain:           updatedChain,
	})
}

func (s *Server) handleCompleteReunionHandover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	chainID := strings.TrimSpace(r.PathValue("id"))
	if chainID == "" {
		writeCustodyJSONError(w, http.StatusBadRequest, "chain id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req completeReunionHandoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCustodyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultCustodyChains(r.Context())

	seqNano := nextCustodyNano()
	now := time.Unix(0, seqNano).UTC().Truncate(time.Second)
	secret := getCustodyLedgerSecretKey()
	proofMethod := domain.OwnershipProofMethod(strings.ToUpper(strings.TrimSpace(string(req.OwnershipProof))))

	var (
		updatedChain domain.PetCustodyChain
		handoverErr  error
		errAlready   = errors.New("custody chain already reunited")
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionPetCustodyChains, chainID, func(current []byte) ([]byte, error) {
		var c domain.PetCustodyChain
		if err := json.Unmarshal(current, &c); err != nil {
			return nil, fmt.Errorf("corrupt custody chain: %w", err)
		}
		if c.CurrentStage == domain.CustodyStageReunited {
			return nil, errAlready
		}
		nextChain, hErr := custody.VerifyAndCompleteReunionHandover(
			c,
			req.HandoverPinCode,
			req.VerifiedOwnerName,
			req.LocationLabel,
			proofMethod,
			req.WaiverAccepted,
			secret,
			now,
		)
		if hErr != nil {
			handoverErr = hErr
			return nil, hErr
		}
		updatedChain = nextChain
		return json.Marshal(nextChain)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCustodyJSONError(w, http.StatusNotFound, "Custody chain not found")
			return
		}
		if errors.Is(err, custody.ErrInvalidHandoverPIN) || errors.Is(handoverErr, custody.ErrInvalidHandoverPIN) {
			writeCustodyJSONError(w, http.StatusForbidden, "Handover PIN rejected: invalid 6-digit cryptographic escrow PIN — custody release denied")
			return
		}
		if errors.Is(err, errAlready) {
			writeCustodyJSONError(w, http.StatusConflict, "Custody chain is already reunited and released")
			return
		}
		if handoverErr != nil {
			writeCustodyJSONError(w, http.StatusBadRequest, handoverErr.Error())
			return
		}
		writeCustodyJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to complete reunion handover: %v", err))
		return
	}

	s.broadcastCustodyEvent("custody_chain_updated", map[string]any{
		"type":  "custody_chain_updated",
		"chain": updatedChain,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(custodyChainEnvelope{
		PetCustodyChain: updatedChain,
		Chain:           updatedChain,
	})
}

func (s *Server) handleVerifyCustodyChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	chainID := strings.TrimSpace(r.PathValue("id"))
	if chainID == "" {
		writeCustodyJSONError(w, http.StatusBadRequest, "chain id is required")
		return
	}

	s.ensureDefaultCustodyChains(r.Context())

	raw, err := s.stateStore.GetState(r.Context(), store.CollectionPetCustodyChains, chainID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCustodyJSONError(w, http.StatusNotFound, "Custody chain not found")
			return
		}
		writeCustodyJSONError(w, http.StatusInternalServerError, "Failed to load custody chain")
		return
	}

	var chain domain.PetCustodyChain
	if err := json.Unmarshal(raw, &chain); err != nil {
		writeCustodyJSONError(w, http.StatusInternalServerError, "Corrupt custody chain record")
		return
	}

	secret := getCustodyLedgerSecretKey()
	intact, hopCount, msg := custody.VerifyCustodyChainIntegrity(chain, secret)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"chainId":            chain.ChainID,
		"verifiedIntact":     intact,
		"hopCount":           hopCount,
		"latestEntryHashHex": chain.LatestEntryHashHex,
		"message":            msg,
	})
}

func (s *Server) handleCustodyStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	if s.reunionHub == nil {
		return
	}

	subCh, unsubscribe := s.reunionHub.Subscribe("custody")
	defer unsubscribe()

	pingInterval := s.reunionPingInterval
	if pingInterval <= 0 {
		pingInterval = defaultReunionPingInterval
	}
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-subCh:
			if !ok {
				return
			}
			if err := formatSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastCustodyEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextCustodyNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "custody",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "custody",
			SenderNodeID:  "custody-ledger-escrow-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
