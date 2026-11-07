package notification

import (
	"context"

	"github.com/mi-24v/miwkey-extension/model"
)

// Store defines the interface for notification storage operations with generics
type Store[T model.Notification] interface {
	Create(ctx context.Context, notification T) error
	List(ctx context.Context, userId string, opts ListOptions) ([]T, error)
	Get(ctx context.Context, id string) (T, error)
	Update(ctx context.Context, id string, isRead bool) error
	Delete(ctx context.Context, id string) error
	DeleteByUser(ctx context.Context, userId string) error
}

// ListOptions controls pagination and filtering when listing notifications.
type ListOptions struct {
	SinceID      string
	UntilID      string
	Limit        int
	IncludeTypes []model.NotificationType
	ExcludeTypes []model.NotificationType
}

// RegisterStore registers a store with the notification service
func RegisterStore[T model.Notification](service *ServiceImpl[T], store Store[T]) {
	service.SetStore(store)
}
