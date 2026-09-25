// Package handler contains the HTTP layer for the user domain.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"user/internal/middleware"
	"user/internal/model"
	"user/internal/service"
)

type Handler struct {
	svc    *service.Service
	logger *slog.Logger
}

func New(svc *service.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) SetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /users/{id}", h.getByID)
	mux.HandleFunc("GET /users/me", h.getMe)
	mux.HandleFunc("PUT /users/{id}", h.update)
	mux.HandleFunc("DELETE /users/{id}", h.delete)
	mux.HandleFunc("/health", h.health)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}
	u, err := h.svc.GetMe(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var upd model.UserUpdate
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}
	u, err := h.svc.Update(r.Context(), r.PathValue("id"), upd)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrInvalidInput):
		status = http.StatusBadRequest
	}
	middleware.WriteJSON(w, status, errorBody(r, err.Error()))
}

func errorBody(r *http.Request, message string) map[string]string {
	return map[string]string{
		"error":      message,
		"request_id": r.Header.Get(middleware.RequestIDHeader),
	}
}
