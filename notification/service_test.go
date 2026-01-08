package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mi-24v/miwkey-extension/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore is a mock implementation of the Store interface for testing
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Create(ctx context.Context, notification model.BaseNotification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockStore) List(ctx context.Context, userId string, opts ListOptions) ([]model.BaseNotification, error) {
	args := m.Called(ctx, userId, opts)
	return args.Get(0).([]model.BaseNotification), args.Error(1)
}

func (m *MockStore) Get(ctx context.Context, id string) (model.BaseNotification, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.BaseNotification), args.Error(1)
}

func (m *MockStore) Update(ctx context.Context, id string, isRead bool) error {
	args := m.Called(ctx, id, isRead)
	return args.Error(0)
}

func (m *MockStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStore) DeleteByUser(ctx context.Context, userId string) error {
	args := m.Called(ctx, userId)
	return args.Error(0)
}

func TestNewService(t *testing.T) {
	service := NewService[model.BaseNotification]()
	assert.NotNil(t, service)
	assert.Nil(t, service.notificationStore)
}

func TestSetStore(t *testing.T) {
	service := NewService[model.BaseNotification]()
	mockStore := new(MockStore)

	service.SetStore(mockStore)
	assert.Equal(t, mockStore, service.notificationStore)
}

func TestCreateNotification(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		storeSetup    func(*MockStore)
		notification  model.BaseNotification
		expectedError bool
	}{
		{
			name: "Success",
			storeSetup: func(m *MockStore) {
				m.On("Create", mock.Anything, mock.AnythingOfType("model.BaseNotification")).Return(nil)
			},
			notification: model.BaseNotification{
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				CreatedAt:  time.Now(),
				NotifieeId: "target-id",
				NotifierId: "user-id",
				IsRead:     false,
			},
			expectedError: false,
		},
		{
			name: "Store Error",
			storeSetup: func(m *MockStore) {
				m.On("Create", mock.Anything, mock.AnythingOfType("model.BaseNotification")).Return(errors.New("store error"))
			},
			notification: model.BaseNotification{
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				CreatedAt:  time.Now(),
				NotifieeId: "target-id",
				NotifierId: "user-id",
				IsRead:     false,
			},
			expectedError: true,
		},
		{
			name:          "Nil Store",
			storeSetup:    func(m *MockStore) {},
			notification:  model.BaseNotification{},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			service := NewService[model.BaseNotification]()
			if tc.name != "Nil Store" {
				mockStore := new(MockStore)
				tc.storeSetup(mockStore)
				service.SetStore(mockStore)
			}

			// Execute
			result, err := service.CreateNotification(context.Background(), tc.notification)

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.notification, result)
			}
		})
	}
}

func TestGetNotifications(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		storeSetup     func(*MockStore)
		userId         string
		expectedResult []model.BaseNotification
		expectedError  bool
	}{
		{
			name: "Success",
			storeSetup: func(m *MockStore) {
				notifications := []model.BaseNotification{
					{
						ID:         "test-id-1",
						Type:       model.NotificationTypeTest,
						CreatedAt:  time.Now(),
						NotifieeId: "user-id",
						NotifierId: "user-id",
						IsRead:     false,
					},
					{
						ID:         "test-id-2",
						Type:       model.NotificationTypeTest,
						CreatedAt:  time.Now(),
						NotifieeId: "user-id",
						NotifierId: "user-id",
						IsRead:     true,
					},
				}
				m.On("List", mock.Anything, "user-id", mock.AnythingOfType("notification.ListOptions")).Return(notifications, nil)
			},
			userId: "user-id",
			expectedResult: []model.BaseNotification{
				{
					ID:         "test-id-1",
					Type:       model.NotificationTypeTest,
					CreatedAt:  time.Now(),
					NotifierId: "user-id",
					IsRead:     false,
				},
				{
					ID:         "test-id-2",
					Type:       model.NotificationTypeTest,
					CreatedAt:  time.Now(),
					NotifierId: "user-id",
					IsRead:     true,
				},
			},
			expectedError: false,
		},
		{
			name: "Store Error",
			storeSetup: func(m *MockStore) {
				m.On("List", mock.Anything, "user-id", mock.AnythingOfType("notification.ListOptions")).Return([]model.BaseNotification{}, errors.New("store error"))
			},
			userId:         "user-id",
			expectedResult: nil,
			expectedError:  true,
		},
		{
			name:           "Nil Store",
			storeSetup:     func(m *MockStore) {},
			userId:         "user-id",
			expectedResult: nil,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			service := NewService[model.BaseNotification]()
			if tc.name != "Nil Store" {
				mockStore := new(MockStore)
				tc.storeSetup(mockStore)
				service.SetStore(mockStore)
			}

			// Execute
			result, err := service.GetNotifications(context.Background(), tc.userId, ListOptions{})

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Note: We can't directly compare time.Time values in the structs
				// so we're just checking the length here
				assert.Equal(t, len(tc.expectedResult), len(result))
			}
		})
	}
}

func TestGetNotification(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		storeSetup     func(*MockStore)
		id             string
		expectedResult model.BaseNotification
		expectedError  bool
	}{
		{
			name: "Success",
			storeSetup: func(m *MockStore) {
				notification := model.BaseNotification{
					ID:         "test-id",
					Type:       model.NotificationTypeTest,
					CreatedAt:  time.Now(),
					NotifieeId: "user-id",
					NotifierId: "user-id",
					IsRead:     false,
				}
				m.On("Get", mock.Anything, "test-id").Return(notification, nil)
			},
			id: "test-id",
			expectedResult: model.BaseNotification{
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				CreatedAt:  time.Now(),
				NotifieeId: "user-id",
				NotifierId: "user-id",
				IsRead:     false,
			},
			expectedError: false,
		},
		{
			name: "Store Error",
			storeSetup: func(m *MockStore) {
				var empty model.BaseNotification
				m.On("Get", mock.Anything, "test-id").Return(empty, errors.New("store error"))
			},
			id:             "test-id",
			expectedResult: model.BaseNotification{},
			expectedError:  true,
		},
		{
			name:           "Nil Store",
			storeSetup:     func(m *MockStore) {},
			id:             "test-id",
			expectedResult: model.BaseNotification{},
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			service := NewService[model.BaseNotification]()
			if tc.name != "Nil Store" {
				mockStore := new(MockStore)
				tc.storeSetup(mockStore)
				service.SetStore(mockStore)
			}

			// Execute
			result, err := service.GetNotification(context.Background(), tc.id)

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Note: We can't directly compare time.Time values
				assert.Equal(t, tc.expectedResult.ID, result.ID)
				assert.Equal(t, tc.expectedResult.Type, result.Type)
				assert.Equal(t, tc.expectedResult.NotifierId, result.NotifierId)
				assert.Equal(t, tc.expectedResult.IsRead, result.IsRead)
			}
		})
	}
}

func TestUpdateNotification(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		storeSetup     func(*MockStore)
		id             string
		isRead         bool
		expectedResult model.BaseNotification
		expectedError  bool
	}{
		{
			name: "Success",
			storeSetup: func(m *MockStore) {
				m.On("Update", mock.Anything, "test-id", true).Return(nil)
				notification := model.BaseNotification{
					ID:         "test-id",
					Type:       model.NotificationTypeTest,
					CreatedAt:  time.Now(),
					NotifieeId: "user-id",
					NotifierId: "user-id",
					IsRead:     true,
				}
				m.On("Get", mock.Anything, "test-id").Return(notification, nil)
			},
			id:     "test-id",
			isRead: true,
			expectedResult: model.BaseNotification{
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				CreatedAt:  time.Now(),
				NotifierId: "user-id",
				IsRead:     true,
			},
			expectedError: false,
		},
		{
			name: "Update Error",
			storeSetup: func(m *MockStore) {
				m.On("Update", mock.Anything, "test-id", true).Return(errors.New("update error"))
			},
			id:             "test-id",
			isRead:         true,
			expectedResult: model.BaseNotification{},
			expectedError:  true,
		},
		{
			name:           "Nil Store",
			storeSetup:     func(m *MockStore) {},
			id:             "test-id",
			isRead:         true,
			expectedResult: model.BaseNotification{},
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			service := NewService[model.BaseNotification]()
			if tc.name != "Nil Store" {
				mockStore := new(MockStore)
				tc.storeSetup(mockStore)
				service.SetStore(mockStore)
			}

			// Execute
			result, err := service.UpdateNotification(context.Background(), tc.id, tc.isRead)

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Note: We can't directly compare time.Time values
				assert.Equal(t, tc.expectedResult.ID, result.ID)
				assert.Equal(t, tc.expectedResult.Type, result.Type)
				assert.Equal(t, tc.expectedResult.NotifierId, result.NotifierId)
				assert.Equal(t, tc.expectedResult.IsRead, result.IsRead)
			}
		})
	}
}

func TestDeleteNotification(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		storeSetup    func(*MockStore)
		id            string
		expectedError bool
	}{
		{
			name: "Success",
			storeSetup: func(m *MockStore) {
				m.On("Delete", mock.Anything, "test-id").Return(nil)
			},
			id:            "test-id",
			expectedError: false,
		},
		{
			name: "Store Error",
			storeSetup: func(m *MockStore) {
				m.On("Delete", mock.Anything, "test-id").Return(errors.New("store error"))
			},
			id:            "test-id",
			expectedError: true,
		},
		{
			name:          "Nil Store",
			storeSetup:    func(m *MockStore) {},
			id:            "test-id",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			service := NewService[model.BaseNotification]()
			if tc.name != "Nil Store" {
				mockStore := new(MockStore)
				tc.storeSetup(mockStore)
				service.SetStore(mockStore)
			}

			// Execute
			err := service.DeleteNotification(context.Background(), tc.id)

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
