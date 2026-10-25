package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"plans-tracker-backend/internal/tasks"
)

func (r *Repository) OccurrenceStates(ctx context.Context, userID string, start, end time.Time) (map[tasks.OccurrenceKey]tasks.OccurrenceState, error) {
	states := make(map[tasks.OccurrenceKey]tasks.OccurrenceState)
	rows, err := r.pool.Query(ctx, `SELECT task_id, occurrence_date, status_id FROM task_occurrence_statuses WHERE user_id = $1 AND occurrence_date BETWEEN $2 AND $3`, userID, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var taskID, statusID string
		var date time.Time
		if err := rows.Scan(&taskID, &date, &statusID); err != nil {
			rows.Close()
			return nil, err
		}
		states[tasks.OccurrenceKey{TaskID: taskID, Date: date.Format(time.DateOnly)}] = tasks.OccurrenceState{StatusID: statusID}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = r.pool.Query(ctx, `SELECT task_id, occurrence_date, completed_at FROM task_completions WHERE user_id = $1 AND occurrence_date BETWEEN $2 AND $3`, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID string
		var date, completedAt time.Time
		if err := rows.Scan(&taskID, &date, &completedAt); err != nil {
			return nil, err
		}
		key := tasks.OccurrenceKey{TaskID: taskID, Date: date.Format(time.DateOnly)}
		state := states[key]
		state.CompletedAt = &completedAt
		states[key] = state
	}
	return states, rows.Err()
}

func (r *Repository) SetOccurrenceStatus(ctx context.Context, userID, taskID string, date time.Time, statusID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var isCompleted bool
	err = tx.QueryRow(ctx, `SELECT is_completed FROM task_statuses WHERE user_id = $1 AND id = $2`, userID, statusID).Scan(&isCompleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return tasks.ErrInvalidReference
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO task_occurrence_statuses (user_id, task_id, occurrence_date, status_id) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, task_id, occurrence_date) DO UPDATE SET status_id = EXCLUDED.status_id`, userID, taskID, date, statusID)
	if err != nil {
		return err
	}
	if isCompleted {
		_, err = tx.Exec(ctx, `INSERT INTO task_completions (user_id, task_id, occurrence_date) VALUES ($1, $2, $3)
			ON CONFLICT (user_id, task_id, occurrence_date) DO NOTHING`, userID, taskID, date)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM task_completions WHERE user_id = $1 AND task_id = $2 AND occurrence_date = $3`, userID, taskID, date)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) SetOccurrenceCompletion(ctx context.Context, userID, taskID string, date time.Time, completed bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if completed {
		_, err = tx.Exec(ctx, `INSERT INTO task_completions (user_id, task_id, occurrence_date) VALUES ($1, $2, $3)
			ON CONFLICT (user_id, task_id, occurrence_date) DO NOTHING`, userID, taskID, date)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM task_completions WHERE user_id = $1 AND task_id = $2 AND occurrence_date = $3`, userID, taskID, date)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE task_occurrence_statuses AS os SET status_id = 'status-todo'
				FROM task_statuses AS s WHERE os.user_id = $1 AND os.task_id = $2 AND os.occurrence_date = $3
				AND s.user_id = os.user_id AND s.id = os.status_id AND s.is_completed`, userID, taskID, date)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
