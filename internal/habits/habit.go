package habits

import "time"

type RepeatType string

const (
	RepeatDaily   RepeatType = "daily"
	RepeatWeekly  RepeatType = "weekly"
	RepeatMonthly RepeatType = "monthly"
	RepeatCustom  RepeatType = "custom"
)

type RepeatRule struct {
	Type       RepeatType `json:"type"`
	DaysOfWeek []int      `json:"daysOfWeek,omitempty"`
	DayOfMonth *int       `json:"dayOfMonth,omitempty"`
	EveryNDays *int       `json:"everyNDays,omitempty"`
}

type Habit struct {
	ID              string     `db:"id"`
	UserID          string     `db:"user_id"`
	Title           string     `db:"title"`
	Note            string     `db:"note"`
	Description     string     `db:"description"`
	CategoryID      *string    `db:"category_id"`
	RepeatRule      RepeatRule `db:"repeat_rule"`
	StartMinute     *int16     `db:"start_minute"`
	DurationMinutes int16      `db:"duration_minutes"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

type HabitCategory struct {
	ID        string  `db:"id"`
	UserID    string  `db:"user_id"`
	Label     string  `db:"label"`
	Color     *string `db:"color"`
	SortOrder int     `db:"sort_order"`
}

type HabitCompletion struct {
	ID             string    `db:"id"`
	UserID         string    `db:"user_id"`
	HabitID        string    `db:"habit_id"`
	OccurrenceDate time.Time `db:"occurrence_date"`
	CompletedAt    time.Time `db:"completed_at"`
}

type HabitDetails struct {
	Habit    Habit
	Category *HabitCategory
}
