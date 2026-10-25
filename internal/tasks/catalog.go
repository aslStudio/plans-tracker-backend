package tasks

import (
	"fmt"
	"strings"
)

type StatusInput struct {
	Label       string
	Color       *string
	IsCompleted bool
}

type StatusPatch struct {
	Label       Change[string]
	Color       Change[*string]
	IsCompleted Change[bool]
}

type PriorityInput struct {
	Label string
	Color *string
}

type PriorityPatch struct {
	Label Change[string]
	Color Change[*string]
}

type GroupInput struct {
	Label string
	Color string
}

type GroupPatch struct {
	Label Change[string]
	Color Change[string]
}

func validateCatalogLabel(label string) error {
	if strings.TrimSpace(label) == "" {
		return fmt.Errorf("%w: label must not be empty", ErrInvalidTask)
	}
	return nil
}

func validateCatalogColor(color *string) error {
	if color != nil && strings.TrimSpace(*color) == "" {
		return fmt.Errorf("%w: color must not be empty", ErrInvalidTask)
	}
	return nil
}

func (input StatusInput) Validate() error {
	if err := validateCatalogLabel(input.Label); err != nil {
		return err
	}
	return validateCatalogColor(input.Color)
}

func (patch StatusPatch) Validate() error {
	if !patch.Label.Set && !patch.Color.Set && !patch.IsCompleted.Set {
		return fmt.Errorf("%w: at least one field is required", ErrInvalidTask)
	}
	if patch.Label.Set {
		if patch.Label.Null {
			return fmt.Errorf("%w: label cannot be null", ErrInvalidTask)
		}
		if err := validateCatalogLabel(patch.Label.Value); err != nil {
			return err
		}
	}
	if patch.IsCompleted.Set && patch.IsCompleted.Null {
		return fmt.Errorf("%w: is_completed cannot be null", ErrInvalidTask)
	}
	if patch.Color.Set {
		return validateCatalogColor(patch.Color.Value)
	}
	return nil
}

func (input PriorityInput) Validate() error {
	if err := validateCatalogLabel(input.Label); err != nil {
		return err
	}
	return validateCatalogColor(input.Color)
}

func (patch PriorityPatch) Validate() error {
	if !patch.Label.Set && !patch.Color.Set {
		return fmt.Errorf("%w: at least one field is required", ErrInvalidTask)
	}
	if patch.Label.Set {
		if patch.Label.Null {
			return fmt.Errorf("%w: label cannot be null", ErrInvalidTask)
		}
		if err := validateCatalogLabel(patch.Label.Value); err != nil {
			return err
		}
	}
	if patch.Color.Set {
		return validateCatalogColor(patch.Color.Value)
	}
	return nil
}

func (input GroupInput) Validate() error {
	if err := validateCatalogLabel(input.Label); err != nil {
		return err
	}
	if strings.TrimSpace(input.Color) == "" {
		return fmt.Errorf("%w: color must not be empty", ErrInvalidTask)
	}
	return nil
}

func (patch GroupPatch) Validate() error {
	if !patch.Label.Set && !patch.Color.Set {
		return fmt.Errorf("%w: at least one field is required", ErrInvalidTask)
	}
	if patch.Label.Set {
		if patch.Label.Null {
			return fmt.Errorf("%w: label cannot be null", ErrInvalidTask)
		}
		if err := validateCatalogLabel(patch.Label.Value); err != nil {
			return err
		}
	}
	if patch.Color.Set && (patch.Color.Null || strings.TrimSpace(patch.Color.Value) == "") {
		return fmt.Errorf("%w: color must not be empty", ErrInvalidTask)
	}
	return nil
}
