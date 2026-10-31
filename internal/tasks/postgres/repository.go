package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"plans-tracker-backend/internal/tasks"
)

type Repository struct {
	pool *pgxpool.Pool
}

const taskDetailsSelect = `
	SELECT t.id, t.user_id, t.title, t.note, t.description, t.status_id,
	       t.priority_id, t.group_id, t.start_date, t.end_date, t.start_minute,
	       t.duration_minutes, t.repeat_rule, t.created_at, t.updated_at,
	       s.id, s.label, s.color, s.sort_order, s.is_completed, s.is_built_in,
	       p.id, p.label, p.color, p.sort_order, p.is_built_in,
	       g.id, g.label, g.color, g.sort_order
	FROM tasks AS t
	JOIN task_statuses AS s ON s.user_id = t.user_id AND s.id = t.status_id
	LEFT JOIN task_priorities AS p ON p.user_id = t.user_id AND p.id = t.priority_id
	LEFT JOIN task_groups AS g ON g.user_id = t.user_id AND g.id = t.group_id
`

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(
	ctx context.Context,
	userID string,
	startDate time.Time,
	endDate time.Time,
) ([]tasks.TaskDetails, error) {
	query := taskDetailsSelect + `
		WHERE t.user_id = $1 AND (
		  (t.repeat_rule IS NULL AND t.start_date <= $3 AND t.end_date >= $2)
		  OR (t.repeat_rule IS NOT NULL AND COALESCE(t.start_date, (t.created_at AT TIME ZONE 'UTC')::date) <= $3)
		)
		ORDER BY t.created_at DESC, t.id DESC
	`

	rows, err := r.pool.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (tasks.TaskDetails, error) {
		return scanTaskDetails(row)
	})
}

func (r *Repository) Backlog(ctx context.Context, userID string) ([]tasks.TaskDetails, error) {
	query := taskDetailsSelect + `
		WHERE t.user_id = $1 AND t.start_date IS NULL AND t.end_date IS NULL AND t.repeat_rule IS NULL
		ORDER BY t.created_at DESC, t.id DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (tasks.TaskDetails, error) { return scanTaskDetails(row) })
}

func (r *Repository) ByID(ctx context.Context, userID, id string) (tasks.TaskDetails, error) {
	query := taskDetailsSelect + `WHERE t.user_id = $1 AND t.id = $2`
	item, err := scanTaskDetails(r.pool.QueryRow(ctx, query, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return tasks.TaskDetails{}, tasks.ErrNotFound
	}
	return item, err
}

type taskDetailsRow interface {
	Scan(dest ...any) error
}

func scanTaskDetails(row taskDetailsRow) (tasks.TaskDetails, error) {
	var item tasks.TaskDetails
	var priorityID, priorityLabel, priorityColor, groupID, groupLabel, groupColor *string
	var prioritySortOrder, groupSortOrder *int
	var priorityIsBuiltIn *bool
	err := row.Scan(
		&item.Task.ID, &item.Task.UserID, &item.Task.Title, &item.Task.Note,
		&item.Task.Description, &item.Task.StatusID, &item.Task.PriorityID,
		&item.Task.GroupID, &item.Task.StartDate, &item.Task.EndDate,
		&item.Task.StartMinute, &item.Task.DurationMinutes, &item.Task.RepeatRule,
		&item.Task.CreatedAt, &item.Task.UpdatedAt,
		&item.Status.ID, &item.Status.Label, &item.Status.Color,
		&item.Status.SortOrder, &item.Status.IsCompleted, &item.Status.IsBuiltIn,
		&priorityID, &priorityLabel, &priorityColor, &prioritySortOrder, &priorityIsBuiltIn,
		&groupID, &groupLabel, &groupColor, &groupSortOrder,
	)
	if err != nil {
		return item, err
	}
	if priorityID != nil {
		item.Priority = &tasks.TaskPriority{
			ID: *priorityID, Label: *priorityLabel, Color: priorityColor,
			SortOrder: *prioritySortOrder, IsBuiltIn: *priorityIsBuiltIn,
		}
	}
	if groupID != nil {
		item.Group = &tasks.TaskGroup{
			ID: *groupID, Label: *groupLabel, Color: *groupColor, SortOrder: *groupSortOrder,
		}
	}
	return item, nil
}

func (r *Repository) PriorityList(
	ctx context.Context,
	userID string,
) ([]tasks.TaskPriority, error) {
	query := `
		SELECT id, label, color, sort_order, is_built_in
		FROM task_priorities
		WHERE user_id = $1
		ORDER BY sort_order, id
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[tasks.TaskPriority])
}

func (r *Repository) PriorityById(
	ctx context.Context,
	userID string,
	id string,
) (tasks.TaskPriority, error) {
	query := `
		SELECT id, label, color, sort_order, is_built_in
		FROM task_priorities
		WHERE user_id = $1 AND id = $2
	`

	var priority tasks.TaskPriority
	err := r.pool.QueryRow(ctx, query, userID, id).Scan(
		&priority.ID, &priority.Label, &priority.Color,
		&priority.SortOrder, &priority.IsBuiltIn,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.TaskPriority{}, tasks.ErrNotFound
		}
		return tasks.TaskPriority{}, err
	}

	return priority, nil
}

func (r *Repository) StatusList(
	ctx context.Context,
	userID string,
) ([]tasks.TaskStatus, error) {
	query := `
		SELECT id, label, color, sort_order, is_completed, is_built_in
		FROM task_statuses
		WHERE user_id = $1
		ORDER BY sort_order, id
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[tasks.TaskStatus])
}

func (r *Repository) StatusById(
	ctx context.Context,
	userID string,
	id string,
) (tasks.TaskStatus, error) {
	query := `
		SELECT id, label, color, sort_order, is_completed, is_built_in
		FROM task_statuses
		WHERE user_id = $1 AND id = $2
	`

	var status tasks.TaskStatus
	err := r.pool.QueryRow(ctx, query, userID, id).Scan(
		&status.ID, &status.Label, &status.Color,
		&status.SortOrder, &status.IsCompleted, &status.IsBuiltIn,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.TaskStatus{}, tasks.ErrNotFound
		}
		return tasks.TaskStatus{}, err
	}

	return status, nil
}

func (r *Repository) GroupList(
	ctx context.Context,
	userID string,
) ([]tasks.TaskGroup, error) {
	query := `
		SELECT id, label, color, sort_order
		FROM task_groups
		WHERE user_id = $1
		ORDER BY sort_order, id
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[tasks.TaskGroup])
}

func (r *Repository) GroupById(
	ctx context.Context,
	userID string,
	id string,
) (tasks.TaskGroup, error) {
	query := `
		SELECT id, label, color, sort_order
		FROM task_groups
		WHERE user_id = $1 AND id = $2
	`

	var group tasks.TaskGroup
	err := r.pool.QueryRow(ctx, query, userID, id).Scan(
		&group.ID, &group.Label, &group.Color, &group.SortOrder,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.TaskGroup{}, tasks.ErrNotFound
		}
		return tasks.TaskGroup{}, err
	}

	return group, nil
}

func (r *Repository) Create(ctx context.Context, userID string, input tasks.TaskWrite) (tasks.TaskDetails, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tasks.TaskDetails{}, err
	}
	defer tx.Rollback(ctx)

	repeatRule, err := repeatRuleJSON(input.RepeatRule)
	if err != nil {
		return tasks.TaskDetails{}, err
	}
	const query = `
		INSERT INTO tasks (
			user_id, title, note, description, status_id, priority_id, group_id,
			start_date, end_date, start_minute, duration_minutes, repeat_rule
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`
	var id string
	err = tx.QueryRow(ctx, query,
		userID, input.Title, input.Note, input.Description, input.StatusID,
		input.PriorityID, input.GroupID, input.StartDate, input.EndDate,
		input.StartMinute, input.DurationMinutes, repeatRule,
	).Scan(&id)
	if err != nil {
		return tasks.TaskDetails{}, taskWriteError(err)
	}
	item, err := scanTaskDetails(tx.QueryRow(ctx, taskDetailsSelect+`WHERE t.user_id = $1 AND t.id = $2`, userID, id))
	if err != nil {
		return tasks.TaskDetails{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tasks.TaskDetails{}, err
	}
	return item, nil
}

func (r *Repository) Patch(ctx context.Context, userID, id string, patch tasks.TaskPatch) (tasks.TaskDetails, error) {
	set := []string{"updated_at = now()"}
	args := []any{userID, id}
	add := func(column string, value any) {
		args = append(args, value)
		set = append(set, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if patch.Title.Set {
		add("title", patch.Title.Value)
	}
	if patch.Note.Set {
		add("note", patch.Note.Value)
	}
	if patch.Description.Set {
		add("description", patch.Description.Value)
	}
	if patch.StatusID.Set {
		add("status_id", patch.StatusID.Value)
	}
	if patch.PriorityID.Set {
		add("priority_id", patch.PriorityID.Value)
	}
	if patch.GroupID.Set {
		add("group_id", patch.GroupID.Value)
	}
	if patch.StartDate.Set {
		add("start_date", patch.StartDate.Value)
		add("end_date", patch.EndDate.Value)
	}
	if patch.StartMinute.Set {
		add("start_minute", patch.StartMinute.Value)
	}
	if patch.DurationMinutes.Set {
		add("duration_minutes", patch.DurationMinutes.Value)
	}
	if patch.RepeatRule.Set {
		rule, err := repeatRuleJSON(patch.RepeatRule.Value)
		if err != nil {
			return tasks.TaskDetails{}, err
		}
		add("repeat_rule", rule)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tasks.TaskDetails{}, err
	}
	defer tx.Rollback(ctx)
	query := `UPDATE tasks SET ` + strings.Join(set, ", ") + ` WHERE user_id = $1 AND id = $2 RETURNING id`
	var updatedID string
	if err := tx.QueryRow(ctx, query, args...).Scan(&updatedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tasks.TaskDetails{}, tasks.ErrNotFound
		}
		return tasks.TaskDetails{}, taskWriteError(err)
	}
	if patch.StartDate.Set || patch.RepeatRule.Set {
		if _, err := tx.Exec(ctx, `DELETE FROM task_completions WHERE user_id = $1 AND task_id = $2`, userID, id); err != nil {
			return tasks.TaskDetails{}, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM task_occurrence_statuses WHERE user_id = $1 AND task_id = $2`, userID, id); err != nil {
			return tasks.TaskDetails{}, err
		}
	}
	item, err := scanTaskDetails(tx.QueryRow(ctx, taskDetailsSelect+`WHERE t.user_id = $1 AND t.id = $2`, userID, updatedID))
	if err != nil {
		return tasks.TaskDetails{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tasks.TaskDetails{}, err
	}
	return item, nil
}

func (r *Repository) Delete(ctx context.Context, userID, id string) error {
	command, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return tasks.ErrNotFound
	}
	return nil
}

func repeatRuleJSON(rule *tasks.RepeatRule) ([]byte, error) {
	if rule == nil {
		return nil, nil
	}
	return json.Marshal(rule)
}

func taskWriteError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503":
			return tasks.ErrInvalidReference
		case "23502", "23514":
			return tasks.ErrInvalidTask
		}
	}
	return err
}

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
