package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plans-tracker-backend/internal/habits"
)

type listRepository struct {
	habits.Repository
	items     []habits.HabitDetails
	err       error
	userID    string
	created   habits.HabitInput
	patch     habits.HabitPatch
	completed bool
	filter    habits.CompletionFilter
	category  habits.CategoryInput
	catPatch  habits.CategoryPatch
	deletedID string
}

func (r *listRepository) List(_ context.Context, userID string) ([]habits.HabitDetails, error) {
	r.userID = userID
	return r.items, r.err
}

func (r *listRepository) Create(_ context.Context, userID string, input habits.HabitInput) (habits.HabitDetails, error) {
	r.userID, r.created = userID, input
	return habits.HabitDetails{Habit: habits.Habit{ID: "habit-1", Title: input.Title, RepeatRule: input.RepeatRule, DurationMinutes: input.DurationMinutes}}, nil
}

func (r *listRepository) Patch(_ context.Context, userID, id string, patch habits.HabitPatch) (habits.HabitDetails, error) {
	r.userID, r.patch = userID, patch
	return habits.HabitDetails{Habit: habits.Habit{ID: id, Title: "Updated", RepeatRule: habits.RepeatRule{Type: habits.RepeatDaily}, DurationMinutes: 60}}, nil
}

func (r *listRepository) CompletionList(_ context.Context, userID string, filter habits.CompletionFilter) ([]habits.HabitCompletion, error) {
	r.userID, r.filter = userID, filter
	return nil, nil
}

func (r *listRepository) SetCompletion(_ context.Context, userID, habitID string, date time.Time, completed bool) (*habits.HabitCompletion, error) {
	r.userID, r.completed = userID, completed
	if !completed {
		return nil, nil
	}
	return &habits.HabitCompletion{ID: "completion-1", HabitID: habitID, OccurrenceDate: date, CompletedAt: date}, nil
}

func (r *listRepository) ByID(_ context.Context, _, _ string) (habits.HabitDetails, error) {
	return habits.HabitDetails{}, habits.ErrNotFound
}

func (r *listRepository) CategoryList(_ context.Context, _ string) ([]habits.HabitCategory, error) {
	return nil, nil
}

func (r *listRepository) CreateCategory(_ context.Context, _ string, input habits.CategoryInput) (habits.HabitCategory, error) {
	r.category = input
	return habits.HabitCategory{ID: "category-1", Label: input.Label, Color: input.Color}, nil
}

func (r *listRepository) PatchCategory(_ context.Context, _, id string, patch habits.CategoryPatch) (habits.HabitCategory, error) {
	r.catPatch = patch
	return habits.HabitCategory{ID: id, Label: "Updated"}, nil
}

func (r *listRepository) DeleteCategory(_ context.Context, _, id string) error {
	r.deletedID = id
	return nil
}

func TestListHabits(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	repository := &listRepository{items: []habits.HabitDetails{{Habit: habits.Habit{
		ID: "habit-1", Title: "Read", RepeatRule: habits.RepeatRule{Type: "daily"},
		DurationMinutes: 60, CreatedAt: now, UpdatedAt: now,
	}}}}
	mux := http.NewServeMux()
	NewHandler(habits.NewService(repository), "user-1").Register(mux)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/habits", nil))

	if response.Code != http.StatusOK || repository.userID != "user-1" {
		t.Fatalf("status = %d, userID = %q", response.Code, repository.userID)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	for _, fragment := range []string{`"id":"habit-1"`, `"repeat_rule":{"type":"daily"}`, `"category_id":null`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("response missing %s: %s", fragment, response.Body.String())
		}
	}
}

func TestListHabitsEmptyAndFailure(t *testing.T) {
	repository := &listRepository{}
	mux := http.NewServeMux()
	NewHandler(habits.NewService(repository), "user-1").Register(mux)

	empty := httptest.NewRecorder()
	mux.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/v1/habits", nil))
	if empty.Code != http.StatusOK || empty.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("empty response = %d %q", empty.Code, empty.Body.String())
	}

	repository.err = errors.New("database unavailable")
	failure := httptest.NewRecorder()
	mux.ServeHTTP(failure, httptest.NewRequest(http.MethodGet, "/v1/habits", nil))
	if failure.Code != http.StatusInternalServerError {
		t.Fatalf("failure status = %d", failure.Code)
	}
	if strings.Contains(failure.Body.String(), "database unavailable") {
		t.Fatalf("internal error leaked: %q", failure.Body.String())
	}
}

func TestHabitCreateAndPatchRequests(t *testing.T) {
	repository := &listRepository{}
	mux := http.NewServeMux()
	NewHandler(habits.NewService(repository), "user-1").Register(mux)

	created := httptest.NewRecorder()
	mux.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/v1/habits", strings.NewReader(`{"title":"Read","repeat_rule":{"type":"weekly","daysOfWeek":[1,3]}}`)))
	if created.Code != http.StatusCreated || created.Header().Get("Location") != "/v1/habits/habit-1" || repository.created.DurationMinutes != 60 || repository.userID != "user-1" {
		t.Fatalf("create = %d %s, input = %#v", created.Code, created.Body.String(), repository.created)
	}

	patched := httptest.NewRecorder()
	mux.ServeHTTP(patched, httptest.NewRequest(http.MethodPatch, "/v1/habits/habit-1", strings.NewReader(`{"category_id":null,"start_minute":null}`)))
	if patched.Code != http.StatusOK || !repository.patch.CategoryID.Set || repository.patch.CategoryID.Value != nil || !repository.patch.StartMinute.Set {
		t.Fatalf("patch = %d %s, input = %#v", patched.Code, patched.Body.String(), repository.patch)
	}

	for _, body := range []string{
		`{"title":"","repeat_rule":{"type":"daily"}}`,
		`{"title":"Read","repeat_rule":{"type":"weekly","daysOfWeek":[]}}`,
		`{"title":"Read","repeat_rule":{"type":"daily","unknown":1}}`,
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/habits", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid %s = %d", body, response.Code)
		}
	}
}

func TestCompletionRangeAndState(t *testing.T) {
	repository := &listRepository{}
	mux := http.NewServeMux()
	NewHandler(habits.NewService(repository), "user-1").Register(mux)

	listed := httptest.NewRecorder()
	mux.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/v1/habit-completions?start_date=2026-10-26&end_date=2026-11-01&habit_id=habit-1", nil))
	if listed.Code != http.StatusOK || repository.filter.HabitID != "habit-1" || repository.filter.StartDate == nil || repository.filter.EndDate == nil {
		t.Fatalf("list = %d %s, filter = %#v", listed.Code, listed.Body.String(), repository.filter)
	}

	for _, query := range []string{"?start_date=2026-10-26", "?start_date=2026-11-01&end_date=2026-10-26", "?start_date=bad&end_date=2026-11-01"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/habit-completions"+query, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid range %s = %d", query, response.Code)
		}
	}

	for _, completed := range []string{"true", "false"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/v1/habits/habit-1/completions/2026-10-30", strings.NewReader(`{"completed":`+completed+`}`)))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"completed":`+completed) {
			t.Fatalf("set %s = %d %s", completed, response.Code, response.Body.String())
		}
	}
}

func TestCategoryRoutesAndNotFound(t *testing.T) {
	repository := &listRepository{}
	mux := http.NewServeMux()
	NewHandler(habits.NewService(repository), "user-1").Register(mux)

	created := httptest.NewRecorder()
	mux.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/v1/habit-categories", strings.NewReader(`{"label":"Health"}`)))
	if created.Code != http.StatusCreated || created.Header().Get("Location") != "/v1/habit-categories/category-1" || repository.category.Label != "Health" {
		t.Fatalf("create category = %d %s, input = %#v", created.Code, created.Body.String(), repository.category)
	}
	patched := httptest.NewRecorder()
	mux.ServeHTTP(patched, httptest.NewRequest(http.MethodPatch, "/v1/habit-categories/category-1", strings.NewReader(`{"color":null,"sort_order":2}`)))
	if patched.Code != http.StatusOK || !repository.catPatch.Color.Set || repository.catPatch.Color.Value != nil || repository.catPatch.SortOrder.Value != 2 {
		t.Fatalf("patch category = %d %s, input = %#v", patched.Code, patched.Body.String(), repository.catPatch)
	}
	deleted := httptest.NewRecorder()
	mux.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, "/v1/habit-categories/category-1", nil))
	if deleted.Code != http.StatusNoContent || repository.deletedID != "category-1" {
		t.Fatalf("delete category = %d, id = %q", deleted.Code, repository.deletedID)
	}
	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/v1/habits/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing habit status = %d", missing.Code)
	}
	invalid := httptest.NewRecorder()
	mux.ServeHTTP(invalid, httptest.NewRequest(http.MethodPatch, "/v1/habit-categories/category-1", strings.NewReader(`{"sort_order":-1}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid category status = %d", invalid.Code)
	}
}
