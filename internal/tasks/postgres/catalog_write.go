package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"plans-tracker-backend/internal/tasks"
)

func (r *Repository) CreateStatus(ctx context.Context, userID string, input tasks.StatusInput) (tasks.TaskStatus, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tasks.TaskStatus{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT ensure_task_defaults($1)`, userID); err != nil {
		return tasks.TaskStatus{}, err
	}
	var position int
	if err := tx.QueryRow(ctx, `SELECT sort_order FROM task_statuses WHERE user_id = $1 AND id = 'status-done' FOR UPDATE`, userID).Scan(&position); err != nil {
		return tasks.TaskStatus{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE task_statuses SET sort_order = sort_order + 1 WHERE user_id = $1 AND id = 'status-done'`, userID); err != nil {
		return tasks.TaskStatus{}, err
	}
	var item tasks.TaskStatus
	err = tx.QueryRow(ctx, `
		INSERT INTO task_statuses (user_id, label, color, sort_order, is_completed)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, label, color, sort_order, is_completed, is_built_in
	`, userID, input.Label, input.Color, position, input.IsCompleted).Scan(
		&item.ID, &item.Label, &item.Color, &item.SortOrder, &item.IsCompleted, &item.IsBuiltIn,
	)
	if err != nil {
		return tasks.TaskStatus{}, taskWriteError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return tasks.TaskStatus{}, err
	}
	return item, nil
}

func (r *Repository) PatchStatus(ctx context.Context, userID, id string, patch tasks.StatusPatch) (tasks.TaskStatus, error) {
	set, args := []string{}, []any{userID, id}
	if patch.Label.Set {
		set, args = addCatalogSet(set, args, "label", patch.Label.Value)
	}
	if patch.Color.Set {
		set, args = addCatalogSet(set, args, "color", patch.Color.Value)
	}
	if patch.IsCompleted.Set {
		set, args = addCatalogSet(set, args, "is_completed", patch.IsCompleted.Value)
	}
	query := `UPDATE task_statuses SET ` + strings.Join(set, ", ") + ` WHERE user_id = $1 AND id = $2`
	if patch.IsCompleted.Set {
		query += fmt.Sprintf(` AND (NOT is_built_in OR is_completed = $%d)`, len(args))
	}
	query += ` RETURNING id, label, color, sort_order, is_completed, is_built_in`
	var item tasks.TaskStatus
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&item.ID, &item.Label, &item.Color, &item.SortOrder, &item.IsCompleted, &item.IsBuiltIn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		if patch.IsCompleted.Set {
			if _, lookupErr := r.StatusById(ctx, userID, id); lookupErr == nil {
				return tasks.TaskStatus{}, tasks.ErrImmutable
			} else if !errors.Is(lookupErr, tasks.ErrNotFound) {
				return tasks.TaskStatus{}, lookupErr
			}
		}
		return tasks.TaskStatus{}, tasks.ErrNotFound
	}
	if err != nil {
		return tasks.TaskStatus{}, taskWriteError(err)
	}
	return item, nil
}

func (r *Repository) DeleteStatus(ctx context.Context, userID, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var builtIn bool
	if err := tx.QueryRow(ctx, `SELECT is_built_in FROM task_statuses WHERE user_id = $1 AND id = $2 FOR UPDATE`, userID, id).Scan(&builtIn); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.ErrNotFound
		}
		return err
	}
	if builtIn {
		return tasks.ErrProtected
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_completions AS c USING task_occurrence_statuses AS os
		WHERE c.user_id = $1 AND os.user_id = c.user_id AND os.task_id = c.task_id
		AND os.occurrence_date = c.occurrence_date AND os.status_id = $2`, userID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE tasks SET status_id = 'status-todo', updated_at = now() WHERE user_id = $1 AND status_id = $2`, userID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE task_occurrence_statuses SET status_id = 'status-todo' WHERE user_id = $1 AND status_id = $2`, userID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_statuses WHERE user_id = $1 AND id = $2`, userID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) CreatePriority(ctx context.Context, userID string, input tasks.PriorityInput) (tasks.TaskPriority, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tasks.TaskPriority{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT ensure_task_defaults($1)`, userID); err != nil {
		return tasks.TaskPriority{}, err
	}
	var item tasks.TaskPriority
	err = tx.QueryRow(ctx, `
		INSERT INTO task_priorities (user_id, label, color, sort_order)
		SELECT $1, $2, $3, COALESCE(MAX(sort_order) + 1, 0)
		FROM task_priorities WHERE user_id = $1
		RETURNING id, label, color, sort_order, is_built_in
	`, userID, input.Label, input.Color).Scan(
		&item.ID, &item.Label, &item.Color, &item.SortOrder, &item.IsBuiltIn,
	)
	if err != nil {
		return tasks.TaskPriority{}, taskWriteError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return tasks.TaskPriority{}, err
	}
	return item, nil
}

func (r *Repository) PatchPriority(ctx context.Context, userID, id string, patch tasks.PriorityPatch) (tasks.TaskPriority, error) {
	set, args := []string{}, []any{userID, id}
	if patch.Label.Set {
		set, args = addCatalogSet(set, args, "label", patch.Label.Value)
	}
	if patch.Color.Set {
		set, args = addCatalogSet(set, args, "color", patch.Color.Value)
	}
	query := `UPDATE task_priorities SET ` + strings.Join(set, ", ") + ` WHERE user_id = $1 AND id = $2 RETURNING id, label, color, sort_order, is_built_in`
	var item tasks.TaskPriority
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&item.ID, &item.Label, &item.Color, &item.SortOrder, &item.IsBuiltIn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return tasks.TaskPriority{}, tasks.ErrNotFound
	}
	if err != nil {
		return tasks.TaskPriority{}, taskWriteError(err)
	}
	return item, nil
}

func (r *Repository) DeletePriority(ctx context.Context, userID, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var builtIn bool
	if err := tx.QueryRow(ctx, `SELECT is_built_in FROM task_priorities WHERE user_id = $1 AND id = $2 FOR UPDATE`, userID, id).Scan(&builtIn); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.ErrNotFound
		}
		return err
	}
	if builtIn {
		return tasks.ErrProtected
	}
	if _, err := tx.Exec(ctx, `UPDATE tasks SET priority_id = NULL, updated_at = now() WHERE user_id = $1 AND priority_id = $2`, userID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_priorities WHERE user_id = $1 AND id = $2`, userID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) CreateGroup(ctx context.Context, userID string, input tasks.GroupInput) (tasks.TaskGroup, error) {
	var item tasks.TaskGroup
	err := r.pool.QueryRow(ctx, `
		INSERT INTO task_groups (user_id, label, color, sort_order)
		SELECT $1, $2, $3, COALESCE(MAX(sort_order) + 1, 0)
		FROM task_groups WHERE user_id = $1
		RETURNING id, label, color, sort_order
	`, userID, input.Label, input.Color).Scan(
		&item.ID, &item.Label, &item.Color, &item.SortOrder,
	)
	if err != nil {
		return tasks.TaskGroup{}, taskWriteError(err)
	}
	return item, nil
}

func (r *Repository) PatchGroup(ctx context.Context, userID, id string, patch tasks.GroupPatch) (tasks.TaskGroup, error) {
	set, args := []string{}, []any{userID, id}
	if patch.Label.Set {
		set, args = addCatalogSet(set, args, "label", patch.Label.Value)
	}
	if patch.Color.Set {
		set, args = addCatalogSet(set, args, "color", patch.Color.Value)
	}
	query := `UPDATE task_groups SET ` + strings.Join(set, ", ") + ` WHERE user_id = $1 AND id = $2 RETURNING id, label, color, sort_order`
	var item tasks.TaskGroup
	err := r.pool.QueryRow(ctx, query, args...).Scan(&item.ID, &item.Label, &item.Color, &item.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return tasks.TaskGroup{}, tasks.ErrNotFound
	}
	if err != nil {
		return tasks.TaskGroup{}, taskWriteError(err)
	}
	return item, nil
}

func (r *Repository) DeleteGroup(ctx context.Context, userID, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var groupID string
	if err := tx.QueryRow(ctx, `SELECT id FROM task_groups WHERE user_id = $1 AND id = $2 FOR UPDATE`, userID, id).Scan(&groupID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.ErrNotFound
		}
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE tasks SET group_id = NULL, updated_at = now() WHERE user_id = $1 AND group_id = $2`, userID, groupID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_groups WHERE user_id = $1 AND id = $2`, userID, groupID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func addCatalogSet(set []string, args []any, column string, value any) ([]string, []any) {
	args = append(args, value)
	set = append(set, fmt.Sprintf("%s = $%d", column, len(args)))
	return set, args
}
