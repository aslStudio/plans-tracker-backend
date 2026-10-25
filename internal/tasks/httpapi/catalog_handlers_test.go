package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"plans-tracker-backend/internal/tasks"
)

func TestCatalogCRUDRoutes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		id         string
		createBody string
		patchBody  string
		kind       string
	}{
		{"status", "/v1/task-statuses", "status-custom", `{"label":"In progress","color":"accent-blue","is_completed":true}`, `{"label":"Doing","color":null,"is_completed":false}`, "status"},
		{"priority", "/v1/task-priorities", "priority-custom", `{"label":"Urgent","color":"accent-red"}`, `{"label":"Critical","color":null}`, "priority"},
		{"group", "/v1/task-groups", "group-custom", `{"label":"Work","color":"accent-blue"}`, `{"label":"Office","color":"accent-red"}`, "group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &listRepository{
				status:   tasks.TaskStatus{ID: tc.id, Label: "In progress", IsCompleted: true},
				priority: tasks.TaskPriority{ID: tc.id, Label: "Urgent"},
				group:    tasks.TaskGroup{ID: tc.id, Label: "Work", Color: "accent-blue"},
			}
			create := requestAPI(t, repository, http.MethodPost, tc.path, tc.createBody)
			if create.Code != http.StatusCreated || create.Header().Get("Location") != tc.path+"/"+tc.id || repository.catalogCall != "create-"+tc.kind || repository.gotUserID != "dev-user" || repository.gotContext == nil {
				t.Fatalf("create = %d %q, Location = %q, call = %q", create.Code, create.Body.String(), create.Header().Get("Location"), repository.catalogCall)
			}
			var created struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil || created.ID != tc.id {
				t.Errorf("created object = %#v, error = %v", created, err)
			}
			get := requestAPI(t, repository, http.MethodGet, tc.path+"/"+tc.id, "")
			if get.Code != http.StatusOK || repository.catalogCall != "get-"+tc.kind || repository.gotTaskID != tc.id {
				t.Errorf("get = %d %q, call = %q", get.Code, get.Body.String(), repository.catalogCall)
			}
			patch := requestAPI(t, repository, http.MethodPatch, tc.path+"/"+tc.id, tc.patchBody)
			if patch.Code != http.StatusOK || repository.catalogCall != "patch-"+tc.kind {
				t.Errorf("patch = %d %q, call = %q", patch.Code, patch.Body.String(), repository.catalogCall)
			}
			deleted := requestAPI(t, repository, http.MethodDelete, tc.path+"/"+tc.id, "")
			if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 || repository.catalogCall != "delete-"+tc.kind {
				t.Errorf("delete = %d %q, call = %q", deleted.Code, deleted.Body.String(), repository.catalogCall)
			}
			switch tc.kind {
			case "status":
				if !repository.statusInput.IsCompleted || !repository.statusPatch.Color.Set || repository.statusPatch.Color.Value != nil || !repository.statusPatch.IsCompleted.Set || repository.statusPatch.IsCompleted.Value {
					t.Errorf("status input/patch = %#v / %#v", repository.statusInput, repository.statusPatch)
				}
			case "priority":
				if repository.priorityInput.Color == nil || *repository.priorityInput.Color != "accent-red" || !repository.priorityPatch.Color.Set || repository.priorityPatch.Color.Value != nil {
					t.Errorf("priority input/patch = %#v / %#v", repository.priorityInput, repository.priorityPatch)
				}
			case "group":
				if repository.groupInput.Color != "accent-blue" || !repository.groupPatch.Color.Set || repository.groupPatch.Color.Value != "accent-red" {
					t.Errorf("group input/patch = %#v / %#v", repository.groupInput, repository.groupPatch)
				}
			}
		})
	}
}

func TestCatalogCRUDRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodPost, "/v1/task-statuses", `{}`},
		{http.MethodPost, "/v1/task-statuses", `{"label":" ","is_completed":true}`},
		{http.MethodPost, "/v1/task-priorities", `{"label":"Urgent","color":" "}`},
		{http.MethodPost, "/v1/task-groups", `{"label":"Work"}`},
		{http.MethodPatch, "/v1/task-statuses/id", `{}`},
		{http.MethodPatch, "/v1/task-statuses/id", `{"is_completed":null}`},
		{http.MethodPatch, "/v1/task-priorities/id", `{"label":null}`},
		{http.MethodPatch, "/v1/task-groups/id", `{"color":null}`},
		{http.MethodPatch, "/v1/task-groups/id", `{"sort_order":2}`},
	} {
		repository := &listRepository{}
		response := requestAPI(t, repository, tc.method, tc.path, tc.body)
		if response.Code != http.StatusBadRequest || repository.catalogCall != "" {
			t.Errorf("%s %s %s: response = %d, call = %q", tc.method, tc.path, tc.body, response.Code, repository.catalogCall)
		}
	}
}

func TestCatalogCRUDErrors(t *testing.T) {
	for _, path := range []string{"/v1/task-statuses/id", "/v1/task-priorities/id", "/v1/task-groups/id"} {
		response := requestAPI(t, &listRepository{catalogErr: tasks.ErrNotFound}, http.MethodGet, path, "")
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, response.Code)
		}
		response = requestAPI(t, &listRepository{catalogErr: errors.New("private database detail")}, http.MethodGet, path, "")
		if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private database detail") {
			t.Errorf("GET %s = %d %q, want safe 500", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/v1/task-statuses/status-todo", "/v1/task-priorities/priority-high"} {
		response := requestAPI(t, &listRepository{catalogErr: tasks.ErrProtected}, http.MethodDelete, path, "")
		if response.Code != http.StatusConflict {
			t.Errorf("DELETE %s = %d, want 409", path, response.Code)
		}
	}
	response := requestAPI(t, &listRepository{catalogErr: tasks.ErrImmutable}, http.MethodPatch, "/v1/task-statuses/status-todo", `{"is_completed":true}`)
	if response.Code != http.StatusConflict {
		t.Errorf("PATCH built-in completion = %d, want 409", response.Code)
	}
}
