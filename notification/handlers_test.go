package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/mi-24v/miwkey-extension/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockService is a mock implementation of the notification service for testing
type MockService struct {
	mock.Mock
}

func (m *MockService) CreateNotification(ctx context.Context, notification model.BaseNotification) (model.BaseNotification, error) {
	args := m.Called(ctx, notification)
	return args.Get(0).(model.BaseNotification), args.Error(1)
}

func (m *MockService) GetNotifications(ctx context.Context, userId string) ([]model.BaseNotification, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]model.BaseNotification), args.Error(1)
}

func (m *MockService) GetNotification(ctx context.Context, id string) (model.BaseNotification, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.BaseNotification), args.Error(1)
}

func (m *MockService) UpdateNotification(ctx context.Context, id string, isRead bool) (model.BaseNotification, error) {
	args := m.Called(ctx, id, isRead)
	return args.Get(0).(model.BaseNotification), args.Error(1)
}

func (m *MockService) DeleteNotification(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestGetNotificationsHandler(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		userId         string
		serviceSetup   func(*MockService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "Success",
			userId: "user-id",
			serviceSetup: func(m *MockService) {
				notifications := []model.BaseNotification{
					{
						ID:         "test-id-1",
						Type:       model.NotificationTypeTest,
						NotifierId: "user-id",
						IsRead:     false,
					},
				}
				m.On("GetNotifications", mock.Anything, "user-id").Return(notifications, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"id":"test-id-1","type":"test","createdAt":"0001-01-01T00:00:00Z","notifierId":"user-id","isRead":false}]`,
		},
		{
			name:   "Missing UserId",
			userId: "",
			serviceSetup: func(m *MockService) {
				// No setup needed as the handler should return an error before calling the service
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"userId is required"}`,
		},
		{
			name:   "Service Error",
			userId: "user-id",
			serviceSetup: func(m *MockService) {
				m.On("GetNotifications", mock.Anything, "user-id").Return([]model.BaseNotification{}, errors.New("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"service error"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Add query parameter if needed
			if tc.userId != "" {
				q := req.URL.Query()
				q.Add("userId", tc.userId)
				req.URL.RawQuery = q.Encode()
			}

			// Create mock service
			mockService := new(MockService)
			tc.serviceSetup(mockService)

			// Create handler with mock service
			handler := NewNotificationHandler[model.BaseNotification](mockService)

			// Execute
			err := handler.GetNotifications(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.JSONEq(t, tc.expectedBody, rec.Body.String())
		})
	}
}

func TestCreateNotificationHandler(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		requestBody    string
		serviceSetup   func(*MockService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Success",
			requestBody: `{
				"id": "test-id",
				"type": "test",
				"notifierId": "user-id",
				"isRead": false
			}`,
			serviceSetup: func(m *MockService) {
				notification := model.BaseNotification{
					ID:         "test-id",
					Type:       model.NotificationTypeTest,
					NotifierId: "user-id",
					IsRead:     false,
				}
				m.On("CreateNotification", mock.Anything, mock.AnythingOfType("model.BaseNotification")).Return(notification, nil)
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"id":"test-id","type":"test","createdAt":"0001-01-01T00:00:00Z","notifierId":"user-id","isRead":false}`,
		},
		{
			name:        "Invalid Request Body",
			requestBody: `invalid json`,
			serviceSetup: func(m *MockService) {
				// No setup needed as the handler should return an error before calling the service
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"Invalid request body: code=400, message=Syntax error: offset=1, error=invalid character 'i' looking for beginning of value"}`,
		},
		{
			name: "Service Error",
			requestBody: `{
				"id": "test-id",
				"type": "test",
				"notifierId": "user-id",
				"isRead": false
			}`,
			serviceSetup: func(m *MockService) {
				m.On("CreateNotification", mock.Anything, mock.AnythingOfType("model.BaseNotification")).Return(model.BaseNotification{}, errors.New("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"service error"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", strings.NewReader(tc.requestBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Create mock service
			mockService := new(MockService)
			tc.serviceSetup(mockService)

			// Create handler with mock service
			handler := NewNotificationHandler[model.BaseNotification](mockService)

			// Execute
			err := handler.CreateNotification(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, rec.Code)

			// For the invalid JSON case, we can't do an exact match because the error message might vary
			if tc.name == "Invalid Request Body" {
				assert.Contains(t, rec.Body.String(), "Invalid request body")
			} else {
				assert.JSONEq(t, tc.expectedBody, rec.Body.String())
			}
		})
	}
}

func TestGetNotificationHandler(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		notificationId string
		serviceSetup   func(*MockService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Success",
			notificationId: "test-id",
			serviceSetup: func(m *MockService) {
				notification := model.BaseNotification{
					ID:         "test-id",
					Type:       model.NotificationTypeTest,
					NotifierId: "user-id",
					IsRead:     false,
				}
				m.On("GetNotification", mock.Anything, "test-id").Return(notification, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"id":"test-id","type":"test","createdAt":"0001-01-01T00:00:00Z","notifierId":"user-id","isRead":false}`,
		},
		{
			name:           "Missing NotificationId",
			notificationId: "",
			serviceSetup: func(m *MockService) {
				// No setup needed as the handler should return an error before calling the service
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"notificationId is required"}`,
		},
		{
			name:           "Notification Not Found",
			notificationId: "test-id",
			serviceSetup: func(m *MockService) {
				m.On("GetNotification", mock.Anything, "test-id").Return(model.BaseNotification{}, &NotFoundError{
					Resource: "Notification",
					ID:       "test-id",
				})
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"Notification with ID test-id not found"}`,
		},
		{
			name:           "Service Error",
			notificationId: "test-id",
			serviceSetup: func(m *MockService) {
				m.On("GetNotification", mock.Anything, "test-id").Return(model.BaseNotification{}, errors.New("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"service error"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/"+tc.notificationId, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Set path parameter
			if tc.notificationId != "" {
				c.SetParamNames("notificationId")
				c.SetParamValues(tc.notificationId)
			}

			// Create mock service
			mockService := new(MockService)
			tc.serviceSetup(mockService)

			// Create handler with mock service
			handler := NewNotificationHandler[model.BaseNotification](mockService)

			// Execute
			err := handler.GetNotification(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, rec.Code)
			assert.JSONEq(t, tc.expectedBody, rec.Body.String())
		})
	}
}

func TestUpdateNotificationHandler(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		notificationId string
		requestBody    string
		serviceSetup   func(*MockService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Success",
			notificationId: "test-id",
			requestBody:    `{"isRead": true}`,
			serviceSetup: func(m *MockService) {
				notification := model.BaseNotification{
					ID:         "test-id",
					Type:       model.NotificationTypeTest,
					NotifierId: "user-id",
					IsRead:     true,
				}
				m.On("UpdateNotification", mock.Anything, "test-id", true).Return(notification, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"id":"test-id","type":"test","createdAt":"0001-01-01T00:00:00Z","notifierId":"user-id","isRead":true}`,
		},
		{
			name:           "Missing NotificationId",
			notificationId: "",
			requestBody:    `{"isRead": true}`,
			serviceSetup: func(m *MockService) {
				// No setup needed as the handler should return an error before calling the service
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"notificationId is required"}`,
		},
		{
			name:           "Invalid Request Body",
			notificationId: "test-id",
			requestBody:    `invalid json`,
			serviceSetup: func(m *MockService) {
				// No setup needed as the handler should return an error before calling the service
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"Invalid request body: code=400, message=Syntax error: offset=1, error=invalid character 'i' looking for beginning of value"}`,
		},
		{
			name:           "Notification Not Found",
			notificationId: "test-id",
			requestBody:    `{"isRead": true}`,
			serviceSetup: func(m *MockService) {
				m.On("UpdateNotification", mock.Anything, "test-id", true).Return(model.BaseNotification{}, &NotFoundError{
					Resource: "Notification",
					ID:       "test-id",
				})
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"Notification with ID test-id not found"}`,
		},
		{
			name:           "Service Error",
			notificationId: "test-id",
			requestBody:    `{"isRead": true}`,
			serviceSetup: func(m *MockService) {
				m.On("UpdateNotification", mock.Anything, "test-id", true).Return(model.BaseNotification{}, errors.New("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"service error"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/"+tc.notificationId, strings.NewReader(tc.requestBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Set path parameter
			if tc.notificationId != "" {
				c.SetParamNames("notificationId")
				c.SetParamValues(tc.notificationId)
			}

			// Create mock service
			mockService := new(MockService)
			tc.serviceSetup(mockService)

			// Create handler with mock service
			handler := NewNotificationHandler[model.BaseNotification](mockService)

			// Execute
			err := handler.UpdateNotification(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, rec.Code)

			// For the invalid JSON case, we can't do an exact match because the error message might vary
			if tc.name == "Invalid Request Body" {
				assert.Contains(t, rec.Body.String(), "Invalid request body")
			} else {
				assert.JSONEq(t, tc.expectedBody, rec.Body.String())
			}
		})
	}
}

func TestDeleteNotificationHandler(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		notificationId string
		serviceSetup   func(*MockService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Success",
			notificationId: "test-id",
			serviceSetup: func(m *MockService) {
				m.On("DeleteNotification", mock.Anything, "test-id").Return(nil)
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:           "Missing NotificationId",
			notificationId: "",
			serviceSetup: func(m *MockService) {
				// No setup needed as the handler should return an error before calling the service
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"notificationId is required"}`,
		},
		{
			name:           "Service Error",
			notificationId: "test-id",
			serviceSetup: func(m *MockService) {
				m.On("DeleteNotification", mock.Anything, "test-id").Return(errors.New("service error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"service error"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/notifications/"+tc.notificationId, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Set path parameter
			if tc.notificationId != "" {
				c.SetParamNames("notificationId")
				c.SetParamValues(tc.notificationId)
			}

			// Create mock service
			mockService := new(MockService)
			tc.serviceSetup(mockService)

			// Create handler with mock service
			handler := NewNotificationHandler[model.BaseNotification](mockService)

			// Execute
			err := handler.DeleteNotification(c)

			// Verify
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, rec.Code)
			if tc.expectedBody != "" {
				assert.JSONEq(t, tc.expectedBody, rec.Body.String())
			} else {
				assert.Empty(t, rec.Body.String())
			}
		})
	}
}

func TestRegisterHandlers(t *testing.T) {
	// Setup
	e := echo.New()
	mockService := new(MockService)

	// Execute
	RegisterHandlers[model.BaseNotification](e, mockService)

	// Verify
	// Check that routes are registered by making a request to each endpoint
	// This is a basic test to ensure the function doesn't panic
	assert.NotPanics(t, func() {
		RegisterHandlers[model.BaseNotification](e, mockService)
	})

	// Verify that the routes are registered correctly
	routes := e.Routes()

	// Expected routes
	expectedRoutes := map[string]string{
		"GET /api/v1/notifications":                    "GetNotifications",
		"POST /api/v1/notifications":                   "CreateNotification",
		"GET /api/v1/notifications/:notificationId":    "GetNotification",
		"PUT /api/v1/notifications/:notificationId":    "UpdateNotification",
		"DELETE /api/v1/notifications/:notificationId": "DeleteNotification",
	}

	// Check that all expected routes are registered
	foundRoutes := make(map[string]bool)
	for _, route := range routes {
		key := route.Method + " " + route.Path
		// It's hard to check the handler name, so we just check that it's not empty
		if _, ok := expectedRoutes[key]; ok {
			foundRoutes[key] = true
			assert.NotEmpty(t, route.Name)
		}
	}

	// Check that all expected routes were found
	for key := range expectedRoutes {
		assert.True(t, foundRoutes[key], "Route not found: "+key)
	}
}
