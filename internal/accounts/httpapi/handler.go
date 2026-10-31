package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"plans-tracker-backend/internal/accounts"
)

type Handler struct {
	Service        accounts.Service
	InternalSecret string
}

func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/challenges", h.challenge)
	mux.HandleFunc("POST /v1/auth/apple", h.signIn)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
	mux.HandleFunc("GET /v1/auth/me", h.me)
	mux.HandleFunc("DELETE /v1/auth/account", h.deleteAccount)
	mux.HandleFunc("POST /v1/auth/apple/notifications", h.appleNotification)
	mux.HandleFunc("POST /internal/auth/introspect", h.introspect)
}

func decode(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return accounts.ErrInvalidInput
	}
	return nil
}
func bearer(r *http.Request) string {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return ""
	}
	return fields[1]
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("auth response encoding failed", "error", err)
	}
}
func fail(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, accounts.ErrInvalidInput):
		http.Error(w, "invalid request", http.StatusBadRequest)
	case errors.Is(err, accounts.ErrUnauthorized), errors.Is(err, accounts.ErrInvalidCredentials):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, accounts.ErrProviderUnavailable), errors.Is(err, accounts.ErrDeletionUnavailable):
		http.Error(w, "dependency unavailable", http.StatusServiceUnavailable)
	default:
		slog.Error("auth request failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	return true
}
func (h Handler) challenge(w http.ResponseWriter, r *http.Request) {
	item, err := h.Service.NewChallenge(r.Context())
	if fail(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) signIn(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IdentityToken     string `json:"identity_token"`
		AuthorizationCode string `json:"authorization_code"`
		Nonce             string `json:"nonce"`
		DisplayName       string `json:"display_name"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, accounts.ErrInvalidInput)
		return
	}
	item, err := h.Service.SignIn(r.Context(), input.IdentityToken, input.AuthorizationCode, input.Nonce, input.DisplayName)
	if fail(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, accounts.ErrInvalidInput)
		return
	}
	item, err := h.Service.Refresh(r.Context(), input.RefreshToken)
	if fail(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	if fail(w, h.Service.Logout(r.Context(), bearer(r))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h Handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.Service.Authenticate(r.Context(), bearer(r), false)
	if fail(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, user)
}
func (h Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	if fail(w, h.Service.DeleteAccount(r.Context(), bearer(r))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h Handler) introspect(w http.ResponseWriter, r *http.Request) {
	provided := r.Header.Get("X-Internal-Auth")
	if h.InternalSecret == "" || len(provided) != len(h.InternalSecret) || subtle.ConstantTimeCompare([]byte(provided), []byte(h.InternalSecret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.Service.Authenticate(r.Context(), bearer(r), false)
	if fail(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, struct {
		UserID string `json:"user_id"`
	}{user.ID})
}

func (h Handler) appleNotification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Payload string `json:"payload"`
	}
	if err := decode(w, r, &input); err != nil || input.Payload == "" {
		fail(w, accounts.ErrInvalidInput)
		return
	}
	if fail(w, h.Service.HandleAppleNotification(r.Context(), input.Payload)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
