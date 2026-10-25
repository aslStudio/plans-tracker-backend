package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"plans-tracker-backend/internal/tasks"
)

type createTaskRequest struct {
	Title           tasks.Change[string]            `json:"title"`
	Note            tasks.Change[string]            `json:"note"`
	Description     tasks.Change[*string]           `json:"description"`
	StatusID        tasks.Change[string]            `json:"status_id"`
	PriorityID      tasks.Change[*string]           `json:"priority_id"`
	GroupID         tasks.Change[*string]           `json:"group_id"`
	StartDate       tasks.Change[*string]           `json:"start_date"`
	EndDate         tasks.Change[*string]           `json:"end_date"`
	StartMinute     tasks.Change[*int]              `json:"start_minute"`
	DurationMinutes tasks.Change[int]               `json:"duration_minutes"`
	RepeatRule      tasks.Change[*tasks.RepeatRule] `json:"repeat_rule"`
}

type patchTaskRequest createTaskRequest

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid JSON: expected one object")
	}
	return nil
}

func (request createTaskRequest) toWrite() (tasks.TaskWrite, error) {
	if !request.Title.Set || request.Title.Null {
		return tasks.TaskWrite{}, fmt.Errorf("title is required")
	}
	input := tasks.TaskWrite{
		Title:           request.Title.Value,
		StatusID:        "status-todo",
		DurationMinutes: 60,
		Description:     request.Description.Value,
		PriorityID:      request.PriorityID.Value,
		GroupID:         request.GroupID.Value,
		RepeatRule:      request.RepeatRule.Value,
	}
	if request.Note.Set {
		if request.Note.Null {
			return tasks.TaskWrite{}, fmt.Errorf("note cannot be null")
		}
		input.Note = request.Note.Value
	}
	if request.StatusID.Set {
		if request.StatusID.Null {
			return tasks.TaskWrite{}, fmt.Errorf("status_id cannot be null")
		}
		input.StatusID = request.StatusID.Value
	}
	if request.DurationMinutes.Set {
		if request.DurationMinutes.Null || request.DurationMinutes.Value < 1 || request.DurationMinutes.Value > math.MaxInt16 {
			return tasks.TaskWrite{}, fmt.Errorf("duration_minutes must be between 1 and 32767")
		}
		input.DurationMinutes = int16(request.DurationMinutes.Value)
	}
	var err error
	input.StartDate, err = parseDate(request.StartDate.Value)
	if err != nil {
		return tasks.TaskWrite{}, fmt.Errorf("start_date: %w", err)
	}
	input.EndDate, err = parseDate(request.EndDate.Value)
	if err != nil {
		return tasks.TaskWrite{}, fmt.Errorf("end_date: %w", err)
	}
	input.StartMinute, err = parseStartMinute(request.StartMinute.Value)
	if err != nil {
		return tasks.TaskWrite{}, err
	}
	return input, nil
}

func (request patchTaskRequest) toPatch() (tasks.TaskPatch, error) {
	patch := tasks.TaskPatch{
		Title:       request.Title,
		Note:        request.Note,
		Description: request.Description,
		StatusID:    request.StatusID,
		PriorityID:  request.PriorityID,
		GroupID:     request.GroupID,
		RepeatRule:  request.RepeatRule,
	}
	patch.StartDate.Set, patch.StartDate.Null = request.StartDate.Set, request.StartDate.Null
	patch.EndDate.Set, patch.EndDate.Null = request.EndDate.Set, request.EndDate.Null
	patch.StartMinute.Set, patch.StartMinute.Null = request.StartMinute.Set, request.StartMinute.Null
	patch.DurationMinutes.Set, patch.DurationMinutes.Null = request.DurationMinutes.Set, request.DurationMinutes.Null
	var err error
	patch.StartDate.Value, err = parseDate(request.StartDate.Value)
	if err != nil {
		return tasks.TaskPatch{}, fmt.Errorf("start_date: %w", err)
	}
	patch.EndDate.Value, err = parseDate(request.EndDate.Value)
	if err != nil {
		return tasks.TaskPatch{}, fmt.Errorf("end_date: %w", err)
	}
	patch.StartMinute.Value, err = parseStartMinute(request.StartMinute.Value)
	if err != nil {
		return tasks.TaskPatch{}, err
	}
	if request.DurationMinutes.Set && !request.DurationMinutes.Null {
		if request.DurationMinutes.Value < 1 || request.DurationMinutes.Value > math.MaxInt16 {
			return tasks.TaskPatch{}, fmt.Errorf("duration_minutes must be between 1 and 32767")
		}
		patch.DurationMinutes.Value = int16(request.DurationMinutes.Value)
	}
	return patch, nil
}

func parseDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	date, err := time.Parse(time.DateOnly, *value)
	if err != nil {
		return nil, fmt.Errorf("must use YYYY-MM-DD format")
	}
	return &date, nil
}

func parseStartMinute(value *int) (*int16, error) {
	if value == nil {
		return nil, nil
	}
	if *value < 0 || *value > 1439 {
		return nil, fmt.Errorf("start_minute must be between 0 and 1439")
	}
	minute := int16(*value)
	return &minute, nil
}
