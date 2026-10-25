package tasks

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("task not found")
var ErrInvalidTask = errors.New("invalid task")
var ErrInvalidReference = errors.New("invalid status_id, priority_id or group_id")
var ErrProtected = errors.New("built-in value cannot be deleted")
var ErrImmutable = errors.New("built-in completion state cannot be changed")
var ErrNotRecurring = errors.New("task does not repeat")

type Repository interface {
	List(ctx context.Context, userID string, startDate, endDate time.Time) ([]TaskDetails, error)
	Backlog(ctx context.Context, userID string) ([]TaskDetails, error)
	OccurrenceStates(ctx context.Context, userID string, startDate, endDate time.Time) (map[OccurrenceKey]OccurrenceState, error)
	SetOccurrenceStatus(ctx context.Context, userID, taskID string, date time.Time, statusID string) error
	SetOccurrenceCompletion(ctx context.Context, userID, taskID string, date time.Time, completed bool) error
	ByID(ctx context.Context, userID, id string) (TaskDetails, error)
	Create(ctx context.Context, userID string, input TaskWrite) (TaskDetails, error)
	Patch(ctx context.Context, userID, id string, patch TaskPatch) (TaskDetails, error)
	Delete(ctx context.Context, userID, id string) error
	StatusList(ctx context.Context, userID string) ([]TaskStatus, error)
	StatusById(ctx context.Context, userID, id string) (TaskStatus, error)
	CreateStatus(ctx context.Context, userID string, input StatusInput) (TaskStatus, error)
	PatchStatus(ctx context.Context, userID, id string, patch StatusPatch) (TaskStatus, error)
	DeleteStatus(ctx context.Context, userID, id string) error
	GroupList(ctx context.Context, userID string) ([]TaskGroup, error)
	GroupById(ctx context.Context, userID, id string) (TaskGroup, error)
	CreateGroup(ctx context.Context, userID string, input GroupInput) (TaskGroup, error)
	PatchGroup(ctx context.Context, userID, id string, patch GroupPatch) (TaskGroup, error)
	DeleteGroup(ctx context.Context, userID, id string) error
	PriorityList(ctx context.Context, userID string) ([]TaskPriority, error)
	PriorityById(ctx context.Context, userID, id string) (TaskPriority, error)
	CreatePriority(ctx context.Context, userID string, input PriorityInput) (TaskPriority, error)
	PatchPriority(ctx context.Context, userID, id string, patch PriorityPatch) (TaskPriority, error)
	DeletePriority(ctx context.Context, userID, id string) error
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context, userID string, startDate, endDate time.Time) ([]TaskDetails, error) {
	if err := validateRange(startDate, endDate); err != nil {
		return nil, err
	}
	items, err := s.repository.List(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	result := make([]TaskDetails, 0, len(items))
	for _, item := range items {
		for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
			if item.Task.OccursOn(date) {
				result = append(result, item)
				break
			}
		}
	}
	return result, nil
}

func (s *Service) ByID(ctx context.Context, userID, id string) (TaskDetails, error) {
	return s.repository.ByID(ctx, userID, id)
}

func (s *Service) Create(ctx context.Context, userID string, input TaskWrite) (TaskDetails, error) {
	if err := input.Validate(); err != nil {
		return TaskDetails{}, err
	}
	return s.repository.Create(ctx, userID, input)
}

func (s *Service) Patch(ctx context.Context, userID, id string, patch TaskPatch) (TaskDetails, error) {
	if err := patch.Validate(); err != nil {
		return TaskDetails{}, err
	}
	return s.repository.Patch(ctx, userID, id, patch)
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repository.Delete(ctx, userID, id)
}

func (s *Service) StatusList(ctx context.Context, userID string) ([]TaskStatus, error) {
	return s.repository.StatusList(ctx, userID)
}

func (s *Service) StatusByID(ctx context.Context, userID, id string) (TaskStatus, error) {
	return s.repository.StatusById(ctx, userID, id)
}

func (s *Service) CreateStatus(ctx context.Context, userID string, input StatusInput) (TaskStatus, error) {
	if err := input.Validate(); err != nil {
		return TaskStatus{}, err
	}
	return s.repository.CreateStatus(ctx, userID, input)
}

func (s *Service) PatchStatus(ctx context.Context, userID, id string, patch StatusPatch) (TaskStatus, error) {
	if err := patch.Validate(); err != nil {
		return TaskStatus{}, err
	}
	return s.repository.PatchStatus(ctx, userID, id, patch)
}

func (s *Service) DeleteStatus(ctx context.Context, userID, id string) error {
	return s.repository.DeleteStatus(ctx, userID, id)
}

func (s *Service) GroupList(ctx context.Context, userID string) ([]TaskGroup, error) {
	return s.repository.GroupList(ctx, userID)
}

func (s *Service) GroupByID(ctx context.Context, userID, id string) (TaskGroup, error) {
	return s.repository.GroupById(ctx, userID, id)
}

func (s *Service) CreateGroup(ctx context.Context, userID string, input GroupInput) (TaskGroup, error) {
	if err := input.Validate(); err != nil {
		return TaskGroup{}, err
	}
	return s.repository.CreateGroup(ctx, userID, input)
}

func (s *Service) PatchGroup(ctx context.Context, userID, id string, patch GroupPatch) (TaskGroup, error) {
	if err := patch.Validate(); err != nil {
		return TaskGroup{}, err
	}
	return s.repository.PatchGroup(ctx, userID, id, patch)
}

func (s *Service) DeleteGroup(ctx context.Context, userID, id string) error {
	return s.repository.DeleteGroup(ctx, userID, id)
}

func (s *Service) PriorityList(ctx context.Context, userID string) ([]TaskPriority, error) {
	return s.repository.PriorityList(ctx, userID)
}

func (s *Service) PriorityByID(ctx context.Context, userID, id string) (TaskPriority, error) {
	return s.repository.PriorityById(ctx, userID, id)
}

func (s *Service) CreatePriority(ctx context.Context, userID string, input PriorityInput) (TaskPriority, error) {
	if err := input.Validate(); err != nil {
		return TaskPriority{}, err
	}
	return s.repository.CreatePriority(ctx, userID, input)
}

func (s *Service) PatchPriority(ctx context.Context, userID, id string, patch PriorityPatch) (TaskPriority, error) {
	if err := patch.Validate(); err != nil {
		return TaskPriority{}, err
	}
	return s.repository.PatchPriority(ctx, userID, id, patch)
}

func (s *Service) DeletePriority(ctx context.Context, userID, id string) error {
	return s.repository.DeletePriority(ctx, userID, id)
}
