package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"plans-tracker-backend/internal/tasks"
)

func parseRange(r *http.Request) (time.Time, time.Time, error) {
	start, err := time.Parse(time.DateOnly, r.URL.Query().Get("start_date"))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date must use YYYY-MM-DD format")
	}
	end, err := time.Parse(time.DateOnly, r.URL.Query().Get("end_date"))
	if err != nil || end.Before(start) || end.Sub(start) > 365*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("end_date must use YYYY-MM-DD format, follow start_date, and span at most 366 days")
	}
	return start, end, nil
}

func parseOccurrenceDate(r *http.Request) (time.Time, error) {
	date, err := time.Parse(time.DateOnly, r.PathValue("date"))
	if err != nil {
		return time.Time{}, fmt.Errorf("date must use YYYY-MM-DD format")
	}
	return date, nil
}

func occurrenceToDTO(item tasks.TaskOccurrence) TaskOccurrenceDto {
	return TaskOccurrenceDto{
		Task: taskToDTO(item.Task), OccurrenceDate: item.Date.Format(time.DateOnly),
		Status: statusToDTO(item.Status), Completed: item.Completed, CompletedAt: item.CompletedAt,
	}
}

func (h *Handler) listBacklog(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Backlog(r.Context(), h.userIDFor(r))
	if err != nil {
		http.Error(w, "Something went wrong while reading backlog", http.StatusInternalServerError)
		return
	}
	result := make([]TaskDto, len(items))
	for i, item := range items {
		result[i] = taskToDTO(item)
	}
	writeJSON(w, http.StatusOK, TaskListDto{Items: result})
}

func (h *Handler) listOccurrences(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items, err := h.service.ListOccurrences(r.Context(), h.userIDFor(r), start, end)
	if writeOccurrenceError(w, err) {
		return
	}
	result := make([]TaskOccurrenceDto, len(items))
	for i, item := range items {
		result[i] = occurrenceToDTO(item)
	}
	writeJSON(w, http.StatusOK, TaskOccurrenceListDto{Items: result})
}

func (h *Handler) getOccurrence(w http.ResponseWriter, r *http.Request) {
	date, err := parseOccurrenceDate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.Occurrence(r.Context(), h.userIDFor(r), r.PathValue("id"), date)
	if writeOccurrenceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, occurrenceToDTO(item))
}

func (h *Handler) setOccurrenceStatus(w http.ResponseWriter, r *http.Request) {
	date, err := parseOccurrenceDate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var request struct {
		StatusID string `json:"status_id"`
	}
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.SetOccurrenceStatus(r.Context(), h.userIDFor(r), r.PathValue("id"), date, request.StatusID)
	if writeOccurrenceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, occurrenceToDTO(item))
}

func (h *Handler) setOccurrenceCompletion(w http.ResponseWriter, r *http.Request) {
	date, err := parseOccurrenceDate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var request struct {
		Completed *bool `json:"completed"`
	}
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Completed == nil {
		http.Error(w, "completed is required", http.StatusBadRequest)
		return
	}
	item, err := h.service.SetOccurrenceCompletion(r.Context(), h.userIDFor(r), r.PathValue("id"), date, *request.Completed)
	if writeOccurrenceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, occurrenceToDTO(item))
}

func writeOccurrenceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		http.Error(w, "task occurrence not found", http.StatusNotFound)
	case errors.Is(err, tasks.ErrNotRecurring):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, tasks.ErrInvalidTask), errors.Is(err, tasks.ErrInvalidReference):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "Something went wrong while reading or updating task occurrence", http.StatusInternalServerError)
	}
	return true
}
