package tasks

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type OccurrenceKey struct {
	TaskID string
	Date   string
}

type OccurrenceState struct {
	StatusID    string
	CompletedAt *time.Time
}

type TaskOccurrence struct {
	Task        TaskDetails
	Date        time.Time
	Status      TaskStatus
	Completed   bool
	CompletedAt *time.Time
}

func dateOnly(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (task Task) OccursOn(date time.Time) bool {
	date = dateOnly(date)
	if task.RepeatRule == nil {
		return task.StartDate != nil && task.EndDate != nil && !date.Before(dateOnly(*task.StartDate)) && !date.After(dateOnly(*task.EndDate))
	}
	anchor := dateOnly(task.CreatedAt.UTC())
	if task.StartDate != nil {
		anchor = dateOnly(*task.StartDate)
	} else if task.EndDate != nil {
		anchor = dateOnly(*task.EndDate)
	}
	if date.Before(anchor) {
		return false
	}
	switch task.RepeatRule.Type {
	case RepeatDaily:
		return true
	case RepeatWeekly:
		for _, day := range task.RepeatRule.DaysOfWeek {
			if int(date.Weekday()) == day {
				return true
			}
		}
	case RepeatMonthly:
		return task.RepeatRule.DayOfMonth != nil && date.Day() == *task.RepeatRule.DayOfMonth
	case RepeatCustom:
		return task.RepeatRule.EveryNDays != nil && *task.RepeatRule.EveryNDays > 0 && int(date.Sub(anchor).Hours()/24)%*task.RepeatRule.EveryNDays == 0
	}
	return false
}

func validateRange(start, end time.Time) error {
	if end.Before(start) || end.Sub(start) > 365*24*time.Hour {
		return fmt.Errorf("%w: date range must contain at most 366 days", ErrInvalidTask)
	}
	return nil
}

func (s *Service) Backlog(ctx context.Context, userID string) ([]TaskDetails, error) {
	return s.repository.Backlog(ctx, userID)
}

func (s *Service) ListOccurrences(ctx context.Context, userID string, start, end time.Time) ([]TaskOccurrence, error) {
	if err := validateRange(start, end); err != nil {
		return nil, err
	}
	templates, err := s.List(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}
	states, err := s.repository.OccurrenceStates(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}
	statuses, err := s.repository.StatusList(ctx, userID)
	if err != nil {
		return nil, err
	}
	statusByID := make(map[string]TaskStatus, len(statuses))
	for _, status := range statuses {
		statusByID[status.ID] = status
	}
	result := make([]TaskOccurrence, 0)
	for _, template := range templates {
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			if !template.Task.OccursOn(date) {
				continue
			}
			state := states[OccurrenceKey{template.Task.ID, date.Format(time.DateOnly)}]
			occurrence := TaskOccurrence{Task: template, Date: date, Status: template.Status, Completed: template.Status.IsCompleted}
			if template.Task.RepeatRule != nil {
				occurrence.Status = statusByID["status-todo"]
				if status, ok := statusByID[state.StatusID]; ok {
					occurrence.Status = status
				}
				if state.CompletedAt != nil && !occurrence.Status.IsCompleted {
					occurrence.Status = statusByID["status-done"]
				}
				occurrence.Completed = state.CompletedAt != nil || occurrence.Status.IsCompleted
				occurrence.CompletedAt = state.CompletedAt
			}
			result = append(result, occurrence)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Date.Before(result[j].Date) })
	return result, nil
}

func (s *Service) Occurrence(ctx context.Context, userID, taskID string, date time.Time) (TaskOccurrence, error) {
	task, err := s.repository.ByID(ctx, userID, taskID)
	if err != nil {
		return TaskOccurrence{}, err
	}
	if !task.Task.OccursOn(date) {
		return TaskOccurrence{}, ErrNotFound
	}
	items, err := s.ListOccurrences(ctx, userID, date, date)
	if err != nil {
		return TaskOccurrence{}, err
	}
	for _, item := range items {
		if item.Task.Task.ID == taskID {
			return item, nil
		}
	}
	return TaskOccurrence{}, ErrNotFound
}

func (s *Service) SetOccurrenceStatus(ctx context.Context, userID, taskID string, date time.Time, statusID string) (TaskOccurrence, error) {
	if statusID == "" {
		return TaskOccurrence{}, ErrInvalidReference
	}
	if err := s.checkRecurringOccurrence(ctx, userID, taskID, date); err != nil {
		return TaskOccurrence{}, err
	}
	if err := s.repository.SetOccurrenceStatus(ctx, userID, taskID, date, statusID); err != nil {
		return TaskOccurrence{}, err
	}
	return s.Occurrence(ctx, userID, taskID, date)
}

func (s *Service) SetOccurrenceCompletion(ctx context.Context, userID, taskID string, date time.Time, completed bool) (TaskOccurrence, error) {
	if err := s.checkRecurringOccurrence(ctx, userID, taskID, date); err != nil {
		return TaskOccurrence{}, err
	}
	if err := s.repository.SetOccurrenceCompletion(ctx, userID, taskID, date, completed); err != nil {
		return TaskOccurrence{}, err
	}
	return s.Occurrence(ctx, userID, taskID, date)
}

func (s *Service) checkRecurringOccurrence(ctx context.Context, userID, taskID string, date time.Time) error {
	task, err := s.repository.ByID(ctx, userID, taskID)
	if err != nil {
		return err
	}
	if task.Task.RepeatRule == nil {
		return ErrNotRecurring
	}
	if !task.Task.OccursOn(date) {
		return ErrNotFound
	}
	return nil
}
