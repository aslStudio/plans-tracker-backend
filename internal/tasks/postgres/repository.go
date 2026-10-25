package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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
