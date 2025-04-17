package dynamo

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mi-24v/miwkey-extension/model"
	"github.com/mi-24v/miwkey-extension/notification"
)

// NotificationStore implements the notification.Store interface for DynamoDB
type NotificationStore[T model.Notification] struct {
	client    *dynamodb.Client
	tableName string
}

// NewNotificationStore creates a new DynamoDB notification store
func NewNotificationStore[T model.Notification](client *dynamodb.Client, tableName string) *NotificationStore[T] {
	return &NotificationStore[T]{
		client:    client,
		tableName: tableName,
	}
}

// Create stores a new notification in DynamoDB
func (s *NotificationStore[T]) Create(ctx context.Context, notification T) error {
	// Convert notification to a map for DynamoDB
	item, err := attributevalue.MarshalMap(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Store in DynamoDB
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to put item in DynamoDB: %w", err)
	}

	return nil
}

// List retrieves all notifications for a user from DynamoDB
func (s *NotificationStore[T]) List(ctx context.Context, userId string) ([]T, error) {
	// Query DynamoDB for notifications for this user
	// Note: This assumes a GSI on userId
	result, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("UserIdIndex"),
		KeyConditionExpression: aws.String("notifierId = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userId},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications: %w", err)
	}

	// Convert DynamoDB items to notifications
	var notifications []T
	for _, item := range result.Items {
		var notification T
		if err := attributevalue.UnmarshalMap(item, &notification); err != nil {
			continue
		}
		notifications = append(notifications, notification)
	}

	return notifications, nil
}

// Get retrieves a specific notification by ID from DynamoDB
func (s *NotificationStore[T]) Get(ctx context.Context, id string) (T, error) {
	var empty T

	// Get the notification from DynamoDB
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return empty, fmt.Errorf("failed to get notification: %w", err)
	}

	if result.Item == nil {
		return empty, nil
	}

	// Unmarshal the item into a notification
	var notification T
	if err := attributevalue.UnmarshalMap(result.Item, &notification); err != nil {
		return empty, fmt.Errorf("failed to unmarshal notification: %w", err)
	}

	return notification, nil
}

// Update updates a notification in DynamoDB (currently only supports marking as read)
func (s *NotificationStore[T]) Update(ctx context.Context, id string, isRead bool) error {
	// Update the notification in DynamoDB
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		UpdateExpression: aws.String("SET isRead = :isRead"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":isRead": &types.AttributeValueMemberBOOL{Value: isRead},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update notification: %w", err)
	}

	return nil
}

// Delete deletes a notification from DynamoDB
func (s *NotificationStore[T]) Delete(ctx context.Context, id string) error {
	// Delete the notification from DynamoDB
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	return nil
}

// Register registers the notification store with the notification service
func (s *NotificationStore[T]) Register(service *notification.Service[T]) {
	service.SetStore(s)
}
