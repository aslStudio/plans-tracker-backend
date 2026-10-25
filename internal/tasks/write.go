package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TaskWrite struct {
	Title           string
	Note            string
	Description     *string
	StatusID        string
	PriorityID      *string
	GroupID         *string
	StartDate       *time.Time
	EndDate         *time.Time
	StartMinute     *int16
	DurationMinutes int16
	RepeatRule      *RepeatRule
}

type Change[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (field *Change[T]) UnmarshalJSON(data []byte) error {
	field.Set = true
	field.Null = bytes.Equal(bytes.TrimSpace(data), []byte("null"))
	if field.Null {
		var zero T
		field.Value = zero
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(&field.Value)
}

type TaskPatch struct {
	Title           Change[string]
	Note            Change[string]
	Description     Change[*string]
	StatusID        Change[string]
	PriorityID      Change[*string]
	GroupID         Change[*string]
	StartDate       Change[*time.Time]
	EndDate         Change[*time.Time]
	StartMinute     Change[*int16]
	DurationMinutes Change[int16]
	RepeatRule      Change[*RepeatRule]
}

func (input *TaskWrite) Validate() error {
	if strings.TrimSpace(input.Title) == "" {
		return fmt.Errorf("%w: title must not be empty", ErrInvalidTask)
	}
	if strings.TrimSpace(input.StatusID) == "" {
		return fmt.Errorf("%w: status_id must not be empty", ErrInvalidTask)
	}
	if err := validateOptionalID("priority_id", input.PriorityID); err != nil {
		return err
	}
	if err := validateOptionalID("group_id", input.GroupID); err != nil {
		return err
	}
	if input.StartDate == nil && input.EndDate != nil {
		input.StartDate = input.EndDate
	}
	if input.EndDate == nil && input.StartDate != nil {
		input.EndDate = input.StartDate
	}
	if input.StartDate != nil && input.EndDate.Before(*input.StartDate) {
		return fmt.Errorf("%w: end_date must be on or after start_date", ErrInvalidTask)
	}
	if err := validateStartMinute(input.StartMinute); err != nil {
		return err
	}
	if input.DurationMinutes <= 0 {
		return fmt.Errorf("%w: duration_minutes must be positive", ErrInvalidTask)
	}
	return validateRepeatRule(input.RepeatRule)
}

func (patch TaskPatch) Validate() error {
	if !patch.Title.Set && !patch.Note.Set && !patch.Description.Set && !patch.StatusID.Set &&
		!patch.PriorityID.Set && !patch.GroupID.Set && !patch.StartDate.Set && !patch.EndDate.Set &&
		!patch.StartMinute.Set && !patch.DurationMinutes.Set && !patch.RepeatRule.Set {
		return fmt.Errorf("%w: at least one field is required", ErrInvalidTask)
	}
	if patch.Title.Set && (patch.Title.Null || strings.TrimSpace(patch.Title.Value) == "") {
		return fmt.Errorf("%w: title must not be empty", ErrInvalidTask)
	}
	if patch.Note.Set && patch.Note.Null {
		return fmt.Errorf("%w: note cannot be null", ErrInvalidTask)
	}
	if patch.StatusID.Set && (patch.StatusID.Null || strings.TrimSpace(patch.StatusID.Value) == "") {
		return fmt.Errorf("%w: status_id must not be empty", ErrInvalidTask)
	}
	if patch.PriorityID.Set {
		if err := validateOptionalID("priority_id", patch.PriorityID.Value); err != nil {
			return err
		}
	}
	if patch.GroupID.Set {
		if err := validateOptionalID("group_id", patch.GroupID.Value); err != nil {
			return err
		}
	}
	if patch.StartDate.Set != patch.EndDate.Set {
		return fmt.Errorf("%w: start_date and end_date must be changed together", ErrInvalidTask)
	}
	if patch.StartDate.Set {
		if (patch.StartDate.Value == nil) != (patch.EndDate.Value == nil) {
			return fmt.Errorf("%w: start_date and end_date must both be null or dates", ErrInvalidTask)
		}
		if patch.StartDate.Value != nil && patch.EndDate.Value.Before(*patch.StartDate.Value) {
			return fmt.Errorf("%w: end_date must be on or after start_date", ErrInvalidTask)
		}
	}
	if patch.StartMinute.Set {
		if err := validateStartMinute(patch.StartMinute.Value); err != nil {
			return err
		}
	}
	if patch.DurationMinutes.Set && (patch.DurationMinutes.Null || patch.DurationMinutes.Value <= 0) {
		return fmt.Errorf("%w: duration_minutes must be positive", ErrInvalidTask)
	}
	if patch.RepeatRule.Set {
		return validateRepeatRule(patch.RepeatRule.Value)
	}
	return nil
}

func validateOptionalID(name string, id *string) error {
	if id != nil && strings.TrimSpace(*id) == "" {
		return fmt.Errorf("%w: %s must not be empty", ErrInvalidTask, name)
	}
	return nil
}

func validateStartMinute(minute *int16) error {
	if minute != nil && (*minute < 0 || *minute > 1439) {
		return fmt.Errorf("%w: start_minute must be between 0 and 1439", ErrInvalidTask)
	}
	return nil
}

func validateRepeatRule(rule *RepeatRule) error {
	if rule == nil {
		return nil
	}
	switch rule.Type {
	case RepeatDaily:
		if len(rule.DaysOfWeek) != 0 || rule.DayOfMonth != nil || rule.EveryNDays != nil {
			return fmt.Errorf("%w: daily repeat_rule has unexpected fields", ErrInvalidTask)
		}
	case RepeatWeekly:
		if len(rule.DaysOfWeek) == 0 || rule.DayOfMonth != nil || rule.EveryNDays != nil {
			return fmt.Errorf("%w: weekly repeat_rule requires daysOfWeek", ErrInvalidTask)
		}
	case RepeatMonthly:
		if rule.DayOfMonth == nil || len(rule.DaysOfWeek) != 0 || rule.EveryNDays != nil {
			return fmt.Errorf("%w: monthly repeat_rule requires dayOfMonth", ErrInvalidTask)
		}
	case RepeatCustom:
		if rule.EveryNDays == nil || len(rule.DaysOfWeek) != 0 || rule.DayOfMonth != nil {
			return fmt.Errorf("%w: custom repeat_rule requires everyNDays", ErrInvalidTask)
		}
	default:
		return fmt.Errorf("%w: invalid repeat_rule type", ErrInvalidTask)
	}
	for _, day := range rule.DaysOfWeek {
		if day < 0 || day > 6 {
			return fmt.Errorf("%w: daysOfWeek must contain values from 0 to 6", ErrInvalidTask)
		}
	}
	for i, day := range rule.DaysOfWeek {
		for _, previous := range rule.DaysOfWeek[:i] {
			if day == previous {
				return fmt.Errorf("%w: daysOfWeek must not contain duplicates", ErrInvalidTask)
			}
		}
	}
	if rule.DayOfMonth != nil && (*rule.DayOfMonth < 1 || *rule.DayOfMonth > 31) {
		return fmt.Errorf("%w: dayOfMonth must be between 1 and 31", ErrInvalidTask)
	}
	if rule.EveryNDays != nil && *rule.EveryNDays < 1 {
		return fmt.Errorf("%w: everyNDays must be positive", ErrInvalidTask)
	}
	return nil
}
