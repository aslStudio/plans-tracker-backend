package calendar

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid calendar input")
var ErrNotFound = errors.New("calendar event not found")
var ErrInvalidLink = errors.New("linked resource not found")
var ErrDependency = errors.New("linked service unavailable")

const DateLayout = "2006-01-02"

type RepeatRule struct {
	Type       string `json:"type"`
	DaysOfWeek []int  `json:"daysOfWeek,omitempty"`
	DayOfMonth *int   `json:"dayOfMonth,omitempty"`
	EveryNDays *int   `json:"everyNDays,omitempty"`
}

type Link struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type EventInput struct {
	Title           string      `json:"title"`
	Date            string      `json:"date"`
	StartMinute     int         `json:"start_minute"`
	DurationMinutes int         `json:"duration_minutes"`
	AllDay          bool        `json:"all_day"`
	Color           string      `json:"color"`
	Description     *string     `json:"description"`
	Notes           string      `json:"notes"`
	Location        string      `json:"location"`
	RepeatRule      *RepeatRule `json:"repeat_rule"`
	RepeatUntil     *string     `json:"repeat_until"`
	ExcludedDates   []string    `json:"excluded_dates"`
	Link            *Link       `json:"link"`
}

type Event struct {
	ID string `json:"id"`
	EventInput
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Settings struct {
	ShowTasks  bool   `json:"show_tasks"`
	ShowHabits bool   `json:"show_habits"`
	ViewMode   string `json:"view_mode"`
}

type SettingsPatch struct {
	ShowTasks  *bool   `json:"show_tasks"`
	ShowHabits *bool   `json:"show_habits"`
	ViewMode   *string `json:"view_mode"`
}

type Repository interface {
	List(context.Context, string, time.Time, time.Time) ([]Event, error)
	ByID(context.Context, string, string) (Event, error)
	Create(context.Context, string, EventInput) (Event, error)
	Replace(context.Context, string, string, EventInput) (Event, error)
	Delete(context.Context, string, string) error
	Exclude(context.Context, string, string, string, bool) (Event, error)
	Settings(context.Context, string) (Settings, error)
	PatchSettings(context.Context, string, SettingsPatch) (Settings, error)
}

type LinkValidator interface {
	Validate(context.Context, Link) error
}

type Service struct {
	Repository Repository
	Links      LinkValidator
}

func ParseDate(value string) (time.Time, error) {
	d, err := time.Parse(DateLayout, value)
	if err != nil || d.Format(DateLayout) != value {
		return time.Time{}, ErrInvalid
	}
	return d, nil
}

func (r *RepeatRule) Validate() error {
	if r == nil {
		return nil
	}
	switch r.Type {
	case "daily":
		if len(r.DaysOfWeek) != 0 || r.DayOfMonth != nil || r.EveryNDays != nil {
			return ErrInvalid
		}
	case "weekly":
		if len(r.DaysOfWeek) == 0 || r.DayOfMonth != nil || r.EveryNDays != nil {
			return ErrInvalid
		}
		seen := map[int]bool{}
		for _, d := range r.DaysOfWeek {
			if d < 0 || d > 6 || seen[d] {
				return ErrInvalid
			}
			seen[d] = true
		}
	case "monthly":
		if len(r.DaysOfWeek) != 0 || r.DayOfMonth == nil || *r.DayOfMonth < 1 || *r.DayOfMonth > 31 || r.EveryNDays != nil {
			return ErrInvalid
		}
	case "custom":
		if len(r.DaysOfWeek) != 0 || r.DayOfMonth != nil || r.EveryNDays == nil || *r.EveryNDays < 1 {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func (e EventInput) Validate() error {
	start, err := ParseDate(e.Date)
	if err != nil || strings.TrimSpace(e.Title) == "" || strings.TrimSpace(e.Color) == "" || e.StartMinute < 0 || e.StartMinute > 1439 || e.DurationMinutes < 1 || e.DurationMinutes > 1440-e.StartMinute {
		return ErrInvalid
	}
	if err := e.RepeatRule.Validate(); err != nil {
		return err
	}
	if e.RepeatUntil != nil {
		until, err := ParseDate(*e.RepeatUntil)
		if err != nil || until.Before(start) || e.RepeatRule == nil {
			return ErrInvalid
		}
	}
	seen := map[string]bool{}
	for _, date := range e.ExcludedDates {
		if _, err := ParseDate(date); err != nil || seen[date] || !Occurs(e, date) {
			return ErrInvalid
		}
		seen[date] = true
	}
	if e.Link != nil && ((e.Link.Type != "task" && e.Link.Type != "habit") || strings.TrimSpace(e.Link.ID) == "") {
		return ErrInvalid
	}
	return nil
}

func Occurs(e EventInput, date string) bool {
	start, err := ParseDate(e.Date)
	if err != nil {
		return false
	}
	day, err := ParseDate(date)
	if err != nil || day.Before(start) {
		return false
	}
	if e.RepeatUntil != nil && date > *e.RepeatUntil {
		return false
	}
	if e.RepeatRule == nil {
		return date == e.Date
	}
	switch e.RepeatRule.Type {
	case "daily":
		return true
	case "weekly":
		for _, d := range e.RepeatRule.DaysOfWeek {
			if d == int(day.Weekday()) {
				return true
			}
		}
		return false
	case "monthly":
		return e.RepeatRule.DayOfMonth != nil && day.Day() == *e.RepeatRule.DayOfMonth
	case "custom":
		return e.RepeatRule.EveryNDays != nil && *e.RepeatRule.EveryNDays > 0 && int(day.Sub(start).Hours()/24)%*e.RepeatRule.EveryNDays == 0
	}
	return false
}

func VisibleInRange(e EventInput, start, end time.Time) bool {
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format(DateLayout)
		if Occurs(e, date) {
			excluded := false
			for _, x := range e.ExcludedDates {
				if x == date {
					excluded = true
					break
				}
			}
			if !excluded {
				return true
			}
		}
	}
	return false
}

func (s Service) List(ctx context.Context, user string, start, end time.Time) ([]Event, error) {
	if end.Before(start) || end.Sub(start) > 365*24*time.Hour {
		return nil, ErrInvalid
	}
	return s.Repository.List(ctx, user, start, end)
}
func (s Service) Create(ctx context.Context, user string, input EventInput) (Event, error) {
	if err := input.Validate(); err != nil {
		return Event{}, err
	}
	if input.Link != nil && s.Links != nil {
		if err := s.Links.Validate(ctx, *input.Link); err != nil {
			return Event{}, err
		}
	}
	return s.Repository.Create(ctx, user, input)
}
func (s Service) Replace(ctx context.Context, user, id string, input EventInput) (Event, error) {
	if err := input.Validate(); err != nil {
		return Event{}, err
	}
	if input.Link != nil && s.Links != nil {
		if err := s.Links.Validate(ctx, *input.Link); err != nil {
			return Event{}, err
		}
	}
	return s.Repository.Replace(ctx, user, id, input)
}
func (s Service) Exclude(ctx context.Context, user, id, date string, exclude bool) (Event, error) {
	if _, err := ParseDate(date); err != nil {
		return Event{}, err
	}
	e, err := s.Repository.ByID(ctx, user, id)
	if err != nil {
		return Event{}, err
	}
	if exclude && !Occurs(e.EventInput, date) {
		return Event{}, ErrInvalid
	}
	return s.Repository.Exclude(ctx, user, id, date, exclude)
}
