package tasks

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

type Task struct {
	ID              string      `db:"id"`
	UserID          string      `db:"user_id"`
	Title           string      `db:"title"`
	Note            string      `db:"note"`
	Description     *string     `db:"description"`
	PriorityID      *string     `db:"priority_id"`
	GroupID         *string     `db:"group_id"`
	StartDate       *time.Time  `db:"start_date"`
	EndDate         *time.Time  `db:"end_date"`
	StartMinute     *int16      `db:"start_minute"`
	DurationMinutes int16       `db:"duration_minutes"`
	RepeatRule      *RepeatRule `db:"repeat_rule"`
	StatusID        string      `db:"status_id"`
	CreatedAt       time.Time   `db:"created_at"`
	UpdatedAt       time.Time   `db:"updated_at"`
}

type TaskDetails struct {
	Task     Task
	Status   TaskStatus
	Priority *TaskPriority
	Group    *TaskGroup
}

type TaskPriority struct {
	ID        string  `db:"id"`
	Label     string  `db:"label"`
	Color     *string `db:"color"`
	SortOrder int     `db:"sort_order"`
	IsBuiltIn bool    `db:"is_built_in"`
}

type TaskGroup struct {
	ID        string `db:"id"`
	Label     string `db:"label"`
	Color     string `db:"color"`
	SortOrder int    `db:"sort_order"`
}

type TaskStatus struct {
	ID          string  `db:"id"`
	Label       string  `db:"label"`
	Color       *string `db:"color"`
	SortOrder   int     `db:"sort_order"`
	IsCompleted bool    `db:"is_completed"`
	IsBuiltIn   bool    `db:"is_built_in"`
}
