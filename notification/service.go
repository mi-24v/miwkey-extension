package notification

import (
	"context"
	"errors"

	"github.com/mi-24v/miwkey-extension/model"
)

// Store defines the interface for notification storage operations with generics
type Store[T model.Notification] interface {
	Create(ctx context.Context, notification T) error
	List(ctx context.Context, userId string) ([]T, error)
	Get(ctx context.Context, id string) (T, error)
	Update(ctx context.Context, id string, isRead bool) error
	Delete(ctx context.Context, id string) error
}

// Service handles notification business logic
type Service[T model.Notification] struct {
	notificationStore Store[T]
}

// NewService creates a new notification service
func NewService[T model.Notification]() *Service[T] {
	// For now, return a service with nil store
	// In a real implementation, you would inject a proper store
	return &Service[T]{
		notificationStore: nil,
	}
}

// SetStore sets the notification store for the service
func (s *Service[T]) SetStore(store Store[T]) {
	s.notificationStore = store
}

// CreateNotification creates a new notification
func (s *Service[T]) CreateNotification(ctx context.Context, notification T) (T, error) {
	var empty T
	if s.notificationStore == nil {
		return empty, errors.New("notification store not initialized")
	}

	err := s.notificationStore.Create(ctx, notification)
	if err != nil {
		return empty, err
	}

	return notification, nil
}

// GetNotifications retrieves all notifications for a user
func (s *Service[T]) GetNotifications(ctx context.Context, userId string) ([]T, error) {
	if s.notificationStore == nil {
		return nil, errors.New("notification store not initialized")
	}

	return s.notificationStore.List(ctx, userId)
}

// GetNotification retrieves a specific notification by ID
func (s *Service[T]) GetNotification(ctx context.Context, id string) (T, error) {
	var empty T
	if s.notificationStore == nil {
		return empty, errors.New("notification store not initialized")
	}

	return s.notificationStore.Get(ctx, id)
}

// UpdateNotification updates a notification (currently only supports marking as read)
func (s *Service[T]) UpdateNotification(ctx context.Context, id string, isRead bool) (T, error) {
	var empty T
	if s.notificationStore == nil {
		return empty, errors.New("notification store not initialized")
	}

	err := s.notificationStore.Update(ctx, id, isRead)
	if err != nil {
		return empty, err
	}

	return s.notificationStore.Get(ctx, id)
}

// DeleteNotification deletes a notification by ID
func (s *Service[T]) DeleteNotification(ctx context.Context, id string) error {
	if s.notificationStore == nil {
		return errors.New("notification store not initialized")
	}

	return s.notificationStore.Delete(ctx, id)
}
