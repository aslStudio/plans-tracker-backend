package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"plans-tracker-backend/internal/tasks"
)

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.StatusByID(r.Context(), h.userID, r.PathValue("id"))
	if writeCatalogError(w, err, "status", "reading") {
		return
	}
	writeJSON(w, http.StatusOK, statusToDTO(item))
}

func (h *Handler) createStatus(w http.ResponseWriter, r *http.Request) {
	var request statusRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input, err := request.toCreate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.CreateStatus(r.Context(), h.userID, input)
	if writeCatalogError(w, err, "status", "creating") {
		return
	}
	w.Header().Set("Location", "/v1/task-statuses/"+url.PathEscape(item.ID))
	writeJSON(w, http.StatusCreated, statusToDTO(item))
}

func (h *Handler) patchStatus(w http.ResponseWriter, r *http.Request) {
	var request statusRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.PatchStatus(r.Context(), h.userID, r.PathValue("id"), request.toPatch())
	if writeCatalogError(w, err, "status", "updating") {
		return
	}
	writeJSON(w, http.StatusOK, statusToDTO(item))
}

func (h *Handler) deleteStatus(w http.ResponseWriter, r *http.Request) {
	if writeCatalogError(w, h.service.DeleteStatus(r.Context(), h.userID, r.PathValue("id")), "status", "deleting") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getPriority(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.PriorityByID(r.Context(), h.userID, r.PathValue("id"))
	if writeCatalogError(w, err, "priority", "reading") {
		return
	}
	writeJSON(w, http.StatusOK, priorityToDTO(item))
}

func (h *Handler) createPriority(w http.ResponseWriter, r *http.Request) {
	var request priorityRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input, err := request.toCreate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.CreatePriority(r.Context(), h.userID, input)
	if writeCatalogError(w, err, "priority", "creating") {
		return
	}
	w.Header().Set("Location", "/v1/task-priorities/"+url.PathEscape(item.ID))
	writeJSON(w, http.StatusCreated, priorityToDTO(item))
}

func (h *Handler) patchPriority(w http.ResponseWriter, r *http.Request) {
	var request priorityRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.PatchPriority(r.Context(), h.userID, r.PathValue("id"), request.toPatch())
	if writeCatalogError(w, err, "priority", "updating") {
		return
	}
	writeJSON(w, http.StatusOK, priorityToDTO(item))
}

func (h *Handler) deletePriority(w http.ResponseWriter, r *http.Request) {
	if writeCatalogError(w, h.service.DeletePriority(r.Context(), h.userID, r.PathValue("id")), "priority", "deleting") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getGroup(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GroupByID(r.Context(), h.userID, r.PathValue("id"))
	if writeCatalogError(w, err, "group", "reading") {
		return
	}
	writeJSON(w, http.StatusOK, groupToDTO(item))
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var request groupRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input, err := request.toCreate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.CreateGroup(r.Context(), h.userID, input)
	if writeCatalogError(w, err, "group", "creating") {
		return
	}
	w.Header().Set("Location", "/v1/task-groups/"+url.PathEscape(item.ID))
	writeJSON(w, http.StatusCreated, groupToDTO(item))
}

func (h *Handler) patchGroup(w http.ResponseWriter, r *http.Request) {
	var request groupRequest
	if err := decodeJSONRequest(w, r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.service.PatchGroup(r.Context(), h.userID, r.PathValue("id"), request.toPatch())
	if writeCatalogError(w, err, "group", "updating") {
		return
	}
	writeJSON(w, http.StatusOK, groupToDTO(item))
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	if writeCatalogError(w, h.service.DeleteGroup(r.Context(), h.userID, r.PathValue("id")), "group", "deleting") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeCatalogError(w http.ResponseWriter, err error, resource, action string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, tasks.ErrNotFound) {
		http.Error(w, resource+" not found", http.StatusNotFound)
		return true
	}
	if errors.Is(err, tasks.ErrProtected) {
		http.Error(w, "built-in "+resource+" cannot be deleted", http.StatusConflict)
		return true
	}
	if errors.Is(err, tasks.ErrImmutable) {
		http.Error(w, err.Error(), http.StatusConflict)
		return true
	}
	if errors.Is(err, tasks.ErrInvalidTask) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return true
	}
	http.Error(w, "Something went wrong while "+action+" "+resource, http.StatusInternalServerError)
	return true
}
