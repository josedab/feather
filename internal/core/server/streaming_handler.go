package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/integrations/streaming"
)

// StreamingHandler handles streaming API requests.
type StreamingHandler struct {
	hub *streaming.Hub
}

// NewStreamingHandler creates a new streaming handler.
func NewStreamingHandler(ctx context.Context) *StreamingHandler {
	return &StreamingHandler{
		hub: streaming.NewHub(ctx),
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

	// Parse subscription parameters from query
	features := r.URL.Query()["feature"]
	entities := r.URL.Query()["entity"]
	eventTypes := r.URL.Query()["event_type"]

	types := make([]streaming.StreamEventType, 0, len(eventTypes))
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
		h.writeError(r.Context(), w, http.StatusInternalServerError, "streaming not supported")
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
	if _, err := w.Write([]byte("event: connected\ndata: " + string(data) + "\n\n")); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
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
			if _, err := w.Write([]byte("event: heartbeat\ndata: {}\n\n")); err != nil {
				h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
				return
			}
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if _, err := w.Write([]byte("event: " + string(event.Type) + "\ndata: " + string(data) + "\n\n")); err != nil {
				h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
				return
			}
			flusher.Flush()
		}
	}
}

// StreamSubscribeRequest represents a subscription request.
type StreamSubscribeRequest struct {
	ClientID   string            `json:"client_id"`
	Features   []string          `json:"features,omitempty"`
	EntityIDs  []string          `json:"entity_ids,omitempty"`
	EventTypes []string          `json:"event_types,omitempty"`
	Filters    map[string]string `json:"filters,omitempty"`
}

// handleSubscribe handles POST /v1/stream/subscribe
func (h *StreamingHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	var req StreamSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	types := make([]streaming.StreamEventType, 0, len(req.EventTypes))
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

	if _, err := h.hub.Subscribe(sub); err != nil {
		slog.Warn("failed to subscribe to streaming hub", "subscription_id", sub.ID, "error", err)
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":         true,
		"subscription_id": sub.ID,
		"message":         "Use SSE endpoint /v1/stream/events with subscription parameters",
	})
}

// handleUnsubscribe handles DELETE /v1/stream/subscribe/{id}
func (h *StreamingHandler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "subscription id required")
		return
	}

	h.hub.Unsubscribe(id)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// handleListSubscriptions handles GET /v1/stream/subscriptions
func (h *StreamingHandler) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs := h.hub.GetSubscriptions()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
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
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Type == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "type is required")
		return
	}

	event := streaming.StreamEvent{
		Type:     streaming.StreamEventType(req.Type),
		EntityID: req.EntityID,
		Feature:  req.Feature,
		Value:    req.Value,
		Metadata: req.Metadata,
	}

	if err := h.hub.Publish(r.Context(), event); err != nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"event":   event,
	})
}

// handleStats handles GET /v1/stream/stats
func (h *StreamingHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

var subCounter atomic.Uint64

func generateSubscriptionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("sub_%d", subCounter.Add(1))
	}
	return "sub_" + hex.EncodeToString(bytes)
}

func (h *StreamingHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *StreamingHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
