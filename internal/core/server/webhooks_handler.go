package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/extensions/webhooks"
)

// WebhooksHandler handles webhook API requests.
type WebhooksHandler struct {
	dispatcher  *webhooks.Dispatcher
	requireAuth func(http.Handler) http.Handler
}

// NewWebhooksHandler creates a new webhooks handler.
func NewWebhooksHandler(dispatcher *webhooks.Dispatcher) *WebhooksHandler {
	return &WebhooksHandler{
		dispatcher: dispatcher,
	}
}

// RegisterRoutes registers webhook API routes.
func (h *WebhooksHandler) RegisterRoutes(mux *http.ServeMux) {
	wrap := h.requireAuth
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	if wrap == nil {
		wrap = func(next http.Handler) http.Handler { return next }
	}
	mux.Handle("GET /v1/webhooks", wrap(http.HandlerFunc(h.handleListWebhooks)))
	mux.Handle("POST /v1/webhooks", wrap(http.HandlerFunc(h.handleRegisterWebhook)))
	mux.Handle("GET /v1/webhooks/{id}", wrap(http.HandlerFunc(h.handleGetWebhook)))
	mux.Handle("PUT /v1/webhooks/{id}", wrap(http.HandlerFunc(h.handleUpdateWebhook)))
	mux.Handle("DELETE /v1/webhooks/{id}", wrap(http.HandlerFunc(h.handleDeleteWebhook)))
	mux.Handle("POST /v1/webhooks/dispatch", wrap(http.HandlerFunc(h.handleDispatch)))
	mux.Handle("GET /v1/webhooks/{id}/deliveries", wrap(http.HandlerFunc(h.handleGetDeliveries)))
	mux.Handle("GET /v1/webhooks/dead-letter", wrap(http.HandlerFunc(h.handleGetDeadLetter)))
	mux.Handle("POST /v1/webhooks/dead-letter/retry", wrap(http.HandlerFunc(h.handleRetryDeadLetter)))
	mux.Handle("GET /v1/webhooks/stats", wrap(http.HandlerFunc(h.handleGetStats)))
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
	if err := strictDecode(r.Body, &wh); err != nil {
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
	if err := strictDecode(r.Body, &wh); err != nil {
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
	if err := strictDecode(r.Body, &event); err != nil {
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
