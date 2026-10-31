package httpapi

import (
	"time"

	"plans-tracker-backend/internal/habits"
)

type habitDTO struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Note            string            `json:"note"`
	Description     string            `json:"description"`
	CategoryID      *string           `json:"category_id"`
	RepeatRule      habits.RepeatRule `json:"repeat_rule"`
	StartMinute     *int16            `json:"start_minute"`
	DurationMinutes int16             `json:"duration_minutes"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type habitListDTO struct {
	Items []habitDTO `json:"items"`
}

type categoryDTO struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Color     *string `json:"color"`
	SortOrder int     `json:"sort_order"`
}

type categoryListDTO struct {
	Items []categoryDTO `json:"items"`
}

type completionDTO struct {
	ID             string    `json:"id"`
	HabitID        string    `json:"habit_id"`
	OccurrenceDate string    `json:"occurrence_date"`
	CompletedAt    time.Time `json:"completed_at"`
}

type completionListDTO struct {
	Items []completionDTO `json:"items"`
}

type completionStateDTO struct {
	Completed  bool           `json:"completed"`
	Completion *completionDTO `json:"completion"`
}

func habitToDTO(details habits.HabitDetails) habitDTO {
	habit := details.Habit
	return habitDTO{
		ID:              habit.ID,
		Title:           habit.Title,
		Note:            habit.Note,
		Description:     habit.Description,
		CategoryID:      habit.CategoryID,
		RepeatRule:      habit.RepeatRule,
		StartMinute:     habit.StartMinute,
		DurationMinutes: habit.DurationMinutes,
		CreatedAt:       habit.CreatedAt,
		UpdatedAt:       habit.UpdatedAt,
	}
}

func categoryToDTO(category habits.HabitCategory) categoryDTO {
	return categoryDTO{ID: category.ID, Label: category.Label, Color: category.Color, SortOrder: category.SortOrder}
}

func completionToDTO(completion habits.HabitCompletion) completionDTO {
	return completionDTO{
		ID: completion.ID, HabitID: completion.HabitID,
		OccurrenceDate: completion.OccurrenceDate.Format("2006-01-02"),
		CompletedAt:    completion.CompletedAt,
	}
}
