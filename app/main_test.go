package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mi-24v/miwkey-extension/model"
	"github.com/mi-24v/miwkey-extension/notification"
	"github.com/stretchr/testify/assert"
)

type stubNotificationService struct{}

func (stubNotificationService) CreateNotification(context.Context, model.BaseNotification) (model.BaseNotification, error) {
	return model.BaseNotification{}, errors.New("not implemented")
}

func (stubNotificationService) GetNotifications(context.Context, string, notification.ListOptions) ([]model.BaseNotification, error) {
	return nil, errors.New("not implemented")
}

func (stubNotificationService) GetNotification(context.Context, string) (model.BaseNotification, error) {
	return model.BaseNotification{}, errors.New("not implemented")
}

func (stubNotificationService) UpdateNotification(context.Context, string, bool) (model.BaseNotification, error) {
	return model.BaseNotification{}, errors.New("not implemented")
}

func (stubNotificationService) DeleteNotification(context.Context, string) error {
	return errors.New("not implemented")
}

func (stubNotificationService) DeleteNotificationsByUser(context.Context, string) error {
	return errors.New("not implemented")
}

func TestHealthzDoesNotRequireAuthorization(t *testing.T) {
	e := newHTTPServer("shared-secret", stubNotificationService{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestNotificationRoutesStillRequireAuthorization(t *testing.T) {
	e := newHTTPServer("shared-secret", stubNotificationService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?userId=user-id", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRunHealthCheckSucceedsWhenHealthzIsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := runHealthCheck(context.Background(), server.URL)

	assert.NoError(t, err)
}

func TestRunHealthCheckFailsWhenHealthzIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := runHealthCheck(context.Background(), server.URL)

	assert.Error(t, err)
}
