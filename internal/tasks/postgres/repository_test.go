package postgres

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"plans-tracker-backend/internal/tasks"
)

func TestRecurringTaskBacklogAndOccurrenceState(t *testing.T) {
	pool, ctx := openTestPool(t)
	var userID string
	if err := pool.QueryRow(ctx, "SELECT gen_random_uuid()::text").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, table := range []string{"tasks", "task_statuses", "task_priorities"} {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM "+table+" WHERE user_id = $1", userID); err != nil {
				t.Errorf("cleanup %s: %v", table, err)
			}
		}
	})
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var recurringID string
	err := pool.QueryRow(ctx, `INSERT INTO tasks (user_id, title, start_date, end_date, repeat_rule) VALUES ($1, 'daily', $2, $2, '{"type":"daily"}') RETURNING id`, userID, anchor).Scan(&recurringID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tasks (user_id, title) VALUES ($1, 'backlog')`, userID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	start := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	listed, err := repo.List(ctx, userID, start, start)
	if err != nil || len(listed) != 1 || listed[0].Task.ID != recurringID {
		t.Fatalf("recurring list = %#v, %v", listed, err)
	}
	backlog, err := repo.Backlog(ctx, userID)
	if err != nil || len(backlog) != 1 || backlog[0].Task.Title != "backlog" {
		t.Fatalf("backlog = %#v, %v", backlog, err)
	}
	if err := repo.SetOccurrenceStatus(ctx, userID, recurringID, start, "status-done"); err != nil {
		t.Fatal(err)
	}
	states, err := repo.OccurrenceStates(ctx, userID, start, start)
	if err != nil {
		t.Fatal(err)
	}
	key := tasks.OccurrenceKey{TaskID: recurringID, Date: "2026-02-02"}
	if states[key].StatusID != "status-done" || states[key].CompletedAt == nil {
		t.Fatalf("done state = %#v", states[key])
	}
	if err := repo.SetOccurrenceCompletion(ctx, userID, recurringID, start, false); err != nil {
		t.Fatal(err)
	}
	states, err = repo.OccurrenceStates(ctx, userID, start, start)
	if err != nil {
		t.Fatal(err)
	}
	if states[key].StatusID != "status-todo" || states[key].CompletedAt != nil {
		t.Fatalf("cleared state = %#v", states[key])
	}
	if err := repo.SetOccurrenceStatus(ctx, userID, recurringID, start, "status-done"); err != nil {
		t.Fatal(err)
	}
	newAnchor := anchor.AddDate(0, 0, 1)
	_, err = repo.Patch(ctx, userID, recurringID, tasks.TaskPatch{
		StartDate: tasks.Change[*time.Time]{Set: true, Value: &newAnchor},
		EndDate:   tasks.Change[*time.Time]{Set: true, Value: &newAnchor},
	})
	if err != nil {
		t.Fatal(err)
	}
	states, err = repo.OccurrenceStates(ctx, userID, start, start)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 0 {
		t.Fatalf("old schedule state survived date change: %#v", states)
	}
}

func TestListAndByIDFilterByUserAndLoadTaskDetails(t *testing.T) {
	pool, ctx := openTestPool(t)

	var userID, otherUserID string
	if err := pool.QueryRow(ctx, "SELECT gen_random_uuid()::text, gen_random_uuid()::text").Scan(&userID, &otherUserID); err != nil {
		t.Fatalf("create test user IDs: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for _, id := range []string{userID, otherUserID} {
			for _, table := range []string{"tasks", "task_groups", "task_statuses", "task_priorities"} {
				if _, err := pool.Exec(cleanupCtx, "DELETE FROM "+table+" WHERE user_id = $1", id); err != nil {
					t.Errorf("clean %s fixtures: %v", table, err)
				}
			}
		}
	})

	startDate := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)
	middleDate := startDate.AddDate(0, 0, 5)
	beforeDate := startDate.AddDate(0, 0, -1)
	wellBeforeDate := beforeDate.AddDate(0, 0, -1)
	afterDate := endDate.AddDate(0, 0, 1)
	wellAfterDate := afterDate.AddDate(0, 0, 1)
	createdAt := time.Now()

	startBoundaryID := insertTestTask(t, ctx, pool, userID, "at start boundary", createdAt.Add(-5*time.Hour), &startDate, &startDate)
	endBoundaryID := insertTestTask(t, ctx, pool, userID, "at end boundary", createdAt.Add(-4*time.Hour), &endDate, &endDate)
	leftOverlapID := insertTestTask(t, ctx, pool, userID, "starts before range", createdAt.Add(-3*time.Hour), &beforeDate, &middleDate)
	rightOverlapID := insertTestTask(t, ctx, pool, userID, "ends after range", createdAt.Add(-2*time.Hour), &middleDate, &afterDate)
	spanningID := insertTestTask(t, ctx, pool, userID, "spans entire range", createdAt.Add(-time.Hour), &beforeDate, &afterDate)
	otherTaskID := insertTestTask(t, ctx, pool, otherUserID, "someone else's", createdAt, &middleDate, &middleDate)
	insertTestTask(t, ctx, pool, userID, "before range", createdAt, &wellBeforeDate, &beforeDate)
	insertTestTask(t, ctx, pool, userID, "after range", createdAt, &afterDate, &wellAfterDate)
	undatedID := insertTestTask(t, ctx, pool, userID, "without dates", createdAt, nil, nil)
	if _, err := pool.Exec(ctx,
		"UPDATE task_statuses SET label = 'My status', color = 'accent-green' WHERE user_id = $1 AND id = 'status-todo'",
		userID,
	); err != nil {
		t.Fatalf("customize test user's status: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"UPDATE task_priorities SET label = 'My priority', color = 'accent-red' WHERE user_id = $1 AND id = 'priority-high'",
		userID,
	); err != nil {
		t.Fatalf("customize test user's priority: %v", err)
	}
	var groupID string
	if err := pool.QueryRow(ctx,
		"INSERT INTO task_groups (user_id, label, color, sort_order) VALUES ($1, 'My group', 'accent-blue', 3) RETURNING id",
		userID,
	).Scan(&groupID); err != nil {
		t.Fatalf("insert test group: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"UPDATE tasks SET priority_id = 'priority-high', group_id = $1 WHERE user_id = $2 AND id = $3",
		groupID, userID, spanningID,
	); err != nil {
		t.Fatalf("attach priority and group to test task: %v", err)
	}

	repository := NewRepository(pool)
	got, err := repository.List(ctx, userID, startDate, endDate)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("tasks = %#v, want exactly five overlapping tasks", got)
	}
	wantIDs := []string{spanningID, rightOverlapID, leftOverlapID, endBoundaryID, startBoundaryID}
	for i, item := range got {
		if item.Task.ID != wantIDs[i] {
			t.Errorf("task %d ID = %q, want %q", i, item.Task.ID, wantIDs[i])
		}
	}
	for _, item := range got {
		task := item.Task
		if task.UserID != userID || task.StatusID != "status-todo" || task.DurationMinutes != 60 {
			t.Errorf("unexpected task fields: %#v", task)
		}
		if item.Status.ID != task.StatusID || item.Status.Label != "My status" || item.Status.Color == nil || *item.Status.Color != "accent-green" || !item.Status.IsBuiltIn {
			t.Errorf("status was not joined: %#v", item.Status)
		}
		if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
			t.Errorf("timestamps were not scanned: %#v", task)
		}
		if task.ID == spanningID {
			if item.Priority == nil || item.Priority.ID != "priority-high" || item.Priority.Label != "My priority" || item.Priority.Color == nil || *item.Priority.Color != "accent-red" {
				t.Errorf("priority was not joined: %#v", item.Priority)
			}
			if item.Group == nil || item.Group.ID != groupID || item.Group.Label != "My group" || item.Group.Color != "accent-blue" || item.Group.SortOrder != 3 {
				t.Errorf("group was not joined: %#v", item.Group)
			}
		} else if item.Priority != nil || item.Group != nil {
			t.Errorf("optional relations should be nil: priority=%#v group=%#v", item.Priority, item.Group)
		}
	}

	item, err := repository.ByID(ctx, userID, spanningID)
	if err != nil {
		t.Fatalf("get task by ID: %v", err)
	}
	if item.Task.ID != spanningID || item.Status.Label != "My status" || item.Priority == nil || item.Priority.Label != "My priority" || item.Group == nil || item.Group.ID != groupID {
		t.Errorf("task details = %#v", item)
	}
	item, err = repository.ByID(ctx, userID, undatedID)
	if err != nil || item.Task.ID != undatedID || item.Priority != nil || item.Group != nil {
		t.Errorf("undated task details = %#v, error = %v", item, err)
	}
	for _, id := range []string{otherTaskID, "missing-task"} {
		if _, err := repository.ByID(ctx, userID, id); !errors.Is(err, tasks.ErrNotFound) {
			t.Errorf("task %q: error = %v, want ErrNotFound", id, err)
		}
	}
}

func TestCreatePatchDeleteTask(t *testing.T) {
	pool, ctx := openTestPool(t)
	var userID, otherUserID string
	if err := pool.QueryRow(ctx, "SELECT gen_random_uuid()::text, gen_random_uuid()::text").Scan(&userID, &otherUserID); err != nil {
		t.Fatalf("create test user IDs: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, id := range []string{userID, otherUserID} {
			for _, table := range []string{"tasks", "task_groups", "task_statuses", "task_priorities"} {
				if _, err := pool.Exec(cleanupCtx, "DELETE FROM "+table+" WHERE user_id = $1", id); err != nil {
					t.Errorf("clean %s fixtures: %v", table, err)
				}
			}
		}
	})

	service := tasks.NewService(NewRepository(pool))
	date := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	item, err := service.Create(ctx, userID, tasks.TaskWrite{
		Title: "Write Go", StatusID: "status-todo", DurationMinutes: 60,
		StartDate: &date, RepeatRule: &tasks.RepeatRule{Type: tasks.RepeatDaily},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if item.Task.ID == "" || item.Task.EndDate == nil || !item.Task.EndDate.Equal(date) || item.Status.ID != "status-todo" || item.Task.RepeatRule == nil || item.Task.RepeatRule.Type != tasks.RepeatDaily {
		t.Errorf("created task = %#v", item)
	}
	var groupID string
	if err := pool.QueryRow(ctx,
		"INSERT INTO task_groups (user_id, label, color) VALUES ($1, 'Work', 'accent-blue') RETURNING id",
		userID,
	).Scan(&groupID); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	priorityID := "priority-high"
	item, err = service.Patch(ctx, userID, item.Task.ID, tasks.TaskPatch{
		Title:      tasks.Change[string]{Set: true, Value: "Updated"},
		PriorityID: tasks.Change[*string]{Set: true, Value: &priorityID},
		GroupID:    tasks.Change[*string]{Set: true, Value: &groupID},
		StartDate:  tasks.Change[*time.Time]{Set: true},
		EndDate:    tasks.Change[*time.Time]{Set: true},
		RepeatRule: tasks.Change[*tasks.RepeatRule]{Set: true},
	})
	if err != nil {
		t.Fatalf("patch task: %v", err)
	}
	if item.Task.Title != "Updated" || item.Task.StartDate != nil || item.Task.EndDate != nil || item.Task.RepeatRule != nil || item.Priority == nil || item.Priority.ID != priorityID || item.Group == nil || item.Group.ID != groupID {
		t.Errorf("patched task = %#v", item)
	}
	if _, err := service.Patch(ctx, userID, item.Task.ID, tasks.TaskPatch{
		StatusID: tasks.Change[string]{Set: true, Value: "missing-status"},
	}); !errors.Is(err, tasks.ErrInvalidReference) {
		t.Errorf("invalid status error = %v, want ErrInvalidReference", err)
	}
	if _, err := service.Patch(ctx, otherUserID, item.Task.ID, tasks.TaskPatch{
		Title: tasks.Change[string]{Set: true, Value: "Other user's edit"},
	}); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's patch error = %v, want ErrNotFound", err)
	}
	if err := service.Delete(ctx, otherUserID, item.Task.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's delete error = %v, want ErrNotFound", err)
	}
	if err := service.Delete(ctx, userID, item.Task.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if _, err := service.ByID(ctx, userID, item.Task.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("deleted task lookup error = %v, want ErrNotFound", err)
	}
}

func TestLookupTablesFilterAndScan(t *testing.T) {
	pool, ctx := openTestPool(t)
	var userID, otherUserID string
	if err := pool.QueryRow(ctx, "SELECT gen_random_uuid()::text, gen_random_uuid()::text").Scan(&userID, &otherUserID); err != nil {
		t.Fatalf("create test user IDs: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, id := range []string{userID, otherUserID} {
			for _, table := range []string{"task_groups", "task_statuses", "task_priorities"} {
				if _, err := pool.Exec(cleanupCtx, "DELETE FROM "+table+" WHERE user_id = $1", id); err != nil {
					t.Errorf("clean %s fixtures: %v", table, err)
				}
			}
		}
	})

	if _, err := pool.Exec(ctx, "SELECT ensure_task_defaults($1)", userID); err != nil {
		t.Fatalf("seed lookup values: %v", err)
	}
	if _, err := pool.Exec(ctx, "SELECT ensure_task_defaults($1)", otherUserID); err != nil {
		t.Fatalf("seed other user's lookup values: %v", err)
	}
	var groupID string
	if err := pool.QueryRow(ctx,
		"INSERT INTO task_groups (user_id, label, color, sort_order) VALUES ($1, 'Personal', 'custom-color', 0) RETURNING id",
		userID,
	).Scan(&groupID); err != nil {
		t.Fatalf("insert group: %v", err)
	}

	repository := NewRepository(pool)
	priorities, err := repository.PriorityList(ctx, userID)
	if err != nil || len(priorities) != 3 {
		t.Fatalf("priorities = %#v, err = %v", priorities, err)
	}
	if priorities[0].ID != "priority-low" || priorities[0].Color == nil || *priorities[0].Color != "gray-gray" {
		t.Errorf("first priority = %#v, want low with gray color", priorities[0])
	}
	priority, err := repository.PriorityById(ctx, userID, "priority-low")
	if err != nil || priority.ID != "priority-low" {
		t.Errorf("priority by ID = %#v, err = %v", priority, err)
	}

	statuses, err := repository.StatusList(ctx, userID)
	if err != nil || len(statuses) != 2 {
		t.Fatalf("statuses = %#v, err = %v", statuses, err)
	}
	if statuses[0].ID != "status-todo" || statuses[0].Color != nil {
		t.Errorf("first status = %#v, want todo with NULL color", statuses[0])
	}
	status, err := repository.StatusById(ctx, userID, "status-done")
	if err != nil || status.ID != "status-done" || !status.IsCompleted {
		t.Errorf("status by ID = %#v, err = %v", status, err)
	}

	groups, err := repository.GroupList(ctx, userID)
	if err != nil || len(groups) != 1 || groups[0].ID != groupID {
		t.Fatalf("groups = %#v, err = %v", groups, err)
	}
	group, err := repository.GroupById(ctx, userID, groupID)
	if err != nil || group.ID != groupID || group.Color != "custom-color" {
		t.Errorf("group by ID = %#v, err = %v", group, err)
	}
	if _, err := repository.GroupById(ctx, otherUserID, groupID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's group lookup error = %v, want ErrNotFound", err)
	}
}

func openTestPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Path != "/tasks_test" {
		t.Fatal("TEST_DATABASE_URL must point to the separate tasks_test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("configure test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	return pool, ctx
}

func insertTestTask(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	userID, title string,
	createdAt time.Time,
	startDate, endDate *time.Time,
) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx,
		"INSERT INTO tasks (user_id, title, created_at, start_date, end_date) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		userID, title, createdAt, startDate, endDate,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert test task: %v", err)
	}
	return id
}
