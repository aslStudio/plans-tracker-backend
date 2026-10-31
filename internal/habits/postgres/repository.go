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

	"plans-tracker-backend/internal/habits"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const habitDetailsSelect = `
	SELECT h.id, h.user_id, h.title, h.note, h.description, h.category_id,
	       h.repeat_rule, h.start_minute, h.duration_minutes, h.created_at, h.updated_at,
	       c.id, c.label, c.color, c.sort_order
	FROM habits AS h
	LEFT JOIN habit_categories AS c
	    ON c.user_id = h.user_id AND c.id = h.category_id
`

func (r *Repository) List(ctx context.Context, userID string) ([]habits.HabitDetails, error) {
	query := habitDetailsSelect + `
		WHERE h.user_id = $1
		ORDER BY h.created_at DESC, h.id DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (habits.HabitDetails, error) {
		return scanHabitDetails(row)
	})
}

func (r *Repository) ByID(ctx context.Context, userID, id string) (habits.HabitDetails, error) {
	query := habitDetailsSelect + `WHERE h.user_id = $1 AND h.id = $2`
	item, err := scanHabitDetails(r.pool.QueryRow(ctx, query, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return habits.HabitDetails{}, habits.ErrNotFound
	}
	return item, err
}

type habitDetailsRow interface {
	Scan(dest ...any) error
}

func scanHabitDetails(row habitDetailsRow) (habits.HabitDetails, error) {
	var item habits.HabitDetails
	var categoryID, categoryLabel, categoryColor *string
	var categorySortOrder *int
	err := row.Scan(
		&item.Habit.ID, &item.Habit.UserID, &item.Habit.Title,
		&item.Habit.Note, &item.Habit.Description, &item.Habit.CategoryID,
		&item.Habit.RepeatRule, &item.Habit.StartMinute,
		&item.Habit.DurationMinutes, &item.Habit.CreatedAt, &item.Habit.UpdatedAt,
		&categoryID, &categoryLabel, &categoryColor, &categorySortOrder,
	)
	if err != nil {
		return habits.HabitDetails{}, err
	}
	if categoryID != nil {
		item.Category = &habits.HabitCategory{
			ID:        *categoryID,
			UserID:    item.Habit.UserID,
			Label:     *categoryLabel,
			Color:     categoryColor,
			SortOrder: *categorySortOrder,
		}
	}
	return item, nil
}

func referenceError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return habits.ErrInvalidReference
	}
	return err
}

func (r *Repository) Create(ctx context.Context, userID string, input habits.HabitInput) (habits.HabitDetails, error) {
	rule, err := json.Marshal(input.RepeatRule)
	if err != nil {
		return habits.HabitDetails{}, err
	}
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO habits (user_id, title, note, description, category_id, repeat_rule, start_minute, duration_minutes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
	`, userID, input.Title, input.Note, input.Description, input.CategoryID, rule, input.StartMinute, input.DurationMinutes).Scan(&id)
	if err != nil {
		return habits.HabitDetails{}, referenceError(err)
	}
	return r.ByID(ctx, userID, id)
}

func (r *Repository) Patch(ctx context.Context, userID, id string, patch habits.HabitPatch) (habits.HabitDetails, error) {
	args := []any{userID, id}
	sets := make([]string, 0, 7)
	add := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
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
	if patch.CategoryID.Set {
		add("category_id", patch.CategoryID.Value)
	}
	if patch.StartMinute.Set {
		add("start_minute", patch.StartMinute.Value)
	}
	if patch.DurationMinutes.Set {
		add("duration_minutes", patch.DurationMinutes.Value)
	}
	if patch.RepeatRule.Set {
		rule, err := json.Marshal(patch.RepeatRule.Value)
		if err != nil {
			return habits.HabitDetails{}, err
		}
		add("repeat_rule", rule)
	}
	var updatedID string
	err := r.pool.QueryRow(ctx, `UPDATE habits SET `+strings.Join(sets, ", ")+` WHERE user_id = $1 AND id = $2 RETURNING id`, args...).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return habits.HabitDetails{}, habits.ErrNotFound
	}
	if err != nil {
		return habits.HabitDetails{}, referenceError(err)
	}
	return r.ByID(ctx, userID, updatedID)
}

func (r *Repository) Delete(ctx context.Context, userID, id string) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM habits WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return habits.ErrNotFound
	}
	return nil
}

func (r *Repository) CategoryList(ctx context.Context, userID string) ([]habits.HabitCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, label, color, sort_order
		FROM habit_categories WHERE user_id = $1 ORDER BY sort_order, id
	`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[habits.HabitCategory])
}

func (r *Repository) CategoryByID(ctx context.Context, userID, id string) (habits.HabitCategory, error) {
	var item habits.HabitCategory
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, label, color, sort_order
		FROM habit_categories WHERE user_id = $1 AND id = $2
	`, userID, id).Scan(&item.ID, &item.UserID, &item.Label, &item.Color, &item.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return habits.HabitCategory{}, habits.ErrNotFound
	}
	return item, err
}

func (r *Repository) CreateCategory(ctx context.Context, userID string, input habits.CategoryInput) (habits.HabitCategory, error) {
	var item habits.HabitCategory
	err := r.pool.QueryRow(ctx, `
		INSERT INTO habit_categories (user_id, label, color, sort_order)
		VALUES ($1, $2, $3, (SELECT COALESCE(MAX(sort_order) + 1, 0) FROM habit_categories WHERE user_id = $1))
		RETURNING id, user_id, label, color, sort_order
	`, userID, input.Label, input.Color).Scan(&item.ID, &item.UserID, &item.Label, &item.Color, &item.SortOrder)
	return item, err
}

func (r *Repository) PatchCategory(ctx context.Context, userID, id string, patch habits.CategoryPatch) (habits.HabitCategory, error) {
	args := []any{userID, id}
	sets := make([]string, 0, 3)
	add := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if patch.Label.Set {
		add("label", patch.Label.Value)
	}
	if patch.Color.Set {
		add("color", patch.Color.Value)
	}
	if patch.SortOrder.Set {
		add("sort_order", patch.SortOrder.Value)
	}
	var item habits.HabitCategory
	err := r.pool.QueryRow(ctx, `UPDATE habit_categories SET `+strings.Join(sets, ", ")+`
		WHERE user_id = $1 AND id = $2 RETURNING id, user_id, label, color, sort_order`, args...).Scan(
		&item.ID, &item.UserID, &item.Label, &item.Color, &item.SortOrder,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return habits.HabitCategory{}, habits.ErrNotFound
	}
	return item, err
}

func (r *Repository) DeleteCategory(ctx context.Context, userID, id string) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM habit_categories WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return habits.ErrNotFound
	}
	return nil
}

func (r *Repository) CompletionList(ctx context.Context, userID string, filter habits.CompletionFilter) ([]habits.HabitCompletion, error) {
	query := `SELECT id, user_id, habit_id, occurrence_date, completed_at FROM habit_completions WHERE user_id = $1`
	args := []any{userID}
	if filter.HabitID != "" {
		args = append(args, filter.HabitID)
		query += fmt.Sprintf(" AND habit_id = $%d", len(args))
	}
	if filter.StartDate != nil {
		args = append(args, *filter.StartDate, *filter.EndDate)
		query += fmt.Sprintf(" AND occurrence_date BETWEEN $%d AND $%d", len(args)-1, len(args))
	}
	query += ` ORDER BY occurrence_date, id`
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[habits.HabitCompletion])
}

func (r *Repository) SetCompletion(ctx context.Context, userID, habitID string, date time.Time, completed bool) (*habits.HabitCompletion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var found int
	err = tx.QueryRow(ctx, `SELECT 1 FROM habits WHERE user_id = $1 AND id = $2 FOR KEY SHARE`, userID, habitID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, habits.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var item *habits.HabitCompletion
	if completed {
		item = &habits.HabitCompletion{}
		err = tx.QueryRow(ctx, `
			INSERT INTO habit_completions (user_id, habit_id, occurrence_date)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id, habit_id, occurrence_date)
			DO UPDATE SET completed_at = clock_timestamp()
			RETURNING id, user_id, habit_id, occurrence_date, completed_at
		`, userID, habitID, date).Scan(&item.ID, &item.UserID, &item.HabitID, &item.OccurrenceDate, &item.CompletedAt)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM habit_completions WHERE user_id = $1 AND habit_id = $2 AND occurrence_date = $3`, userID, habitID, date)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return item, nil
}
