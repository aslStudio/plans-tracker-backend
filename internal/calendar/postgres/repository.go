package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"plans-tracker-backend/internal/calendar"
)

type Repository struct{ Pool *pgxpool.Pool }

const columns = `id, title, to_char(event_date, 'YYYY-MM-DD'), start_minute, duration_minutes,
	all_day, color, description, notes, location, repeat_rule,
	CASE WHEN repeat_until IS NULL THEN NULL ELSE to_char(repeat_until, 'YYYY-MM-DD') END,
	ARRAY(SELECT to_char(d, 'YYYY-MM-DD') FROM unnest(excluded_dates) AS d ORDER BY d),
	linked_type, linked_id, created_at, updated_at`

type row interface{ Scan(...any) error }

func scan(r row) (calendar.Event, error) {
	var e calendar.Event
	var rule []byte
	var linkType, linkID *string
	err := r.Scan(&e.ID, &e.Title, &e.Date, &e.StartMinute, &e.DurationMinutes,
		&e.AllDay, &e.Color, &e.Description, &e.Notes, &e.Location, &rule,
		&e.RepeatUntil, &e.ExcludedDates, &linkType, &linkID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return e, err
	}
	if len(rule) != 0 {
		e.RepeatRule = new(calendar.RepeatRule)
		if err := json.Unmarshal(rule, e.RepeatRule); err != nil {
			return e, err
		}
	}
	if linkType != nil && linkID != nil {
		e.Link = &calendar.Link{Type: *linkType, ID: *linkID}
	}
	if e.ExcludedDates == nil {
		e.ExcludedDates = []string{}
	}
	return e, nil
}

func (r Repository) ByID(ctx context.Context, user, id string) (calendar.Event, error) {
	e, err := scan(r.Pool.QueryRow(ctx, `SELECT `+columns+` FROM calendar_events WHERE user_id=$1 AND id=$2`, user, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return e, calendar.ErrNotFound
	}
	return e, err
}
func (r Repository) List(ctx context.Context, user string, start, end time.Time) ([]calendar.Event, error) {
	rows, err := r.Pool.Query(ctx, `SELECT `+columns+` FROM calendar_events
		WHERE user_id=$1 AND event_date <= $3 AND (repeat_rule IS NOT NULL OR event_date >= $2)
		AND (repeat_until IS NULL OR repeat_until >= $2) ORDER BY event_date, id`, user, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []calendar.Event{}
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		if calendar.VisibleInRange(e.EventInput, start, end) {
			items = append(items, e)
		}
	}
	return items, rows.Err()
}

func args(user string, e calendar.EventInput) ([]any, error) {
	date, err := calendar.ParseDate(e.Date)
	if err != nil {
		return nil, err
	}
	var rule any
	if e.RepeatRule != nil {
		data, err := json.Marshal(e.RepeatRule)
		if err != nil {
			return nil, err
		}
		rule = data
	}
	var linkType, linkID any
	if e.Link != nil {
		linkType, linkID = e.Link.Type, e.Link.ID
	}
	var until *time.Time
	if e.RepeatUntil != nil {
		value, err := calendar.ParseDate(*e.RepeatUntil)
		if err != nil {
			return nil, err
		}
		until = &value
	}
	exclusions := make([]time.Time, len(e.ExcludedDates))
	for i, value := range e.ExcludedDates {
		parsed, err := calendar.ParseDate(value)
		if err != nil {
			return nil, err
		}
		exclusions[i] = parsed
	}
	return []any{user, e.Title, date, e.StartMinute, e.DurationMinutes, e.AllDay, e.Color,
		e.Description, e.Notes, e.Location, rule, until, exclusions, linkType, linkID}, nil
}
func (r Repository) Create(ctx context.Context, user string, input calendar.EventInput) (calendar.Event, error) {
	params, err := args(user, input)
	if err != nil {
		return calendar.Event{}, err
	}
	var id string
	err = r.Pool.QueryRow(ctx, `INSERT INTO calendar_events
		(user_id,title,event_date,start_minute,duration_minutes,all_day,color,description,notes,location,repeat_rule,repeat_until,excluded_dates,linked_type,linked_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::date[],$14,$15) RETURNING id`, params...).Scan(&id)
	if err != nil {
		return calendar.Event{}, err
	}
	return r.ByID(ctx, user, id)
}
func (r Repository) Replace(ctx context.Context, user, id string, input calendar.EventInput) (calendar.Event, error) {
	params, err := args(user, input)
	if err != nil {
		return calendar.Event{}, err
	}
	params = append(params, id)
	var updated string
	err = r.Pool.QueryRow(ctx, `UPDATE calendar_events SET
		title=$2,event_date=$3,start_minute=$4,duration_minutes=$5,all_day=$6,color=$7,
		description=$8,notes=$9,location=$10,repeat_rule=$11,repeat_until=$12,
		excluded_dates=$13::date[],linked_type=$14,linked_id=$15,updated_at=clock_timestamp()
		WHERE user_id=$1 AND id=$16 RETURNING id`, params...).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return calendar.Event{}, calendar.ErrNotFound
	}
	if err != nil {
		return calendar.Event{}, err
	}
	return r.ByID(ctx, user, updated)
}
func (r Repository) Delete(ctx context.Context, user, id string) error {
	result, err := r.Pool.Exec(ctx, `DELETE FROM calendar_events WHERE user_id=$1 AND id=$2`, user, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return calendar.ErrNotFound
	}
	return nil
}
func (r Repository) Exclude(ctx context.Context, user, id, date string, exclude bool) (calendar.Event, error) {
	parsed, err := calendar.ParseDate(date)
	if err != nil {
		return calendar.Event{}, err
	}
	statement := `UPDATE calendar_events SET excluded_dates=array_remove(excluded_dates,$3::date),updated_at=clock_timestamp() WHERE user_id=$1 AND id=$2 RETURNING id`
	if exclude {
		statement = `UPDATE calendar_events SET excluded_dates=CASE WHEN $3::date=ANY(excluded_dates) THEN excluded_dates ELSE array_append(excluded_dates,$3::date) END,updated_at=clock_timestamp() WHERE user_id=$1 AND id=$2 RETURNING id`
	}
	var updated string
	err = r.Pool.QueryRow(ctx, statement, user, id, parsed).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return calendar.Event{}, calendar.ErrNotFound
	}
	if err != nil {
		return calendar.Event{}, err
	}
	return r.ByID(ctx, user, updated)
}
func (r Repository) Settings(ctx context.Context, user string) (calendar.Settings, error) {
	var s calendar.Settings
	if _, err := r.Pool.Exec(ctx, `INSERT INTO calendar_settings (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, user); err != nil {
		return s, err
	}
	var err error
	err = r.Pool.QueryRow(ctx, `SELECT show_tasks,show_habits,view_mode FROM calendar_settings WHERE user_id=$1`, user).Scan(&s.ShowTasks, &s.ShowHabits, &s.ViewMode)
	return s, err
}
func (r Repository) PatchSettings(ctx context.Context, user string, patch calendar.SettingsPatch) (calendar.Settings, error) {
	args := []any{user}
	sets := []string{}
	if patch.ShowTasks != nil {
		args = append(args, *patch.ShowTasks)
		sets = append(sets, "show_tasks=$"+strconv.Itoa(len(args)))
	}
	if patch.ShowHabits != nil {
		args = append(args, *patch.ShowHabits)
		sets = append(sets, "show_habits=$"+strconv.Itoa(len(args)))
	}
	if patch.ViewMode != nil {
		args = append(args, *patch.ViewMode)
		sets = append(sets, "view_mode=$"+strconv.Itoa(len(args)))
	}
	var s calendar.Settings
	query := `INSERT INTO calendar_settings (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`
	if _, err := r.Pool.Exec(ctx, query, user); err != nil {
		return s, err
	}
	err := r.Pool.QueryRow(ctx, `UPDATE calendar_settings SET `+strings.Join(sets, ",")+`,updated_at=clock_timestamp() WHERE user_id=$1 RETURNING show_tasks,show_habits,view_mode`, args...).Scan(&s.ShowTasks, &s.ShowHabits, &s.ViewMode)
	return s, err
}
