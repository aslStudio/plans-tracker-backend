package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"plans-tracker-backend/internal/auth"
	"plans-tracker-backend/internal/calendar"
)

type Handler struct {
	Service calendar.Service
	UserID  string
}

func (h Handler) userIDFor(r *http.Request) string {
	if id := auth.UserID(r.Context()); id != "" {
		return id
	}
	return h.UserID
}

func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/calendar-events", h.list)
	mux.HandleFunc("POST /v1/calendar-events", h.create)
	mux.HandleFunc("GET /v1/calendar-events/{id}", h.get)
	mux.HandleFunc("PUT /v1/calendar-events/{id}", h.replace)
	mux.HandleFunc("DELETE /v1/calendar-events/{id}", h.delete)
	mux.HandleFunc("PUT /v1/calendar-events/{id}/excluded-dates/{date}", h.exclude)
	mux.HandleFunc("DELETE /v1/calendar-events/{id}/excluded-dates/{date}", h.restore)
	mux.HandleFunc("GET /v1/calendar-settings", h.settings)
	mux.HandleFunc("PATCH /v1/calendar-settings", h.patchSettings)
}

func decode(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return calendar.ErrInvalid
	}
	return nil
}

func eventInput(w http.ResponseWriter, r *http.Request) (calendar.EventInput, error) {
	var raw map[string]json.RawMessage
	if err := decode(w, r, &raw); err != nil || raw == nil {
		return calendar.EventInput{}, calendar.ErrInvalid
	}
	for _, key := range []string{"title", "date", "start_minute", "duration_minutes", "all_day", "color"} {
		value, ok := raw[key]
		if !ok || string(value) == "null" {
			return calendar.EventInput{}, calendar.ErrInvalid
		}
	}
	data, _ := json.Marshal(raw)
	var input calendar.EventInput
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return calendar.EventInput{}, err
	}
	return input, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("calendar response failed", "error", err)
	}
}
func fail(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, calendar.ErrInvalid), errors.Is(err, calendar.ErrInvalidLink):
		http.Error(w, "invalid request", http.StatusBadRequest)
	case errors.Is(err, calendar.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, calendar.ErrDependency):
		http.Error(w, "dependency unavailable", http.StatusServiceUnavailable)
	default:
		slog.Error("calendar request failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	return true
}
func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	start, err := calendar.ParseDate(r.URL.Query().Get("start_date"))
	if fail(w, err) {
		return
	}
	end, err := calendar.ParseDate(r.URL.Query().Get("end_date"))
	if fail(w, err) {
		return
	}
	items, err := h.Service.List(r.Context(), h.userIDFor(r), start, end)
	if fail(w, err) {
		return
	}
	writeJSON(w, 200, struct {
		Items []calendar.Event `json:"items"`
	}{items})
}
func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.Service.Repository.ByID(r.Context(), h.userIDFor(r), r.PathValue("id"))
	if fail(w, err) {
		return
	}
	writeJSON(w, 200, item)
}
func (h Handler) create(w http.ResponseWriter, r *http.Request) {
	input, err := eventInput(w, r)
	if fail(w, err) {
		return
	}
	item, err := h.Service.Create(r.Context(), h.userIDFor(r), input)
	if fail(w, err) {
		return
	}
	w.Header().Set("Location", "/v1/calendar-events/"+url.PathEscape(item.ID))
	writeJSON(w, 201, item)
}
func (h Handler) replace(w http.ResponseWriter, r *http.Request) {
	input, err := eventInput(w, r)
	if fail(w, err) {
		return
	}
	item, err := h.Service.Replace(r.Context(), h.userIDFor(r), r.PathValue("id"), input)
	if fail(w, err) {
		return
	}
	writeJSON(w, 200, item)
}
func (h Handler) delete(w http.ResponseWriter, r *http.Request) {
	if fail(w, h.Service.Repository.Delete(r.Context(), h.userIDFor(r), r.PathValue("id"))) {
		return
	}
	w.WriteHeader(204)
}
func (h Handler) exclude(w http.ResponseWriter, r *http.Request) { h.setExcluded(w, r, true) }
func (h Handler) restore(w http.ResponseWriter, r *http.Request) { h.setExcluded(w, r, false) }
func (h Handler) setExcluded(w http.ResponseWriter, r *http.Request, excluded bool) {
	item, err := h.Service.Exclude(r.Context(), h.userIDFor(r), r.PathValue("id"), r.PathValue("date"), excluded)
	if fail(w, err) {
		return
	}
	writeJSON(w, 200, item)
}
func (h Handler) settings(w http.ResponseWriter, r *http.Request) {
	item, err := h.Service.Repository.Settings(r.Context(), h.userIDFor(r))
	if fail(w, err) {
		return
	}
	writeJSON(w, 200, item)
}
func (h Handler) patchSettings(w http.ResponseWriter, r *http.Request) {
	var raw map[string]json.RawMessage
	if err := decode(w, r, &raw); err != nil || len(raw) == 0 {
		fail(w, calendar.ErrInvalid)
		return
	}
	for _, value := range raw {
		if string(value) == "null" {
			fail(w, calendar.ErrInvalid)
			return
		}
	}
	data, _ := json.Marshal(raw)
	var patch calendar.SettingsPatch
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&patch); err != nil || (patch.ShowTasks == nil && patch.ShowHabits == nil && patch.ViewMode == nil) {
		fail(w, calendar.ErrInvalid)
		return
	}
	if patch.ViewMode != nil && *patch.ViewMode != "day" && *patch.ViewMode != "3day" && *patch.ViewMode != "week" && *patch.ViewMode != "month" {
		fail(w, calendar.ErrInvalid)
		return
	}
	item, err := h.Service.Repository.PatchSettings(r.Context(), h.userIDFor(r), patch)
	if fail(w, err) {
		return
	}
	writeJSON(w, 200, item)
}
