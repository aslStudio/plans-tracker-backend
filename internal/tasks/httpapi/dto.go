package httpapi

import (
	"time"

	"plans-tracker-backend/internal/tasks"
)

type TaskDto struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Note            string            `json:"note"`
	Description     *string           `json:"description"`
	Priority        *TaskPriorityDto  `json:"priority"`
	Group           *TaskGroupDto     `json:"group"`
	StartDate       *string           `json:"start_date"`
	EndDate         *string           `json:"end_date"`
	StartMinute     *int              `json:"start_minute"`
	DurationMinutes *int              `json:"duration_minutes"`
	RepeatRule      *tasks.RepeatRule `json:"repeat_rule"`
	Status          TaskStatusDto     `json:"status"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type TaskPriorityDto struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Color     *string `json:"color"`
	SortOrder int     `json:"sort_order"`
	IsBuiltIn bool    `json:"is_built_in"`
}

type TaskGroupDto struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

type TaskStatusDto struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Color       *string `json:"color"`
	SortOrder   int     `json:"sort_order"`
	IsCompleted bool    `json:"is_completed"`
	IsBuiltIn   bool    `json:"is_built_in"`
}

type TaskListDto struct {
	Items []TaskDto `json:"items"`
}

type TaskOccurrenceDto struct {
	Task           TaskDto       `json:"task"`
	OccurrenceDate string        `json:"occurrence_date"`
	Status         TaskStatusDto `json:"status"`
	Completed      bool          `json:"completed"`
	CompletedAt    *time.Time    `json:"completed_at"`
}

type TaskOccurrenceListDto struct {
	Items []TaskOccurrenceDto `json:"items"`
}

type PriorityListDto struct {
	Items []TaskPriorityDto `json:"items"`
}

type GroupListDto struct {
	Items []TaskGroupDto `json:"items"`
}

type StatusListDto struct {
	Items []TaskStatusDto `json:"items"`
}
