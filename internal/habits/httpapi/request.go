package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"plans-tracker-backend/internal/habits"
)

type habitRequest struct {
	Title           habits.Field[string]            `json:"title"`
	Note            habits.Field[string]            `json:"note"`
	Description     habits.Field[string]            `json:"description"`
	CategoryID      habits.Field[*string]           `json:"category_id"`
	RepeatRule      habits.Field[habits.RepeatRule] `json:"repeat_rule"`
	StartMinute     habits.Field[*int16]            `json:"start_minute"`
	DurationMinutes habits.Field[int16]             `json:"duration_minutes"`
}

func (request habitRequest) toCreateInput() (habits.HabitInput, error) {
	if !request.Title.Set || !request.RepeatRule.Set || request.Title.Null ||
		request.RepeatRule.Null || request.Note.Null || request.Description.Null || request.DurationMinutes.Null {
		return habits.HabitInput{}, habits.ErrInvalidInput
	}
	input := habits.HabitInput{
		Title: request.Title.Value, RepeatRule: request.RepeatRule.Value,
		CategoryID: request.CategoryID.Value, StartMinute: request.StartMinute.Value,
		DurationMinutes: 60,
	}
	if request.Note.Set {
		input.Note = request.Note.Value
	}
	if request.Description.Set {
		input.Description = request.Description.Value
	}
	if request.DurationMinutes.Set {
		input.DurationMinutes = request.DurationMinutes.Value
	}
	return input, nil
}

func (request habitRequest) toPatch() habits.HabitPatch {
	return habits.HabitPatch{
		Title: request.Title, Note: request.Note, Description: request.Description,
		CategoryID: request.CategoryID, RepeatRule: request.RepeatRule,
		StartMinute: request.StartMinute, DurationMinutes: request.DurationMinutes,
	}
}

type categoryRequest struct {
	Label     habits.Field[string]  `json:"label"`
	Color     habits.Field[*string] `json:"color"`
	SortOrder habits.Field[int]     `json:"sort_order"`
}

func (request categoryRequest) toCreateInput() (habits.CategoryInput, error) {
	if !request.Label.Set || request.Label.Null || request.SortOrder.Set {
		return habits.CategoryInput{}, habits.ErrInvalidInput
	}
	return habits.CategoryInput{Label: request.Label.Value, Color: request.Color.Value}, nil
}

func (request categoryRequest) toPatch() habits.CategoryPatch {
	return habits.CategoryPatch{Label: request.Label, Color: request.Color, SortOrder: request.SortOrder}
}

type completionRequest struct {
	Completed habits.Field[bool] `json:"completed"`
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return habits.ErrInvalidInput
	}
	return nil
}
