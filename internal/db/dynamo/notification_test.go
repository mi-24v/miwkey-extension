package dynamo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mi-24v/miwkey-extension/model"
	"github.com/mi-24v/miwkey-extension/notification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDynamoDBClient is a mock implementation of the DynamoDB client
type MockDynamoDBClient struct {
	mock.Mock
}

func (m *MockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}

func (m *MockDynamoDBClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*dynamodb.QueryOutput), args.Error(1)
}

func (m *MockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*dynamodb.GetItemOutput), args.Error(1)
}

func (m *MockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*dynamodb.UpdateItemOutput), args.Error(1)
}

func (m *MockDynamoDBClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*dynamodb.DeleteItemOutput), args.Error(1)
}

func TestNewNotificationStore(t *testing.T) {
	// Setup
	mockClient := new(MockDynamoDBClient)
	tableName := "test-table"

	// Execute
	store := NewNotificationStore[model.BaseNotification](mockClient, tableName)

	// Verify
	assert.NotNil(t, store)
	assert.Equal(t, mockClient, store.client)
	assert.Equal(t, tableName, store.tableName)
}

func TestCreate(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		notification  model.BaseNotification
		clientSetup   func(*MockDynamoDBClient)
		expectedError bool
	}{
		{
			name: "Success",
			notification: model.BaseNotification{
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				NotifieeId: "user-id",
				NotifierId: "user-id",
				CreatedAt:  time.Now(),
				IsRead:     false,
			},
			clientSetup: func(m *MockDynamoDBClient) {
				m.On("PutItem", mock.Anything, mock.AnythingOfType("*dynamodb.PutItemInput")).Return(&dynamodb.PutItemOutput{}, nil)
			},
			expectedError: false,
		},
		{
			name: "Marshal Error",
			notification: model.BaseNotification{
				// This would cause a marshal error in a real scenario, but for testing we'll mock the error
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				NotifieeId: "user-id",
				NotifierId: "user-id",
				CreatedAt:  time.Now(),
				IsRead:     false,
			},
			clientSetup: func(m *MockDynamoDBClient) {
				// No setup needed as we'll mock the marshal error
			},
			expectedError: true,
		},
		{
			name: "DynamoDB Error",
			notification: model.BaseNotification{
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				NotifieeId: "user-id",
				NotifierId: "user-id",
				CreatedAt:  time.Now(),
				IsRead:     false,
			},
			clientSetup: func(m *MockDynamoDBClient) {
				m.On("PutItem", mock.Anything, mock.AnythingOfType("*dynamodb.PutItemInput")).Return(&dynamodb.PutItemOutput{}, errors.New("dynamodb error"))
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockDynamoDBClient)
			if tc.name != "Marshal Error" {
				tc.clientSetup(mockClient)
			}
			store := NewNotificationStore[model.BaseNotification](mockClient, "test-table")

			// Execute
			var err error
			if tc.name == "Marshal Error" {
				// Mock the marshal error by using a custom marshal function
				marshalMap := func(in interface{}) (map[string]types.AttributeValue, error) {
					return nil, errors.New("marshal error")
				}
				store.marshalMap = marshalMap
				err = store.Create(context.Background(), tc.notification)
				store.marshalMap = attributevalue.MarshalMap
			} else {
				err = store.Create(context.Background(), tc.notification)
			}

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestList(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		userId         string
		clientSetup    func(*MockDynamoDBClient)
		expectedResult []model.BaseNotification
		expectedError  bool
	}{
		{
			name:   "Success",
			userId: "user-id",
			clientSetup: func(m *MockDynamoDBClient) {
				n1 := model.BaseNotification{ID: "test-id-1", Type: model.NotificationTypeTest, NotifieeId: "user-id", NotifierId: "user-id", IsRead: false, CreatedAt: time.Now()}
				n2 := model.BaseNotification{ID: "test-id-2", Type: model.NotificationTypeTest, NotifieeId: "user-id", NotifierId: "user-id", IsRead: true, CreatedAt: time.Now()}
				item1, _ := attributevalue.MarshalMap(n1)
				item1["sortKey"] = &types.AttributeValueMemberS{Value: "000#test-id-1"}
				item2, _ := attributevalue.MarshalMap(n2)
				item2["sortKey"] = &types.AttributeValueMemberS{Value: "000#test-id-2"}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item1, item2}}, nil)
			},
			expectedResult: []model.BaseNotification{
				{
					ID:         "test-id-1",
					Type:       model.NotificationTypeTest,
					NotifierId: "user-id",
					NotifieeId: "user-id",
					IsRead:     false,
				},
				{
					ID:         "test-id-2",
					Type:       model.NotificationTypeTest,
					NotifierId: "user-id",
					NotifieeId: "user-id",
					IsRead:     true,
				},
			},
			expectedError: false,
		},
		{
			name:   "DynamoDB Error",
			userId: "user-id",
			clientSetup: func(m *MockDynamoDBClient) {
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{}, errors.New("dynamodb error"))
			},
			expectedResult: nil,
			expectedError:  true,
		},
		{
			name:   "Unmarshal Error",
			userId: "user-id",
			clientSetup: func(m *MockDynamoDBClient) {
				// Create a mock response with invalid items that will cause unmarshal errors
				items := []map[string]types.AttributeValue{
					{
						"id":         &types.AttributeValueMemberS{Value: "test-id-1"},
						"type":       &types.AttributeValueMemberS{Value: "test"},
						"notifierId": &types.AttributeValueMemberS{Value: "user-id"},
						"notifieeId": &types.AttributeValueMemberS{Value: "user-id"},
						"sortKey":    &types.AttributeValueMemberS{Value: "000#test-id-1"},
						"isRead":     &types.AttributeValueMemberS{Value: "not-a-bool"}, // This will cause an unmarshal error
					},
				}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{
					Items: items,
				}, nil)
			},
			expectedResult: []model.BaseNotification{}, // Empty slice because the item will be skipped due to unmarshal error
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockDynamoDBClient)
			tc.clientSetup(mockClient)
			store := NewNotificationStore[model.BaseNotification](mockClient, "test-table")

			// Execute
			result, err := store.List(context.Background(), tc.userId, notification.ListOptions{})

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tc.name == "Unmarshal Error" {
					// For the unmarshal error case, we just check that the result is empty
					assert.Empty(t, result)
				} else {
					// For other cases, we check that the result matches the expected result
					assert.Equal(t, len(tc.expectedResult), len(result))
					for i, notification := range result {
						assert.Equal(t, tc.expectedResult[i].ID, notification.ID)
						assert.Equal(t, tc.expectedResult[i].Type, notification.Type)
						assert.Equal(t, tc.expectedResult[i].NotifierId, notification.NotifierId)
						assert.Equal(t, tc.expectedResult[i].IsRead, notification.IsRead)
					}
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		id             string
		clientSetup    func(*MockDynamoDBClient)
		expectedResult model.BaseNotification
		expectedError  bool
	}{
		{
			name: "Success",
			id:   "test-id",
			clientSetup: func(m *MockDynamoDBClient) {
				n := model.BaseNotification{ID: "test-id", Type: model.NotificationTypeTest, NotifieeId: "user-id", NotifierId: "user-id", IsRead: false, CreatedAt: time.Now()}
				item, _ := attributevalue.MarshalMap(n)
				item["sortKey"] = &types.AttributeValueMemberS{Value: "000#test-id"}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item}}, nil)
			},
			expectedResult: model.BaseNotification{
				ID:         "test-id",
				Type:       model.NotificationTypeTest,
				NotifierId: "user-id",
				NotifieeId: "user-id",
				IsRead:     false,
			},
			expectedError: false,
		},
		{
			name: "Not Found",
			id:   "test-id",
			clientSetup: func(m *MockDynamoDBClient) {
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)
			},
			expectedResult: model.BaseNotification{},
			expectedError:  true,
		},
		{
			name: "DynamoDB Error",
			id:   "test-id",
			clientSetup: func(m *MockDynamoDBClient) {
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{}, errors.New("dynamodb error"))
			},
			expectedResult: model.BaseNotification{},
			expectedError:  true,
		},
		{
			name: "Unmarshal Error",
			id:   "test-id",
			clientSetup: func(m *MockDynamoDBClient) {
				item := map[string]types.AttributeValue{
					"id":         &types.AttributeValueMemberS{Value: "test-id"},
					"type":       &types.AttributeValueMemberS{Value: "test"},
					"notifierId": &types.AttributeValueMemberS{Value: "user-id"},
					"notifieeId": &types.AttributeValueMemberS{Value: "user-id"},
					"sortKey":    &types.AttributeValueMemberS{Value: "000#test-id"},
					"isRead":     &types.AttributeValueMemberS{Value: "not-a-bool"}, // This will cause an unmarshal error
				}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item}}, nil)
			},
			expectedResult: model.BaseNotification{},
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockDynamoDBClient)
			tc.clientSetup(mockClient)
			store := NewNotificationStore[model.BaseNotification](mockClient, "test-table")

			// Execute
			result, err := store.Get(context.Background(), tc.id)

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult.ID, result.ID)
				assert.Equal(t, tc.expectedResult.Type, result.Type)
				assert.Equal(t, tc.expectedResult.NotifierId, result.NotifierId)
				assert.Equal(t, tc.expectedResult.IsRead, result.IsRead)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		id            string
		isRead        bool
		clientSetup   func(*MockDynamoDBClient)
		expectedError bool
	}{
		{
			name:   "Success",
			id:     "test-id",
			isRead: true,
			clientSetup: func(m *MockDynamoDBClient) {
				item := map[string]types.AttributeValue{
					"id":         &types.AttributeValueMemberS{Value: "test-id"},
					"type":       &types.AttributeValueMemberS{Value: "test"},
					"notifieeId": &types.AttributeValueMemberS{Value: "user-id"},
					"notifierId": &types.AttributeValueMemberS{Value: "user-id"},
					"sortKey":    &types.AttributeValueMemberS{Value: "000#test-id"},
					"isRead":     &types.AttributeValueMemberBOOL{Value: false},
				}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item}}, nil)
				m.On("UpdateItem", mock.Anything, mock.AnythingOfType("*dynamodb.UpdateItemInput")).Return(&dynamodb.UpdateItemOutput{}, nil)
			},
			expectedError: false,
		},
		{
			name:   "DynamoDB Error",
			id:     "test-id",
			isRead: true,
			clientSetup: func(m *MockDynamoDBClient) {
				item := map[string]types.AttributeValue{
					"id":         &types.AttributeValueMemberS{Value: "test-id"},
					"type":       &types.AttributeValueMemberS{Value: "test"},
					"notifieeId": &types.AttributeValueMemberS{Value: "user-id"},
					"notifierId": &types.AttributeValueMemberS{Value: "user-id"},
					"sortKey":    &types.AttributeValueMemberS{Value: "000#test-id"},
					"isRead":     &types.AttributeValueMemberBOOL{Value: false},
				}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item}}, nil)
				m.On("UpdateItem", mock.Anything, mock.AnythingOfType("*dynamodb.UpdateItemInput")).Return(&dynamodb.UpdateItemOutput{}, errors.New("dynamodb error"))
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockDynamoDBClient)
			tc.clientSetup(mockClient)
			store := NewNotificationStore[model.BaseNotification](mockClient, "test-table")

			// Execute
			err := store.Update(context.Background(), tc.id, tc.isRead)

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		id            string
		clientSetup   func(*MockDynamoDBClient)
		expectedError bool
	}{
		{
			name: "Success",
			id:   "test-id",
			clientSetup: func(m *MockDynamoDBClient) {
				item := map[string]types.AttributeValue{
					"id":         &types.AttributeValueMemberS{Value: "test-id"},
					"type":       &types.AttributeValueMemberS{Value: "test"},
					"notifieeId": &types.AttributeValueMemberS{Value: "user-id"},
					"notifierId": &types.AttributeValueMemberS{Value: "user-id"},
					"sortKey":    &types.AttributeValueMemberS{Value: "000#test-id"},
					"isRead":     &types.AttributeValueMemberBOOL{Value: false},
				}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item}}, nil)
				m.On("DeleteItem", mock.Anything, mock.AnythingOfType("*dynamodb.DeleteItemInput")).Return(&dynamodb.DeleteItemOutput{}, nil)
			},
			expectedError: false,
		},
		{
			name: "DynamoDB Error",
			id:   "test-id",
			clientSetup: func(m *MockDynamoDBClient) {
				item := map[string]types.AttributeValue{
					"id":         &types.AttributeValueMemberS{Value: "test-id"},
					"type":       &types.AttributeValueMemberS{Value: "test"},
					"notifieeId": &types.AttributeValueMemberS{Value: "user-id"},
					"notifierId": &types.AttributeValueMemberS{Value: "user-id"},
					"sortKey":    &types.AttributeValueMemberS{Value: "000#test-id"},
					"isRead":     &types.AttributeValueMemberBOOL{Value: false},
				}
				m.On("Query", mock.Anything, mock.AnythingOfType("*dynamodb.QueryInput")).Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item}}, nil)
				m.On("DeleteItem", mock.Anything, mock.AnythingOfType("*dynamodb.DeleteItemInput")).Return(&dynamodb.DeleteItemOutput{}, errors.New("dynamodb error"))
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockClient := new(MockDynamoDBClient)
			tc.clientSetup(mockClient)
			store := NewNotificationStore[model.BaseNotification](mockClient, "test-table")

			// Execute
			err := store.Delete(context.Background(), tc.id)

			// Verify
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
