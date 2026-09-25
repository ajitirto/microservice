// Package handler contains the HTTP layer for the auth domain.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"auth/internal/middleware"
	"auth/internal/model"
	"auth/internal/service"
)

type Handler struct {
	svc    *service.Service
	logger *slog.Logger
}

func New(svc *service.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) SetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", h.register)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/refresh", h.refresh)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /auth/verify", h.verify)
	mux.HandleFunc("/health", h.health)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in model.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}

	pair, err := h.svc.Register(r.Context(), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, pair)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in model.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}

	pair, err := h.svc.Login(r.Context(), in)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, pair)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var in model.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}

	pair, err := h.svc.Refresh(r.Context(), in.RefreshToken)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, pair)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var in model.RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, errorBody(r, "invalid request body"))
		return
	}

	if err := h.svc.Logout(r.Context(), in.RefreshToken); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if header == "" {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "missing Authorization header"))
		return
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		middleware.WriteJSON(w, http.StatusUnauthorized, errorBody(r, "invalid Authorization header"))
		return
	}

	result, err := h.svc.Verify(r.Context(), strings.TrimPrefix(header, prefix))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrInvalidToken):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrUserAlreadyExists):
		status = http.StatusConflict
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
