package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/webhooks"
)

// WebhooksHandler handles webhook API requests.
type WebhooksHandler struct {
	dispatcher *webhooks.Dispatcher
}

// NewWebhooksHandler creates a new webhooks handler.
func NewWebhooksHandler(dispatcher *webhooks.Dispatcher) *WebhooksHandler {
	return &WebhooksHandler{
		dispatcher: dispatcher,
	}
}

// RegisterRoutes registers webhook API routes.
func (h *WebhooksHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/webhooks", h.handleListWebhooks)
	mux.HandleFunc("POST /v1/webhooks", h.handleRegisterWebhook)
	mux.HandleFunc("GET /v1/webhooks/{id}", h.handleGetWebhook)
	mux.HandleFunc("PUT /v1/webhooks/{id}", h.handleUpdateWebhook)
	mux.HandleFunc("DELETE /v1/webhooks/{id}", h.handleDeleteWebhook)
	mux.HandleFunc("POST /v1/webhooks/dispatch", h.handleDispatch)
	mux.HandleFunc("GET /v1/webhooks/{id}/deliveries", h.handleGetDeliveries)
	mux.HandleFunc("GET /v1/webhooks/dead-letter", h.handleGetDeadLetter)
	mux.HandleFunc("POST /v1/webhooks/dead-letter/retry", h.handleRetryDeadLetter)
	mux.HandleFunc("GET /v1/webhooks/stats", h.handleGetStats)
}

// handleListWebhooks handles GET /v1/webhooks
func (h *WebhooksHandler) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	whs := h.dispatcher.ListWebhooks()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"webhooks": whs,
	})
}

// handleRegisterWebhook handles POST /v1/webhooks
func (h *WebhooksHandler) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	var wh webhooks.WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.dispatcher.RegisterWebhook(wh); err != nil {
		if errors.Is(err, webhooks.ErrWebhookExists) {
			h.writeError(r.Context(), w, http.StatusConflict, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, SuccessResponse{Success: true, Message: "webhook registered"})
}

// handleGetWebhook handles GET /v1/webhooks/{id}
func (h *WebhooksHandler) handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "webhook id required")
		return
	}

	wh, err := h.dispatcher.GetWebhook(id)
	if err != nil {
		if errors.Is(err, webhooks.ErrWebhookNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, wh)
}

// handleUpdateWebhook handles PUT /v1/webhooks/{id}
func (h *WebhooksHandler) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "webhook id required")
		return
	}

	var wh webhooks.WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.dispatcher.UpdateWebhook(id, wh); err != nil {
		if errors.Is(err, webhooks.ErrWebhookNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "webhook updated"})
}

// handleDeleteWebhook handles DELETE /v1/webhooks/{id}
func (h *WebhooksHandler) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "webhook id required")
		return
	}

	if err := h.dispatcher.DeleteWebhook(id); err != nil {
		if errors.Is(err, webhooks.ErrWebhookNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "webhook deleted"})
}

// handleDispatch handles POST /v1/webhooks/dispatch
func (h *WebhooksHandler) handleDispatch(w http.ResponseWriter, r *http.Request) {
	var event webhooks.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	results := h.dispatcher.Dispatch(event)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"deliveries": results,
	})
}

// handleGetDeliveries handles GET /v1/webhooks/{id}/deliveries
func (h *WebhooksHandler) handleGetDeliveries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "webhook id required")
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	deliveries := h.dispatcher.GetDeliveries(id, limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"deliveries": deliveries,
	})
}

// handleGetDeadLetter handles GET /v1/webhooks/dead-letter
func (h *WebhooksHandler) handleGetDeadLetter(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events := h.dispatcher.GetDeadLetter(limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
	})
}

// handleRetryDeadLetter handles POST /v1/webhooks/dead-letter/retry
func (h *WebhooksHandler) handleRetryDeadLetter(w http.ResponseWriter, r *http.Request) {
	results := h.dispatcher.RetryDeadLetter()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"deliveries": results,
	})
}

// handleGetStats handles GET /v1/webhooks/stats
func (h *WebhooksHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.dispatcher.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *WebhooksHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *WebhooksHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
