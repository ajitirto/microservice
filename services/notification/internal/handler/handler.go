// Package handler contains the HTTP layer for the notification domain.
package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"notification/internal/middleware"
	"notification/internal/service"
)

const userIDHeader = "X-User-ID"

type Handler struct {
	svc    *service.Service
	logger *slog.Logger
}

func New(svc *service.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) SetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /events/{type}", h.ingest)
	mux.HandleFunc("GET /notifications", h.list)
	mux.HandleFunc("GET /notifications/{id}", h.getByID)
	mux.HandleFunc("POST /notifications/{id}/read", h.markRead)
	mux.HandleFunc("/health", h.health)
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}

	n, err := h.svc.Ingest(r.Context(), r.PathValue("type"), payload)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, n)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}

	notifications, err := h.svc.ListByUser(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"notifications": notifications})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}

	n, err := h.svc.GetByID(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, n)
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}

	n, err := h.svc.MarkRead(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, n)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrNotificationNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
	}
	middleware.WriteJSON(w, status, errorBody(r, err.Error()))
}

func errorBody(r *http.Request, message string) map[string]string {
	return map[string]string{
		"error":      message,
		"request_id": r.Header.Get(middleware.RequestIDHeader),
	}
}
