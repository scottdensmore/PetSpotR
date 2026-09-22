package webfrontend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/qrcode"
)

const (
	defaultMeshPingInterval = 15 * time.Second
	maxMeshBodyBytes        = 10485760 // 10MB to accommodate compressed field images/payloads
)

// handleApiMeshSignalEvents serves the real-time SSE stream relaying WebRTC signaling envelopes to mesh peers.
func (s *Server) handleApiMeshSignalEvents(w http.ResponseWriter, r *http.Request) {
	setMeshCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	searchPartyID := strings.TrimSpace(r.URL.Query().Get("searchPartyId"))
	if searchPartyID == "" {
		searchPartyID = strings.TrimSpace(r.URL.Query().Get("partyId"))
	}
	nodeID := strings.TrimSpace(r.URL.Query().Get("nodeId"))

	if searchPartyID == "" {
		http.Error(w, "searchPartyId query parameter is required", http.StatusBadRequest)
		return
	}
	if nodeID == "" {
		http.Error(w, "nodeId query parameter is required", http.StatusBadRequest)
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

	if s.signalingHub == nil {
		return
	}

	subCh, unsubscribe := s.signalingHub.Subscribe(searchPartyID, nodeID)
	defer unsubscribe()

	ticker := time.NewTicker(defaultMeshPingInterval)
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
		case envelope, ok := <-subCh:
			if !ok {
				return
			}
			dataBytes, err := json.Marshal(envelope)
			if err != nil {
				continue
			}
			eventID := fmt.Sprintf("%d", time.Now().UnixNano())
			_, _ = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", eventID, envelope.Type, dataBytes)
			flusher.Flush()
		}
	}
}

// handleApiMeshSignalMessage ingests a signaling envelope from a peer and relays it to recipient peers.
func (s *Server) handleApiMeshSignalMessage(w http.ResponseWriter, r *http.Request) {
	setMeshCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxMeshBodyBytes)
	var envelope mesh.SignalingEnvelope
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if envelope.SearchPartyID == "" {
		envelope.SearchPartyID = strings.TrimSpace(r.URL.Query().Get("searchPartyId"))
	}
	if envelope.SenderNodeID == "" {
		envelope.SenderNodeID = strings.TrimSpace(r.URL.Query().Get("nodeId"))
	}

	if envelope.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}
	if envelope.SearchPartyID == "" {
		http.Error(w, "searchPartyId is required", http.StatusBadRequest)
		return
	}
	if envelope.SenderNodeID == "" {
		http.Error(w, "senderNodeId is required", http.StatusBadRequest)
		return
	}

	if s.signalingHub != nil {
		s.signalingHub.Relay(envelope)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleApiMeshUplinkSync processes WAN cloud uplink reconciliation requests from off-grid mesh peers.
func (s *Server) handleApiMeshUplinkSync(w http.ResponseWriter, r *http.Request) {
	setMeshCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxMeshBodyBytes)
	var batch domain.MeshBatchDelta
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	var opts []mesh.UplinkOption
	if s.reunionHub != nil {
		opts = append(opts, mesh.WithBroadcaster(s.reunionHub))
	}

	resp, err := mesh.ReconcileUplinkBatch(r.Context(), s.stateStore, batch, opts...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to reconcile mesh batch: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleApiMeshQRSignaling generates a pure Go SVG QR code for optical WebRTC signaling payloads.
func (s *Server) handleApiMeshQRSignaling(w http.ResponseWriter, r *http.Request) {
	setMeshCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := strings.TrimSpace(r.URL.Query().Get("data"))
	if data == "" {
		http.Error(w, "data parameter is required", http.StatusBadRequest)
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

	svgBytes, err := qrcode.GenerateSVG(data, size, margin)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate QR SVG: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(svgBytes)
	}
}

func setMeshCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Idempotency-Key")
}
