// Package handler contains the HTTP layer for the post domain.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"post/internal/middleware"
	"post/internal/model"
	"post/internal/service"
	"post/internal/userresolver"
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
	mux.HandleFunc("GET /posts", h.list)
	mux.HandleFunc("POST /posts", h.create)
	mux.HandleFunc("GET /posts/{id}", h.getByID)
	mux.HandleFunc("PUT /posts/{id}", h.update)
	mux.HandleFunc("DELETE /posts/{id}", h.delete)
	mux.HandleFunc("POST /posts/{id}/like", h.like)
	mux.HandleFunc("/health", h.health)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}

	var in model.CreatePostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}

	post, err := h.svc.Create(r.Context(), userID, in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, post)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	post, err := h.svc.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, post)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	posts, err := h.svc.List(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}

	var in model.UpdatePostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}

	post, err := h.svc.Update(r.Context(), r.PathValue("id"), userID, in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, post)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}

	if err := h.svc.Delete(r.Context(), r.PathValue("id"), userID); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) like(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing X-User-ID header"))
		return
	}

	post, err := h.svc.Like(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, post)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrPostNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, userresolver.ErrUserNotFound):
		status = http.StatusBadRequest
	case errors.Is(err, userresolver.ErrUserUnavailable):
		status = http.StatusServiceUnavailable
	}
	middleware.WriteJSON(w, status, errorBody(r, err.Error()))
}

func errorBody(r *http.Request, message string) map[string]string {
	return map[string]string{
		"error":      message,
		"request_id": r.Header.Get(middleware.RequestIDHeader),
	}
}
