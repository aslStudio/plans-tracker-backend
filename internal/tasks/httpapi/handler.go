package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"plans-tracker-backend/internal/auth"
	"plans-tracker-backend/internal/tasks"
)

type Handler struct {
	service *tasks.Service
	userID  string
}

func (h *Handler) userIDFor(r *http.Request) string {
	if id := auth.UserID(r.Context()); id != "" {
		return id
	}
	return h.userID
}

func NewHandler(service *tasks.Service, userID string) *Handler {

	return &Handler{service: service, userID: userID}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/tasks", h.listTasks)
	mux.HandleFunc("GET /v1/tasks/backlog", h.listBacklog)
	mux.HandleFunc("GET /v1/task-occurrences", h.listOccurrences)
	mux.HandleFunc("GET /v1/tasks/{id}/occurrences/{date}", h.getOccurrence)
	mux.HandleFunc("PUT /v1/tasks/{id}/occurrences/{date}/status", h.setOccurrenceStatus)
	mux.HandleFunc("PUT /v1/tasks/{id}/occurrences/{date}/completion", h.setOccurrenceCompletion)
	mux.HandleFunc("POST /v1/tasks", h.createTask)
	mux.HandleFunc("GET /v1/tasks/{id}", h.getTask)
	mux.HandleFunc("PATCH /v1/tasks/{id}", h.patchTask)
	mux.HandleFunc("DELETE /v1/tasks/{id}", h.deleteTask)
	mux.HandleFunc("GET /v1/task-statuses", h.listStatuses)
	mux.HandleFunc("POST /v1/task-statuses", h.createStatus)
	mux.HandleFunc("GET /v1/task-statuses/{id}", h.getStatus)
	mux.HandleFunc("PATCH /v1/task-statuses/{id}", h.patchStatus)
	mux.HandleFunc("DELETE /v1/task-statuses/{id}", h.deleteStatus)
	mux.HandleFunc("GET /v1/task-groups", h.listGroups)
	mux.HandleFunc("POST /v1/task-groups", h.createGroup)
	mux.HandleFunc("GET /v1/task-groups/{id}", h.getGroup)
	mux.HandleFunc("PATCH /v1/task-groups/{id}", h.patchGroup)
	mux.HandleFunc("DELETE /v1/task-groups/{id}", h.deleteGroup)
	mux.HandleFunc("GET /v1/task-priorities", h.listPriorities)
	mux.HandleFunc("POST /v1/task-priorities", h.createPriority)
	mux.HandleFunc("GET /v1/task-priorities/{id}", h.getPriority)
	mux.HandleFunc("PATCH /v1/task-priorities/{id}", h.patchPriority)
	mux.HandleFunc("DELETE /v1/task-priorities/{id}", h.deletePriority)
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	startDate, endDate, err := parseRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := h.service.List(r.Context(), h.userIDFor(r), startDate, endDate)

	if err != nil {
		if errors.Is(err, tasks.ErrInvalidTask) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Something went wrong while reading tasks list", http.StatusInternalServerError)
		return
	}

	resultList := make([]TaskDto, len(items))

	for i, item := range items {
		resultList[i] = taskToDTO(item)
	}

	writeJSON(w, http.StatusOK, TaskListDto{Items: resultList})
}

func (h *Handler) getTask(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.ByID(r.Context(), h.userIDFor(r), r.PathValue("id"))
	if errors.Is(err, tasks.ErrNotFound) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Something went wrong while reading task", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, taskToDTO(item))
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var request createTaskRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input, err := request.toWrite()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.Create(r.Context(), h.userIDFor(r), input)
	if writeTaskError(w, err, "Something went wrong while creating task") {
		return
	}
	w.Header().Set("Location", "/v1/tasks/"+url.PathEscape(item.Task.ID))
	writeJSON(w, http.StatusCreated, taskToDTO(item))
}

func (h *Handler) patchTask(w http.ResponseWriter, r *http.Request) {
	var request patchTaskRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	patch, err := request.toPatch()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.Patch(r.Context(), h.userIDFor(r), r.PathValue("id"), patch)
	if writeTaskError(w, err, "Something went wrong while updating task") {
		return
	}
	writeJSON(w, http.StatusOK, taskToDTO(item))
}

func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	err := h.service.Delete(r.Context(), h.userIDFor(r), r.PathValue("id"))
	if writeTaskError(w, err, "Something went wrong while deleting task") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeTaskError(w http.ResponseWriter, err error, internalMessage string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, tasks.ErrNotFound) {
		http.Error(w, "task not found", http.StatusNotFound)
		return true
	}
	if errors.Is(err, tasks.ErrInvalidTask) || errors.Is(err, tasks.ErrInvalidReference) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return true
	}
	http.Error(w, internalMessage, http.StatusInternalServerError)
	return true
}

func (h *Handler) listStatuses(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.StatusList(r.Context(), h.userIDFor(r))
	if err != nil {
		http.Error(w, "Something went wrong while reading task statuses", http.StatusInternalServerError)
		return
	}
	result := make([]TaskStatusDto, len(items))
	for i, item := range items {
		result[i] = statusToDTO(item)
	}
	writeJSON(w, http.StatusOK, StatusListDto{Items: result})
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.GroupList(r.Context(), h.userIDFor(r))
	if err != nil {
		http.Error(w, "Something went wrong while reading task groups", http.StatusInternalServerError)
		return
	}
	result := make([]TaskGroupDto, len(items))
	for i, item := range items {
		result[i] = groupToDTO(item)
	}
	writeJSON(w, http.StatusOK, GroupListDto{Items: result})
}

func (h *Handler) listPriorities(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.PriorityList(r.Context(), h.userIDFor(r))
	if err != nil {
		http.Error(w, "Something went wrong while reading task priorities", http.StatusInternalServerError)
		return
	}
	result := make([]TaskPriorityDto, len(items))
	for i, item := range items {
		result[i] = priorityToDTO(item)
	}
	writeJSON(w, http.StatusOK, PriorityListDto{Items: result})
}

func taskToDTO(item tasks.TaskDetails) TaskDto {
	var priority *TaskPriorityDto
	if item.Priority != nil {
		value := priorityToDTO(*item.Priority)
		priority = &value
	}
	var group *TaskGroupDto
	if item.Group != nil {
		value := groupToDTO(*item.Group)
		group = &value
	}
	var startMinute *int
	if item.Task.StartMinute != nil {
		minute := int(*item.Task.StartMinute)
		startMinute = &minute
	}
	durationMinutes := int(item.Task.DurationMinutes)
	return TaskDto{
		ID:              item.Task.ID,
		Title:           item.Task.Title,
		Note:            item.Task.Note,
		Description:     item.Task.Description,
		Priority:        priority,
		Group:           group,
		StartDate:       formatTaskDate(item.Task.StartDate),
		EndDate:         formatTaskDate(item.Task.EndDate),
		StartMinute:     startMinute,
		DurationMinutes: &durationMinutes,
		RepeatRule:      item.Task.RepeatRule,
		Status:          statusToDTO(item.Status),
		CreatedAt:       item.Task.CreatedAt,
		UpdatedAt:       item.Task.UpdatedAt,
	}
}

func formatTaskDate(date *time.Time) *string {
	if date == nil {
		return nil
	}
	value := date.Format(time.DateOnly)
	return &value
}

func statusToDTO(item tasks.TaskStatus) TaskStatusDto {
	return TaskStatusDto{
		ID:          item.ID,
		Label:       item.Label,
		Color:       item.Color,
		SortOrder:   item.SortOrder,
		IsCompleted: item.IsCompleted,
		IsBuiltIn:   item.IsBuiltIn,
	}
}

func groupToDTO(item tasks.TaskGroup) TaskGroupDto {
	return TaskGroupDto{
		ID:        item.ID,
		Label:     item.Label,
		Color:     item.Color,
		SortOrder: item.SortOrder,
	}
}

func priorityToDTO(item tasks.TaskPriority) TaskPriorityDto {
	return TaskPriorityDto{
		ID:        item.ID,
		Label:     item.Label,
		Color:     item.Color,
		SortOrder: item.SortOrder,
		IsBuiltIn: item.IsBuiltIn,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
