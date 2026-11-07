package notification

import (
	"errors"
	"fmt"
)

// ErrNotFound is a sentinel error for not found resources
var ErrNotFound = errors.New("resource not found")

// NotFoundError represents a resource not found error
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID %s not found", e.Resource, e.ID)
}

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}
