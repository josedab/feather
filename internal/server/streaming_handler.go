package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/streaming"
)

// StreamingHandler handles streaming API requests.
type StreamingHandler struct {
	hub *streaming.Hub
}

// NewStreamingHandler creates a new streaming handler.
func NewStreamingHandler() *StreamingHandler {
	return &StreamingHandler{
		hub: streaming.NewHub(),
	}
}

// RegisterRoutes registers streaming API routes.
func (h *StreamingHandler) RegisterRoutes(mux *http.ServeMux) {
	// SSE endpoint
	mux.HandleFunc("GET /v1/stream/events", h.handleSSE)

	// Subscription management
	mux.HandleFunc("POST /v1/stream/subscribe", h.handleSubscribe)
	mux.HandleFunc("DELETE /v1/stream/subscribe/{id}", h.handleUnsubscribe)
	mux.HandleFunc("GET /v1/stream/subscriptions", h.handleListSubscriptions)

	// Publishing (for internal use or testing)
	mux.HandleFunc("POST /v1/stream/publish", h.handlePublish)

	// Stats
	mux.HandleFunc("GET /v1/stream/stats", h.handleStats)
}

// GetHub returns the streaming hub for integration.
func (h *StreamingHandler) GetHub() *streaming.Hub {
	return h.hub
}

// handleSSE handles GET /v1/stream/events (Server-Sent Events)
func (h *StreamingHandler) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Parse subscription parameters from query
	features := r.URL.Query()["feature"]
	entities := r.URL.Query()["entity"]
	eventTypes := r.URL.Query()["event_type"]

	var types []streaming.StreamEventType
	for _, et := range eventTypes {
		types = append(types, streaming.StreamEventType(et))
	}

	// Create subscription
	sub := &streaming.Subscription{
		ID:         generateSubscriptionID(),
		ClientID:   r.Header.Get("X-Client-ID"),
		Features:   features,
		EntityIDs:  entities,
		EventTypes: types,
		CreatedAt:  time.Now(),
	}

	events, unsubscribe := h.hub.Subscribe(sub)
	defer unsubscribe()

	// Get flusher for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Send initial connection event
	connEvent := streaming.StreamEvent{
		Type:      "connected",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"subscription_id": sub.ID,
		},
	}
	data, _ := json.Marshal(connEvent)
	w.Write([]byte("event: connected\ndata: " + string(data) + "\n\n"))
	flusher.Flush()

	// Send heartbeat periodically
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			w.Write([]byte("event: heartbeat\ndata: {}\n\n"))
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			w.Write([]byte("event: " + string(event.Type) + "\ndata: " + string(data) + "\n\n"))
			flusher.Flush()
		}
	}
}

// SubscribeRequest represents a subscription request.
type StreamSubscribeRequest struct {
	ClientID   string              `json:"client_id"`
	Features   []string            `json:"features,omitempty"`
	EntityIDs  []string            `json:"entity_ids,omitempty"`
	EventTypes []string            `json:"event_types,omitempty"`
	Filters    map[string]string   `json:"filters,omitempty"`
}

// handleSubscribe handles POST /v1/stream/subscribe
func (h *StreamingHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	var req StreamSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var types []streaming.StreamEventType
	for _, et := range req.EventTypes {
		types = append(types, streaming.StreamEventType(et))
	}

	sub := &streaming.Subscription{
		ID:         generateSubscriptionID(),
		ClientID:   req.ClientID,
		Features:   req.Features,
		EntityIDs:  req.EntityIDs,
		EventTypes: types,
		Filters:    req.Filters,
		CreatedAt:  time.Now(),
	}

	_, _ = h.hub.Subscribe(sub)

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success":         true,
		"subscription_id": sub.ID,
		"message":         "Use SSE endpoint /v1/stream/events with subscription parameters",
	})
}

// handleUnsubscribe handles DELETE /v1/stream/subscribe/{id}
func (h *StreamingHandler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "subscription id required")
		return
	}

	h.hub.Unsubscribe(id)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// handleListSubscriptions handles GET /v1/stream/subscriptions
func (h *StreamingHandler) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs := h.hub.GetSubscriptions()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"subscriptions": subs,
		"count":         len(subs),
	})
}

// StreamPublishRequest represents a publish request.
type StreamPublishRequest struct {
	Type     string                 `json:"type"`
	EntityID string                 `json:"entity_id,omitempty"`
	Feature  string                 `json:"feature,omitempty"`
	Value    interface{}            `json:"value,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// handlePublish handles POST /v1/stream/publish
func (h *StreamingHandler) handlePublish(w http.ResponseWriter, r *http.Request) {
	var req StreamPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Type == "" {
		h.writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	event := streaming.StreamEvent{
		Type:     streaming.StreamEventType(req.Type),
		EntityID: req.EntityID,
		Feature:  req.Feature,
		Value:    req.Value,
		Metadata: req.Metadata,
	}

	h.hub.Publish(event)

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"event":   event,
	})
}

// handleStats handles GET /v1/stream/stats
func (h *StreamingHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.Stats()
	h.writeJSON(w, http.StatusOK, stats)
}

func generateSubscriptionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "sub_" + hex.EncodeToString(bytes)
}

func (h *StreamingHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *StreamingHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
