package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plans-tracker-backend/internal/tasks"
)

type listRepository struct {
	items         []tasks.TaskDetails
	err           error
	item          tasks.TaskDetails
	byIDErr       error
	statuses      []tasks.TaskStatus
	groups        []tasks.TaskGroup
	priorities    []tasks.TaskPriority
	status        tasks.TaskStatus
	priority      tasks.TaskPriority
	group         tasks.TaskGroup
	statusInput   tasks.StatusInput
	statusPatch   tasks.StatusPatch
	priorityInput tasks.PriorityInput
	priorityPatch tasks.PriorityPatch
	groupInput    tasks.GroupInput
	groupPatch    tasks.GroupPatch
	catalogErr    error
	catalogCall   string
	lookupErr     error
	lookupCalled  string
	createInput   tasks.TaskWrite
	createCalled  bool
	createErr     error
	patchInput    tasks.TaskPatch
	patchCalled   bool
	patchErr      error
	deleteCalled  bool
	deleteErr     error
	gotTaskID     string
	gotUserID     string
	gotContext    context.Context
	gotStartDate  time.Time
	gotEndDate    time.Time
	states        map[tasks.OccurrenceKey]tasks.OccurrenceState
	setStatusID   string
	setCompleted  *bool
}

func (r *listRepository) Backlog(ctx context.Context, userID string) ([]tasks.TaskDetails, error) {
	r.gotContext, r.gotUserID = ctx, userID
	return r.items, r.err
}

func (r *listRepository) OccurrenceStates(ctx context.Context, userID string, start, end time.Time) (map[tasks.OccurrenceKey]tasks.OccurrenceState, error) {
	return r.states, nil
}

func (r *listRepository) SetOccurrenceStatus(ctx context.Context, userID, taskID string, date time.Time, statusID string) error {
	r.setStatusID = statusID
	if r.states == nil {
		r.states = make(map[tasks.OccurrenceKey]tasks.OccurrenceState)
	}
	r.states[tasks.OccurrenceKey{TaskID: taskID, Date: date.Format(time.DateOnly)}] = tasks.OccurrenceState{StatusID: statusID}
	return nil
}

func (r *listRepository) SetOccurrenceCompletion(ctx context.Context, userID, taskID string, date time.Time, completed bool) error {
	r.setCompleted = &completed
	if r.states == nil {
		r.states = make(map[tasks.OccurrenceKey]tasks.OccurrenceState)
	}
	key := tasks.OccurrenceKey{TaskID: taskID, Date: date.Format(time.DateOnly)}
	state := r.states[key]
	if completed {
		now := time.Now()
		state.CompletedAt = &now
	} else {
		state.CompletedAt = nil
	}
	r.states[key] = state
	return nil
}

func (r *listRepository) List(ctx context.Context, userID string, startDate, endDate time.Time) ([]tasks.TaskDetails, error) {
	r.gotContext = ctx
	r.gotUserID = userID
	r.gotStartDate = startDate
	r.gotEndDate = endDate
	return r.items, r.err
}

func (r *listRepository) ByID(ctx context.Context, userID, id string) (tasks.TaskDetails, error) {
	r.gotContext = ctx
	r.gotUserID = userID
	r.gotTaskID = id
	return r.item, r.byIDErr
}

func (r *listRepository) StatusList(ctx context.Context, userID string) ([]tasks.TaskStatus, error) {
	r.gotContext = ctx
	r.gotUserID = userID
	r.lookupCalled = "statuses"
	return r.statuses, r.lookupErr
}

func (r *listRepository) GroupList(ctx context.Context, userID string) ([]tasks.TaskGroup, error) {
	r.gotContext = ctx
	r.gotUserID = userID
	r.lookupCalled = "groups"
	return r.groups, r.lookupErr
}

func (r *listRepository) PriorityList(ctx context.Context, userID string) ([]tasks.TaskPriority, error) {
	r.gotContext = ctx
	r.gotUserID = userID
	r.lookupCalled = "priorities"
	return r.priorities, r.lookupErr
}

func (r *listRepository) StatusById(ctx context.Context, userID, id string) (tasks.TaskStatus, error) {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "get-status"
	return r.status, r.catalogErr
}

func (r *listRepository) CreateStatus(ctx context.Context, userID string, input tasks.StatusInput) (tasks.TaskStatus, error) {
	r.gotContext, r.gotUserID, r.catalogCall = ctx, userID, "create-status"
	r.statusInput = input
	return r.status, r.catalogErr
}

func (r *listRepository) PatchStatus(ctx context.Context, userID, id string, patch tasks.StatusPatch) (tasks.TaskStatus, error) {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "patch-status"
	r.statusPatch = patch
	return r.status, r.catalogErr
}

func (r *listRepository) DeleteStatus(ctx context.Context, userID, id string) error {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "delete-status"
	return r.catalogErr
}

func (r *listRepository) PriorityById(ctx context.Context, userID, id string) (tasks.TaskPriority, error) {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "get-priority"
	return r.priority, r.catalogErr
}

func (r *listRepository) CreatePriority(ctx context.Context, userID string, input tasks.PriorityInput) (tasks.TaskPriority, error) {
	r.gotContext, r.gotUserID, r.catalogCall = ctx, userID, "create-priority"
	r.priorityInput = input
	return r.priority, r.catalogErr
}

func (r *listRepository) PatchPriority(ctx context.Context, userID, id string, patch tasks.PriorityPatch) (tasks.TaskPriority, error) {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "patch-priority"
	r.priorityPatch = patch
	return r.priority, r.catalogErr
}

func (r *listRepository) DeletePriority(ctx context.Context, userID, id string) error {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "delete-priority"
	return r.catalogErr
}

func (r *listRepository) GroupById(ctx context.Context, userID, id string) (tasks.TaskGroup, error) {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "get-group"
	return r.group, r.catalogErr
}

func (r *listRepository) CreateGroup(ctx context.Context, userID string, input tasks.GroupInput) (tasks.TaskGroup, error) {
	r.gotContext, r.gotUserID, r.catalogCall = ctx, userID, "create-group"
	r.groupInput = input
	return r.group, r.catalogErr
}

func (r *listRepository) PatchGroup(ctx context.Context, userID, id string, patch tasks.GroupPatch) (tasks.TaskGroup, error) {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "patch-group"
	r.groupPatch = patch
	return r.group, r.catalogErr
}

func (r *listRepository) DeleteGroup(ctx context.Context, userID, id string) error {
	r.gotContext, r.gotUserID, r.gotTaskID, r.catalogCall = ctx, userID, id, "delete-group"
	return r.catalogErr
}

func (r *listRepository) Create(ctx context.Context, userID string, input tasks.TaskWrite) (tasks.TaskDetails, error) {
	r.gotContext = ctx
	r.gotUserID = userID
	r.createInput = input
	r.createCalled = true
	return r.item, r.createErr
}

func (r *listRepository) Patch(ctx context.Context, userID, id string, input tasks.TaskPatch) (tasks.TaskDetails, error) {
	r.gotContext = ctx
	r.gotUserID = userID
	r.gotTaskID = id
	r.patchInput = input
	r.patchCalled = true
	return r.item, r.patchErr
}

func (r *listRepository) Delete(ctx context.Context, userID, id string) error {
	r.gotContext = ctx
	r.gotUserID = userID
	r.gotTaskID = id
	r.deleteCalled = true
	return r.deleteErr
}

func requestTaskList(t *testing.T, repository *listRepository, target string) *httptest.ResponseRecorder {
	return requestAPI(t, repository, http.MethodGet, target, "")
}

func requestAPI(t *testing.T, repository *listRepository, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(tasks.NewService(repository), "dev-user").Register(mux)
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

const taskListTarget = "/v1/tasks?start_date=2026-10-10&end_date=2026-10-20"

func TestRecurringOccurrencesAndCompletion(t *testing.T) {
	anchor := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	item := tasks.TaskDetails{
		Task:   tasks.Task{ID: "repeating", UserID: "dev-user", Title: "Read", StartDate: &anchor, EndDate: &anchor, RepeatRule: &tasks.RepeatRule{Type: tasks.RepeatDaily}, StatusID: "status-todo", DurationMinutes: 60},
		Status: tasks.TaskStatus{ID: "status-todo", Label: "Todo"},
	}
	repository := &listRepository{items: []tasks.TaskDetails{item}, item: item, statuses: []tasks.TaskStatus{
		{ID: "status-todo", Label: "Todo"}, {ID: "status-done", Label: "Done", IsCompleted: true},
	}}
	response := requestAPI(t, repository, http.MethodGet, "/v1/task-occurrences?start_date=2026-10-11&end_date=2026-10-12", "")
	if response.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", response.Code, response.Body.String())
	}
	var list TaskOccurrenceListDto
	if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 || list.Items[0].OccurrenceDate != "2026-10-11" || list.Items[1].OccurrenceDate != "2026-10-12" {
		t.Fatalf("occurrences = %#v", list.Items)
	}
	response = requestAPI(t, repository, http.MethodPut, "/v1/tasks/repeating/occurrences/2026-10-11/completion", `{"completed":true}`)
	if response.Code != http.StatusOK {
		t.Fatalf("complete = %d: %s", response.Code, response.Body.String())
	}
	var completed TaskOccurrenceDto
	if err := json.Unmarshal(response.Body.Bytes(), &completed); err != nil {
		t.Fatal(err)
	}
	if !completed.Completed || completed.Status.ID != "status-done" || completed.CompletedAt == nil {
		t.Errorf("completed = %#v", completed)
	}
	response = requestAPI(t, repository, http.MethodGet, "/v1/tasks/repeating/occurrences/2026-10-12", "")
	if response.Code != http.StatusOK {
		t.Fatalf("next occurrence = %d: %s", response.Code, response.Body.String())
	}
	var next TaskOccurrenceDto
	if err := json.Unmarshal(response.Body.Bytes(), &next); err != nil {
		t.Fatal(err)
	}
	if next.Completed || next.Status.ID != "status-todo" {
		t.Errorf("next occurrence affected: %#v", next)
	}
}

func TestOccurrenceRejectsUnscheduledDateAndNonRecurringMutation(t *testing.T) {
	date := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	item := tasks.TaskDetails{Task: tasks.Task{ID: "one-off", StartDate: &date, EndDate: &date}, Status: tasks.TaskStatus{ID: "status-todo"}}
	repository := &listRepository{item: item, items: []tasks.TaskDetails{item}}
	response := requestAPI(t, repository, http.MethodGet, "/v1/tasks/one-off/occurrences/2026-10-11", "")
	if response.Code != http.StatusNotFound {
		t.Errorf("unscheduled = %d", response.Code)
	}
	response = requestAPI(t, repository, http.MethodPut, "/v1/tasks/one-off/occurrences/2026-10-10/status", `{"status_id":"status-done"}`)
	if response.Code != http.StatusConflict {
		t.Errorf("one-off mutation = %d", response.Code)
	}
}

func TestListTasksReturnsJoinedRelationsAndPassesDateRange(t *testing.T) {
	startDate := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	description := "Read the pgx docs"
	color := "accent-green"
	priorityID := "priority-high"
	groupID := "group-work"
	repository := &listRepository{items: []tasks.TaskDetails{
		{
			Task:     tasks.Task{ID: "newer", UserID: "dev-user", Title: "Second task", Note: "important", Description: &description, PriorityID: &priorityID, GroupID: &groupID, DurationMinutes: 60, StatusID: "status-done", StartDate: &startDate, EndDate: &endDate},
			Status:   tasks.TaskStatus{ID: "status-done", Label: "Done", Color: &color, SortOrder: 1, IsCompleted: true, IsBuiltIn: true},
			Priority: &tasks.TaskPriority{ID: priorityID, Label: "High", Color: &color, SortOrder: 2, IsBuiltIn: true},
			Group:    &tasks.TaskGroup{ID: groupID, Label: "Work", Color: "accent-blue", SortOrder: 3},
		},
		{
			Task:   tasks.Task{ID: "older", UserID: "dev-user", Title: "First task", DurationMinutes: 60, StatusID: "status-todo", StartDate: &startDate, EndDate: &endDate},
			Status: tasks.TaskStatus{ID: "status-todo", Label: "To do", IsBuiltIn: true},
		},
	}}
	response := requestTaskList(t, repository, taskListTarget)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if repository.gotUserID != "dev-user" || repository.gotContext == nil {
		t.Errorf("repository did not receive user and request context")
	}
	if repository.gotStartDate.Format(time.DateOnly) != "2026-10-10" || repository.gotEndDate.Format(time.DateOnly) != "2026-10-20" {
		t.Errorf("repository received range %s..%s", repository.gotStartDate, repository.gotEndDate)
	}

	var body TaskListDto
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items = %#v, want two tasks", body.Items)
	}
	if body.Items[0].ID != "newer" || body.Items[1].ID != "older" {
		t.Errorf("task order = [%q, %q], want newer then older", body.Items[0].ID, body.Items[1].ID)
	}
	status := body.Items[0].Status
	if status.ID != "status-done" || status.Label != "Done" || status.Color == nil || *status.Color != color || !status.IsCompleted || !status.IsBuiltIn || status.SortOrder != 1 {
		t.Errorf("joined status = %#v", status)
	}
	if body.Items[0].Description == nil || *body.Items[0].Description != description {
		t.Errorf("description = %#v", body.Items[0].Description)
	}
	priority := body.Items[0].Priority
	if priority == nil || priority.ID != priorityID || priority.Label != "High" || priority.Color == nil || *priority.Color != color || priority.SortOrder != 2 || !priority.IsBuiltIn {
		t.Errorf("priority = %#v", priority)
	}
	group := body.Items[0].Group
	if group == nil || group.ID != groupID || group.Label != "Work" || group.Color != "accent-blue" || group.SortOrder != 3 {
		t.Errorf("group = %#v", group)
	}
	if body.Items[1].Priority != nil || body.Items[1].Group != nil {
		t.Errorf("optional relations should be null: priority=%#v group=%#v", body.Items[1].Priority, body.Items[1].Group)
	}
	for _, rawID := range []string{"status_id", "priority_id", "group_id"} {
		if strings.Contains(response.Body.String(), `"`+rawID+`"`) {
			t.Errorf("response still exposes %s: %q", rawID, response.Body.String())
		}
	}
}

func TestListTasksReturnsEmptyArray(t *testing.T) {
	response := requestTaskList(t, &listRepository{}, taskListTarget)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := strings.TrimSpace(response.Body.String()); got != `{"items":[]}` {
		t.Errorf("body = %q, want empty items array", got)
	}
}

func TestListTasksRejectsInvalidDateRange(t *testing.T) {
	for _, target := range []string{
		"/v1/tasks",
		"/v1/tasks?start_date=bad&end_date=2026-10-20",
		"/v1/tasks?start_date=2026-10-10&end_date=2026-10-09",
	} {
		repository := &listRepository{}
		response := requestTaskList(t, repository, target)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, response.Code)
		}
		if repository.gotContext != nil {
			t.Errorf("%s: repository was called with invalid dates", target)
		}
	}
}

func TestListTasksHidesRepositoryError(t *testing.T) {
	response := requestTaskList(t, &listRepository{err: errors.New("private database detail")}, taskListTarget)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "private database detail") {
		t.Errorf("response exposes repository error: %q", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Something went wrong while reading tasks list") {
		t.Errorf("unexpected error response: %q", response.Body.String())
	}
}

func TestGetTaskReturnsDetails(t *testing.T) {
	color := "accent-green"
	repository := &listRepository{item: tasks.TaskDetails{
		Task:     tasks.Task{ID: "task-1", UserID: "dev-user", Title: "One task", DurationMinutes: 60, StatusID: "status-done"},
		Status:   tasks.TaskStatus{ID: "status-done", Label: "Done", IsCompleted: true},
		Priority: &tasks.TaskPriority{ID: "priority-high", Label: "High", Color: &color, SortOrder: 2},
		Group:    &tasks.TaskGroup{ID: "group-work", Label: "Work", Color: "accent-blue", SortOrder: 3},
	}}
	response := requestTaskList(t, repository, "/v1/tasks/task-1")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if repository.gotUserID != "dev-user" || repository.gotTaskID != "task-1" || repository.gotContext == nil {
		t.Errorf("repository received user=%q task=%q context=%v", repository.gotUserID, repository.gotTaskID, repository.gotContext)
	}
	var body TaskDto
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "task-1" || body.Status.ID != "status-done" || !body.Status.IsCompleted {
		t.Errorf("task and status = %#v", body)
	}
	if body.Priority == nil || body.Priority.ID != "priority-high" || body.Priority.Color == nil || *body.Priority.Color != color {
		t.Errorf("priority = %#v", body.Priority)
	}
	if body.Group == nil || body.Group.ID != "group-work" || body.Group.Color != "accent-blue" {
		t.Errorf("group = %#v", body.Group)
	}
}

func TestGetTaskReturnsNotFound(t *testing.T) {
	response := requestTaskList(t, &listRepository{byIDErr: tasks.ErrNotFound}, "/v1/tasks/missing")
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "task not found") {
		t.Errorf("status = %d, body = %q; want 404", response.Code, response.Body.String())
	}
}

func TestGetTaskHidesRepositoryError(t *testing.T) {
	response := requestTaskList(t, &listRepository{byIDErr: errors.New("private database detail")}, "/v1/tasks/task-1")
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private database detail") {
		t.Errorf("status = %d, body = %q; want safe 500", response.Code, response.Body.String())
	}
}

func TestLookupListsReturnFullDTOs(t *testing.T) {
	color := "accent-green"
	statuses := &listRepository{statuses: []tasks.TaskStatus{
		{ID: "status-todo", Label: "To do", SortOrder: 0, IsBuiltIn: true},
		{ID: "status-done", Label: "Done", Color: &color, SortOrder: 1, IsCompleted: true, IsBuiltIn: true},
	}}
	statusResponse := requestTaskList(t, statuses, "/v1/task-statuses")
	if statusResponse.Code != http.StatusOK || statuses.gotUserID != "dev-user" || statuses.gotContext == nil || statuses.lookupCalled != "statuses" {
		t.Fatalf("status list response = %d, user = %q, method = %q", statusResponse.Code, statuses.gotUserID, statuses.lookupCalled)
	}
	var statusBody StatusListDto
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &statusBody); err != nil {
		t.Fatalf("decode statuses: %v", err)
	}
	if len(statusBody.Items) != 2 || statusBody.Items[0].ID != "status-todo" || statusBody.Items[1].ID != "status-done" || statusBody.Items[1].Color == nil || *statusBody.Items[1].Color != color || !statusBody.Items[1].IsCompleted {
		t.Errorf("statuses = %#v", statusBody.Items)
	}

	groups := &listRepository{groups: []tasks.TaskGroup{
		{ID: "group-home", Label: "Home", Color: "accent-blue", SortOrder: 0},
		{ID: "group-work", Label: "Work", Color: "accent-red", SortOrder: 1},
	}}
	groupResponse := requestTaskList(t, groups, "/v1/task-groups")
	if groupResponse.Code != http.StatusOK || groups.gotUserID != "dev-user" || groups.gotContext == nil || groups.lookupCalled != "groups" {
		t.Fatalf("group list response = %d, user = %q, method = %q", groupResponse.Code, groups.gotUserID, groups.lookupCalled)
	}
	var groupBody GroupListDto
	if err := json.Unmarshal(groupResponse.Body.Bytes(), &groupBody); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	if len(groupBody.Items) != 2 || groupBody.Items[0].ID != "group-home" || groupBody.Items[1].ID != "group-work" || groupBody.Items[1].Color != "accent-red" || groupBody.Items[1].SortOrder != 1 {
		t.Errorf("groups = %#v", groupBody.Items)
	}

	priorities := &listRepository{priorities: []tasks.TaskPriority{
		{ID: "priority-low", Label: "Low", SortOrder: 0, IsBuiltIn: true},
		{ID: "priority-high", Label: "High", Color: &color, SortOrder: 1, IsBuiltIn: true},
	}}
	priorityResponse := requestTaskList(t, priorities, "/v1/task-priorities")
	if priorityResponse.Code != http.StatusOK || priorities.gotUserID != "dev-user" || priorities.gotContext == nil || priorities.lookupCalled != "priorities" {
		t.Fatalf("priority list response = %d, user = %q, method = %q", priorityResponse.Code, priorities.gotUserID, priorities.lookupCalled)
	}
	var priorityBody PriorityListDto
	if err := json.Unmarshal(priorityResponse.Body.Bytes(), &priorityBody); err != nil {
		t.Fatalf("decode priorities: %v", err)
	}
	if len(priorityBody.Items) != 2 || priorityBody.Items[0].ID != "priority-low" || priorityBody.Items[1].ID != "priority-high" || priorityBody.Items[1].Color == nil || *priorityBody.Items[1].Color != color || !priorityBody.Items[1].IsBuiltIn {
		t.Errorf("priorities = %#v", priorityBody.Items)
	}
}

func TestLookupListsReturnEmptyArraysAndHideErrors(t *testing.T) {
	for _, target := range []string{"/v1/task-statuses", "/v1/task-groups", "/v1/task-priorities"} {
		t.Run(target, func(t *testing.T) {
			response := requestTaskList(t, &listRepository{}, target)
			if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"items":[]}` {
				t.Errorf("empty response = %d %q", response.Code, response.Body.String())
			}
			response = requestTaskList(t, &listRepository{lookupErr: errors.New("private database detail")}, target)
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private database detail") {
				t.Errorf("error response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestCreateTaskAppliesDefaultsAndReturnsLocation(t *testing.T) {
	date := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	repository := &listRepository{item: tasks.TaskDetails{
		Task:   tasks.Task{ID: "new-task", UserID: "dev-user", Title: "Write Go", StatusID: "status-todo", DurationMinutes: 60, StartDate: &date, EndDate: &date},
		Status: tasks.TaskStatus{ID: "status-todo", Label: "To do"},
	}}
	response := requestAPI(t, repository, http.MethodPost, "/v1/tasks", `{"title":"Write Go","start_date":"2026-10-10"}`)
	if response.Code != http.StatusCreated || response.Header().Get("Location") != "/v1/tasks/new-task" {
		t.Fatalf("response = %d, Location = %q, body = %q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	if !repository.createCalled || repository.gotUserID != "dev-user" || repository.gotContext == nil {
		t.Errorf("create was not called for current user")
	}
	input := repository.createInput
	if input.StatusID != "status-todo" || input.DurationMinutes != 60 || input.StartDate == nil || input.EndDate == nil || !input.StartDate.Equal(*input.EndDate) {
		t.Errorf("create defaults and dates = %#v", input)
	}
	var body TaskDto
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.ID != "new-task" || body.Status.ID != "status-todo" || body.StartDate == nil || *body.StartDate != "2026-10-10" || body.EndDate == nil || *body.EndDate != "2026-10-10" {
		t.Errorf("created task = %#v, decode error = %v", body, err)
	}
}

func TestCreateTaskRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"title":" "}`,
		`{"title":"Task","start_date":"bad"}`,
		`{"title":"Task","duration_minutes":0}`,
		`{"title":"Task","start_minute":1440}`,
		`{"title":"Task","unknown":true}`,
		`{"title":"Task","repeat_rule":{"type":"yearly"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			repository := &listRepository{}
			response := requestAPI(t, repository, http.MethodPost, "/v1/tasks", body)
			if response.Code != http.StatusBadRequest || repository.createCalled {
				t.Errorf("response = %d, create called = %v", response.Code, repository.createCalled)
			}
		})
	}
}

func TestPatchTaskDistinguishesNullAndOmittedFields(t *testing.T) {
	repository := &listRepository{item: tasks.TaskDetails{
		Task:   tasks.Task{ID: "task-1", UserID: "dev-user", Title: "Changed", StatusID: "status-todo", DurationMinutes: 60},
		Status: tasks.TaskStatus{ID: "status-todo", Label: "To do"},
	}}
	response := requestAPI(t, repository, http.MethodPatch, "/v1/tasks/task-1", `{"title":"Changed","priority_id":null,"start_date":null,"end_date":null}`)
	if response.Code != http.StatusOK || !repository.patchCalled || repository.gotTaskID != "task-1" || repository.gotUserID != "dev-user" {
		t.Fatalf("response = %d, patch called = %v, task ID = %q", response.Code, repository.patchCalled, repository.gotTaskID)
	}
	patch := repository.patchInput
	if !patch.Title.Set || patch.Title.Value != "Changed" || !patch.PriorityID.Set || patch.PriorityID.Value != nil || !patch.StartDate.Set || !patch.EndDate.Set || patch.StartDate.Value != nil || patch.EndDate.Value != nil || patch.GroupID.Set {
		t.Errorf("patch fields = %#v", patch)
	}
	var body TaskDto
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.ID != "task-1" {
		t.Errorf("patched task = %#v, decode error = %v", body, err)
	}
}

func TestPatchTaskRejectsInvalidInputAndMapsErrors(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"title":null}`,
		`{"start_date":"2026-10-10"}`,
		`{"start_date":"2026-10-20","end_date":"2026-10-10"}`,
		`{"duration_minutes":32768}`,
		`{"note":null}`,
	} {
		t.Run(body, func(t *testing.T) {
			repository := &listRepository{}
			response := requestAPI(t, repository, http.MethodPatch, "/v1/tasks/task-1", body)
			if response.Code != http.StatusBadRequest || repository.patchCalled {
				t.Errorf("response = %d, patch called = %v", response.Code, repository.patchCalled)
			}
		})
	}
	for _, tc := range []struct {
		err  error
		want int
	}{
		{tasks.ErrNotFound, http.StatusNotFound},
		{tasks.ErrInvalidReference, http.StatusBadRequest},
		{errors.New("private database detail"), http.StatusInternalServerError},
	} {
		response := requestAPI(t, &listRepository{patchErr: tc.err}, http.MethodPatch, "/v1/tasks/task-1", `{"title":"Changed"}`)
		if response.Code != tc.want || strings.Contains(response.Body.String(), "private database detail") {
			t.Errorf("error %v: response = %d %q", tc.err, response.Code, response.Body.String())
		}
	}
}

func TestDeleteTaskReturnsNoContentAndNotFound(t *testing.T) {
	repository := &listRepository{}
	response := requestAPI(t, repository, http.MethodDelete, "/v1/tasks/task-1", "")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 || !repository.deleteCalled || repository.gotTaskID != "task-1" || repository.gotUserID != "dev-user" {
		t.Errorf("delete response = %d %q, repository = %#v", response.Code, response.Body.String(), repository)
	}
	response = requestAPI(t, &listRepository{deleteErr: tasks.ErrNotFound}, http.MethodDelete, "/v1/tasks/missing", "")
	if response.Code != http.StatusNotFound {
		t.Errorf("missing task delete response = %d", response.Code)
	}
}
