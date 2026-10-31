package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"plans-tracker-backend/internal/habits"
)

type Handler struct {
	service *habits.Service
	userID  string
}

func NewHandler(service *habits.Service, userID string) *Handler {
	return &Handler{service: service, userID: userID}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/habits", h.listHabits)
	mux.HandleFunc("POST /v1/habits", h.createHabit)
	mux.HandleFunc("GET /v1/habits/{id}", h.getHabit)
	mux.HandleFunc("PATCH /v1/habits/{id}", h.patchHabit)
	mux.HandleFunc("DELETE /v1/habits/{id}", h.deleteHabit)
	mux.HandleFunc("GET /v1/habit-categories", h.listCategories)
	mux.HandleFunc("POST /v1/habit-categories", h.createCategory)
	mux.HandleFunc("GET /v1/habit-categories/{id}", h.getCategory)
	mux.HandleFunc("PATCH /v1/habit-categories/{id}", h.patchCategory)
	mux.HandleFunc("DELETE /v1/habit-categories/{id}", h.deleteCategory)
	mux.HandleFunc("GET /v1/habit-completions", h.listCompletions)
	mux.HandleFunc("PUT /v1/habits/{id}/completions/{date}", h.setCompletion)
}

func (h *Handler) listHabits(w http.ResponseWriter, r *http.Request) {
	listed, err := h.service.List(r.Context(), h.userID)
	if writeServiceError(w, err) {
		return
	}
	items := make([]habitDTO, len(listed))
	for i, item := range listed {
		items[i] = habitToDTO(item)
	}
	writeJSON(w, http.StatusOK, habitListDTO{Items: items})
}

func (h *Handler) getHabit(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.ByID(r.Context(), h.userID, r.PathValue("id"))
	if writeServiceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, habitToDTO(item))
}

func (h *Handler) createHabit(w http.ResponseWriter, r *http.Request) {
	var request habitRequest
	if err := decodeRequest(w, r, &request); err != nil {
		badRequest(w)
		return
	}
	input, err := request.toCreateInput()
	if err != nil {
		badRequest(w)
		return
	}
	item, err := h.service.Create(r.Context(), h.userID, input)
	if writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/v1/habits/"+url.PathEscape(item.Habit.ID))
	writeJSON(w, http.StatusCreated, habitToDTO(item))
}

func (h *Handler) patchHabit(w http.ResponseWriter, r *http.Request) {
	var request habitRequest
	if err := decodeRequest(w, r, &request); err != nil {
		badRequest(w)
		return
	}
	item, err := h.service.Patch(r.Context(), h.userID, r.PathValue("id"), request.toPatch())
	if writeServiceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, habitToDTO(item))
}

func (h *Handler) deleteHabit(w http.ResponseWriter, r *http.Request) {
	if writeServiceError(w, h.service.Delete(r.Context(), h.userID, r.PathValue("id"))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	listed, err := h.service.CategoryList(r.Context(), h.userID)
	if writeServiceError(w, err) {
		return
	}
	items := make([]categoryDTO, len(listed))
	for i, item := range listed {
		items[i] = categoryToDTO(item)
	}
	writeJSON(w, http.StatusOK, categoryListDTO{Items: items})
}

func (h *Handler) getCategory(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.CategoryByID(r.Context(), h.userID, r.PathValue("id"))
	if writeServiceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, categoryToDTO(item))
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var request categoryRequest
	if err := decodeRequest(w, r, &request); err != nil {
		badRequest(w)
		return
	}
	input, err := request.toCreateInput()
	if err != nil {
		badRequest(w)
		return
	}
	item, err := h.service.CreateCategory(r.Context(), h.userID, input)
	if writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/v1/habit-categories/"+url.PathEscape(item.ID))
	writeJSON(w, http.StatusCreated, categoryToDTO(item))
}

func (h *Handler) patchCategory(w http.ResponseWriter, r *http.Request) {
	var request categoryRequest
	if err := decodeRequest(w, r, &request); err != nil {
		badRequest(w)
		return
	}
	item, err := h.service.PatchCategory(r.Context(), h.userID, r.PathValue("id"), request.toPatch())
	if writeServiceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, categoryToDTO(item))
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	if writeServiceError(w, h.service.DeleteCategory(r.Context(), h.userID, r.PathValue("id"))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listCompletions(w http.ResponseWriter, r *http.Request) {
	filter := habits.CompletionFilter{HabitID: r.URL.Query().Get("habit_id")}
	if start := r.URL.Query().Get("start_date"); start != "" {
		date, err := parseDate(start)
		if err != nil {
			badRequest(w)
			return
		}
		filter.StartDate = &date
	}
	if end := r.URL.Query().Get("end_date"); end != "" {
		date, err := parseDate(end)
		if err != nil {
			badRequest(w)
			return
		}
		filter.EndDate = &date
	}
	listed, err := h.service.CompletionList(r.Context(), h.userID, filter)
	if writeServiceError(w, err) {
		return
	}
	items := make([]completionDTO, len(listed))
	for i, item := range listed {
		items[i] = completionToDTO(item)
	}
	writeJSON(w, http.StatusOK, completionListDTO{Items: items})
}

func (h *Handler) setCompletion(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.PathValue("date"))
	if err != nil {
		badRequest(w)
		return
	}
	var request completionRequest
	if err := decodeRequest(w, r, &request); err != nil || !request.Completed.Set || request.Completed.Null {
		badRequest(w)
		return
	}
	item, err := h.service.SetCompletion(r.Context(), h.userID, r.PathValue("id"), date, request.Completed.Value)
	if writeServiceError(w, err) {
		return
	}
	state := completionStateDTO{Completed: item != nil}
	if item != nil {
		completion := completionToDTO(*item)
		state.Completion = &completion
	}
	writeJSON(w, http.StatusOK, state)
}

func parseDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value {
		return time.Time{}, habits.ErrInvalidInput
	}
	return date, nil
}

func badRequest(w http.ResponseWriter) {
	http.Error(w, "invalid request", http.StatusBadRequest)
}

func writeServiceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, habits.ErrInvalidInput), errors.Is(err, habits.ErrInvalidReference):
		badRequest(w)
	case errors.Is(err, habits.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		slog.Error("habits request failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to write habits response", "error", err)
	}
}
