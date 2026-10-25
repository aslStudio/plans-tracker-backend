package httpapi

import (
	"fmt"

	"plans-tracker-backend/internal/tasks"
)

type statusRequest struct {
	Label       tasks.Change[string]  `json:"label"`
	Color       tasks.Change[*string] `json:"color"`
	IsCompleted tasks.Change[bool]    `json:"is_completed"`
}

func (request statusRequest) toCreate() (tasks.StatusInput, error) {
	if !request.Label.Set || request.Label.Null {
		return tasks.StatusInput{}, fmt.Errorf("label is required")
	}
	if request.IsCompleted.Null {
		return tasks.StatusInput{}, fmt.Errorf("is_completed cannot be null")
	}
	return tasks.StatusInput{
		Label: request.Label.Value, Color: request.Color.Value,
		IsCompleted: request.IsCompleted.Value,
	}, nil
}

func (request statusRequest) toPatch() tasks.StatusPatch {
	return tasks.StatusPatch{
		Label: request.Label, Color: request.Color, IsCompleted: request.IsCompleted,
	}
}

type priorityRequest struct {
	Label tasks.Change[string]  `json:"label"`
	Color tasks.Change[*string] `json:"color"`
}

func (request priorityRequest) toCreate() (tasks.PriorityInput, error) {
	if !request.Label.Set || request.Label.Null {
		return tasks.PriorityInput{}, fmt.Errorf("label is required")
	}
	return tasks.PriorityInput{Label: request.Label.Value, Color: request.Color.Value}, nil
}

func (request priorityRequest) toPatch() tasks.PriorityPatch {
	return tasks.PriorityPatch{Label: request.Label, Color: request.Color}
}

type groupRequest struct {
	Label tasks.Change[string] `json:"label"`
	Color tasks.Change[string] `json:"color"`
}

func (request groupRequest) toCreate() (tasks.GroupInput, error) {
	if !request.Label.Set || request.Label.Null {
		return tasks.GroupInput{}, fmt.Errorf("label is required")
	}
	if !request.Color.Set || request.Color.Null {
		return tasks.GroupInput{}, fmt.Errorf("color is required")
	}
	return tasks.GroupInput{Label: request.Label.Value, Color: request.Color.Value}, nil
}

func (request groupRequest) toPatch() tasks.GroupPatch {
	return tasks.GroupPatch{Label: request.Label, Color: request.Color}
}
