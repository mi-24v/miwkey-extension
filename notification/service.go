package notification

import (
	"context"
	"errors"

	"github.com/mi-24v/miwkey-extension/model"
)

type Service[T model.Notification] interface {
	CreateNotification(ctx context.Context, notification T) (T, error)
	GetNotifications(ctx context.Context, userId string, opts ListOptions) ([]T, error)
	GetNotification(ctx context.Context, id string) (T, error)
	UpdateNotification(ctx context.Context, id string, isRead bool) (T, error)
	DeleteNotification(ctx context.Context, id string) error
	DeleteNotificationsByUser(ctx context.Context, userId string) error
}

// ServiceImpl handles notification business logic
type ServiceImpl[T model.Notification] struct {
	notificationStore Store[T]
}

// NewService creates a new notification service
func NewService[T model.Notification]() *ServiceImpl[T] {
	// For now, return a service with nil store
	// In a real implementation, you would inject a proper store
	return &ServiceImpl[T]{
		notificationStore: nil,
	}
}

// SetStore sets the notification store for the service
func (s *ServiceImpl[T]) SetStore(store Store[T]) {
	s.notificationStore = store
}

// CreateNotification creates a new notification
func (s *ServiceImpl[T]) CreateNotification(ctx context.Context, notification T) (T, error) {
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
func (s *ServiceImpl[T]) GetNotifications(ctx context.Context, userId string, opts ListOptions) ([]T, error) {
	if s.notificationStore == nil {
		return nil, errors.New("notification store not initialized")
	}

	return s.notificationStore.List(ctx, userId, opts)
}

// GetNotification retrieves a specific notification by ID
func (s *ServiceImpl[T]) GetNotification(ctx context.Context, id string) (T, error) {
	var empty T
	if s.notificationStore == nil {
		return empty, errors.New("notification store not initialized")
	}

	return s.notificationStore.Get(ctx, id)
}

// UpdateNotification updates a notification (currently only supports marking as read)
func (s *ServiceImpl[T]) UpdateNotification(ctx context.Context, id string, isRead bool) (T, error) {
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
func (s *ServiceImpl[T]) DeleteNotification(ctx context.Context, id string) error {
	if s.notificationStore == nil {
		return errors.New("notification store not initialized")
	}

	return s.notificationStore.Delete(ctx, id)
}

// DeleteNotificationsByUser deletes all notifications for a user
func (s *ServiceImpl[T]) DeleteNotificationsByUser(ctx context.Context, userId string) error {
	if s.notificationStore == nil {
		return errors.New("notification store not initialized")
	}

	return s.notificationStore.DeleteByUser(ctx, userId)
}
