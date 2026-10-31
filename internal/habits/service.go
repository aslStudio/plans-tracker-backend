package habits

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"
)

var ErrNotFound = errors.New("habit not found")
var ErrInvalidInput = errors.New("invalid habit input")
var ErrInvalidReference = errors.New("category does not belong to user")

type Field[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (f *Field[T]) UnmarshalJSON(data []byte) error {
	f.Set = true
	f.Null = bytes.Equal(bytes.TrimSpace(data), []byte("null"))
	if f.Null {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&f.Value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return ErrInvalidInput
	}
	return nil
}

type HabitInput struct {
	Title           string
	Note            string
	Description     string
	CategoryID      *string
	RepeatRule      RepeatRule
	StartMinute     *int16
	DurationMinutes int16
}

type HabitPatch struct {
	Title           Field[string]
	Note            Field[string]
	Description     Field[string]
	CategoryID      Field[*string]
	RepeatRule      Field[RepeatRule]
	StartMinute     Field[*int16]
	DurationMinutes Field[int16]
}

func (p HabitPatch) Empty() bool {
	return !p.Title.Set && !p.Note.Set && !p.Description.Set && !p.CategoryID.Set &&
		!p.RepeatRule.Set && !p.StartMinute.Set && !p.DurationMinutes.Set
}

type CategoryInput struct {
	Label string
	Color *string
}

type CategoryPatch struct {
	Label     Field[string]
	Color     Field[*string]
	SortOrder Field[int]
}

func (p CategoryPatch) Empty() bool {
	return !p.Label.Set && !p.Color.Set && !p.SortOrder.Set
}

type CompletionFilter struct {
	HabitID   string
	StartDate *time.Time
	EndDate   *time.Time
}

type Repository interface {
	List(ctx context.Context, userID string) ([]HabitDetails, error)
	ByID(ctx context.Context, userID, id string) (HabitDetails, error)
	Create(ctx context.Context, userID string, input HabitInput) (HabitDetails, error)
	Patch(ctx context.Context, userID, id string, patch HabitPatch) (HabitDetails, error)
	Delete(ctx context.Context, userID, id string) error
	CategoryList(ctx context.Context, userID string) ([]HabitCategory, error)
	CategoryByID(ctx context.Context, userID, id string) (HabitCategory, error)
	CreateCategory(ctx context.Context, userID string, input CategoryInput) (HabitCategory, error)
	PatchCategory(ctx context.Context, userID, id string, patch CategoryPatch) (HabitCategory, error)
	DeleteCategory(ctx context.Context, userID, id string) error
	CompletionList(ctx context.Context, userID string, filter CompletionFilter) ([]HabitCompletion, error)
	SetCompletion(ctx context.Context, userID, habitID string, date time.Time, completed bool) (*HabitCompletion, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context, userID string) ([]HabitDetails, error) {
	return s.repository.List(ctx, userID)
}

func (s *Service) ByID(ctx context.Context, userID, id string) (HabitDetails, error) {
	return s.repository.ByID(ctx, userID, id)
}

func (s *Service) Create(ctx context.Context, userID string, input HabitInput) (HabitDetails, error) {
	if err := input.Validate(); err != nil {
		return HabitDetails{}, err
	}
	return s.repository.Create(ctx, userID, input)
}

func (s *Service) Patch(ctx context.Context, userID, id string, patch HabitPatch) (HabitDetails, error) {
	if err := patch.Validate(); err != nil {
		return HabitDetails{}, err
	}
	return s.repository.Patch(ctx, userID, id, patch)
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repository.Delete(ctx, userID, id)
}

func (s *Service) CategoryList(ctx context.Context, userID string) ([]HabitCategory, error) {
	return s.repository.CategoryList(ctx, userID)
}

func (s *Service) CategoryByID(ctx context.Context, userID, id string) (HabitCategory, error) {
	return s.repository.CategoryByID(ctx, userID, id)
}

func (s *Service) CreateCategory(ctx context.Context, userID string, input CategoryInput) (HabitCategory, error) {
	if strings.TrimSpace(input.Label) == "" {
		return HabitCategory{}, ErrInvalidInput
	}
	return s.repository.CreateCategory(ctx, userID, input)
}

func (s *Service) PatchCategory(ctx context.Context, userID, id string, patch CategoryPatch) (HabitCategory, error) {
	if patch.Empty() || patch.Label.Null || (patch.Label.Set && strings.TrimSpace(patch.Label.Value) == "") ||
		patch.SortOrder.Null || (patch.SortOrder.Set && (patch.SortOrder.Value < 0 || patch.SortOrder.Value > math.MaxInt32)) {
		return HabitCategory{}, ErrInvalidInput
	}
	return s.repository.PatchCategory(ctx, userID, id, patch)
}

func (s *Service) DeleteCategory(ctx context.Context, userID, id string) error {
	return s.repository.DeleteCategory(ctx, userID, id)
}

func (s *Service) CompletionList(ctx context.Context, userID string, filter CompletionFilter) ([]HabitCompletion, error) {
	if (filter.StartDate == nil) != (filter.EndDate == nil) {
		return nil, ErrInvalidInput
	}
	if filter.StartDate != nil && (filter.EndDate.Before(*filter.StartDate) || filter.EndDate.Sub(*filter.StartDate) > 365*24*time.Hour) {
		return nil, ErrInvalidInput
	}
	return s.repository.CompletionList(ctx, userID, filter)
}

func (s *Service) SetCompletion(ctx context.Context, userID, habitID string, date time.Time, completed bool) (*HabitCompletion, error) {
	return s.repository.SetCompletion(ctx, userID, habitID, date, completed)
}

func (r RepeatRule) Validate() error {
	switch r.Type {
	case RepeatDaily:
		if len(r.DaysOfWeek) != 0 || r.DayOfMonth != nil || r.EveryNDays != nil {
			return ErrInvalidInput
		}
	case RepeatWeekly:
		if len(r.DaysOfWeek) == 0 || r.DayOfMonth != nil || r.EveryNDays != nil {
			return ErrInvalidInput
		}
		seen := map[int]bool{}
		for _, day := range r.DaysOfWeek {
			if day < 0 || day > 6 || seen[day] {
				return ErrInvalidInput
			}
			seen[day] = true
		}
	case RepeatMonthly:
		if len(r.DaysOfWeek) != 0 || r.DayOfMonth == nil || *r.DayOfMonth < 1 || *r.DayOfMonth > 31 || r.EveryNDays != nil {
			return ErrInvalidInput
		}
	case RepeatCustom:
		if len(r.DaysOfWeek) != 0 || r.DayOfMonth != nil || r.EveryNDays == nil || *r.EveryNDays < 1 || int64(*r.EveryNDays) > 9007199254740991 {
			return ErrInvalidInput
		}
	default:
		return ErrInvalidInput
	}
	return nil
}

func (in HabitInput) Validate() error {
	if strings.TrimSpace(in.Title) == "" || in.DurationMinutes < 1 ||
		(in.StartMinute != nil && (*in.StartMinute < 0 || *in.StartMinute > 1439)) ||
		(in.CategoryID != nil && *in.CategoryID == "") {
		return ErrInvalidInput
	}
	return in.RepeatRule.Validate()
}

func (p HabitPatch) Validate() error {
	if p.Empty() || p.Title.Null || (p.Title.Set && strings.TrimSpace(p.Title.Value) == "") ||
		p.Note.Null || p.Description.Null || p.RepeatRule.Null || p.DurationMinutes.Null ||
		(p.DurationMinutes.Set && p.DurationMinutes.Value < 1) ||
		(p.StartMinute.Set && p.StartMinute.Value != nil && (*p.StartMinute.Value < 0 || *p.StartMinute.Value > 1439)) ||
		(p.CategoryID.Set && p.CategoryID.Value != nil && *p.CategoryID.Value == "") {
		return ErrInvalidInput
	}
	if p.RepeatRule.Set {
		return p.RepeatRule.Value.Validate()
	}
	return nil
}
