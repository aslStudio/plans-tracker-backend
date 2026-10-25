package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"plans-tracker-backend/internal/tasks"
)

func TestCatalogCRUDAndTaskReferences(t *testing.T) {
	pool, ctx := openTestPool(t)
	var userID, otherUserID string
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()::text, gen_random_uuid()::text`).Scan(&userID, &otherUserID); err != nil {
		t.Fatalf("create user IDs: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, id := range []string{userID, otherUserID} {
			for _, table := range []string{"task_occurrence_statuses", "tasks", "task_groups", "task_statuses", "task_priorities"} {
				if _, err := pool.Exec(cleanupCtx, "DELETE FROM "+table+" WHERE user_id = $1", id); err != nil {
					t.Errorf("clean %s: %v", table, err)
				}
			}
		}
	})

	service := tasks.NewService(NewRepository(pool))
	color := "accent-blue"
	status, err := service.CreateStatus(ctx, userID, tasks.StatusInput{Label: "In progress", Color: &color, IsCompleted: true})
	if err != nil || status.ID == "" || status.IsBuiltIn || status.SortOrder != 1 {
		t.Fatalf("created status = %#v, error = %v", status, err)
	}
	done, err := service.StatusByID(ctx, userID, "status-done")
	if err != nil || done.SortOrder != 2 {
		t.Errorf("done status after insertion = %#v, error = %v", done, err)
	}
	priority, err := service.CreatePriority(ctx, userID, tasks.PriorityInput{Label: "Urgent", Color: &color})
	if err != nil || priority.ID == "" || priority.IsBuiltIn || priority.SortOrder != 3 {
		t.Fatalf("created priority = %#v, error = %v", priority, err)
	}
	group, err := service.CreateGroup(ctx, userID, tasks.GroupInput{Label: "Work", Color: "accent-blue"})
	if err != nil || group.ID == "" || group.SortOrder != 0 {
		t.Fatalf("created group = %#v, error = %v", group, err)
	}
	if _, err := service.StatusByID(ctx, otherUserID, status.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's status lookup = %v, want ErrNotFound", err)
	}
	if _, err := service.PriorityByID(ctx, otherUserID, priority.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's priority lookup = %v, want ErrNotFound", err)
	}
	if _, err := service.GroupByID(ctx, otherUserID, group.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's group lookup = %v, want ErrNotFound", err)
	}
	if _, err := service.PatchGroup(ctx, otherUserID, group.ID, tasks.GroupPatch{
		Label: tasks.Change[string]{Set: true, Value: "Other user's edit"},
	}); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's group patch = %v, want ErrNotFound", err)
	}
	if err := service.DeletePriority(ctx, otherUserID, priority.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's priority delete = %v, want ErrNotFound", err)
	}
	status, err = service.PatchStatus(ctx, userID, status.ID, tasks.StatusPatch{
		Label:       tasks.Change[string]{Set: true, Value: "Doing"},
		Color:       tasks.Change[*string]{Set: true},
		IsCompleted: tasks.Change[bool]{Set: true, Value: false},
	})
	if err != nil || status.Label != "Doing" || status.Color != nil || status.IsCompleted {
		t.Errorf("patched status = %#v, error = %v", status, err)
	}
	priority, err = service.PatchPriority(ctx, userID, priority.ID, tasks.PriorityPatch{
		Label: tasks.Change[string]{Set: true, Value: "Critical"},
	})
	if err != nil || priority.Label != "Critical" {
		t.Errorf("patched priority = %#v, error = %v", priority, err)
	}
	group, err = service.PatchGroup(ctx, userID, group.ID, tasks.GroupPatch{
		Color: tasks.Change[string]{Set: true, Value: "accent-red"},
	})
	if err != nil || group.Color != "accent-red" {
		t.Errorf("patched group = %#v, error = %v", group, err)
	}

	task, err := service.Create(ctx, userID, tasks.TaskWrite{
		Title: "Linked task", StatusID: status.ID, PriorityID: &priority.ID,
		GroupID: &group.ID, DurationMinutes: 60,
	})
	if err != nil {
		t.Fatalf("create task with catalog references: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO task_occurrence_statuses (user_id, task_id, occurrence_date, status_id)
		VALUES ($1, $2, DATE '2026-10-10', $3)
	`, userID, task.Task.ID, status.ID); err != nil {
		t.Fatalf("create occurrence status: %v", err)
	}
	if err := service.DeleteStatus(ctx, userID, "status-todo"); !errors.Is(err, tasks.ErrProtected) {
		t.Errorf("delete built-in status = %v, want ErrProtected", err)
	}
	if _, err := service.PatchStatus(ctx, userID, "status-todo", tasks.StatusPatch{
		IsCompleted: tasks.Change[bool]{Set: true, Value: true},
	}); !errors.Is(err, tasks.ErrImmutable) {
		t.Errorf("change built-in completion = %v, want ErrImmutable", err)
	}
	if err := service.DeletePriority(ctx, userID, "priority-high"); !errors.Is(err, tasks.ErrProtected) {
		t.Errorf("delete built-in priority = %v, want ErrProtected", err)
	}
	if err := service.DeleteStatus(ctx, otherUserID, status.ID); !errors.Is(err, tasks.ErrNotFound) {
		t.Errorf("other user's delete = %v, want ErrNotFound", err)
	}
	if err := service.DeleteStatus(ctx, userID, status.ID); err != nil {
		t.Fatalf("delete status: %v", err)
	}
	var occurrenceStatusID string
	if err := pool.QueryRow(ctx, `SELECT status_id FROM task_occurrence_statuses WHERE user_id = $1 AND task_id = $2`, userID, task.Task.ID).Scan(&occurrenceStatusID); err != nil || occurrenceStatusID != "status-todo" {
		t.Errorf("occurrence status = %q, error = %v", occurrenceStatusID, err)
	}
	if err := service.DeletePriority(ctx, userID, priority.ID); err != nil {
		t.Fatalf("delete priority: %v", err)
	}
	if err := service.DeleteGroup(ctx, userID, group.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	task, err = service.ByID(ctx, userID, task.Task.ID)
	if err != nil || task.Task.StatusID != "status-todo" || task.Task.PriorityID != nil || task.Task.GroupID != nil || task.Priority != nil || task.Group != nil {
		t.Errorf("task after catalog deletes = %#v, error = %v", task, err)
	}
}
