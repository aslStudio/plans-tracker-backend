package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"plans-tracker-backend/internal/tasks"
)

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
