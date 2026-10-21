package dynamo

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mi-24v/miwkey-extension/model"
	"github.com/mi-24v/miwkey-extension/notification"
)

// DynamoDBClient defines the interface for DynamoDB operations
type DynamoDBClient interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

// NotificationStoreImpl implements the notification.Store interface for DynamoDB
type NotificationStoreImpl[T model.Notification] struct {
	client     DynamoDBClient
	tableName  string
	marshalMap func(interface{}) (map[string]types.AttributeValue, error)
}

const notificationIdIndex = "notificationId-index"

// NewNotificationStore creates a new DynamoDB notification store
func NewNotificationStore[T model.Notification](client DynamoDBClient, tableName string) *NotificationStoreImpl[T] {
	return &NotificationStoreImpl[T]{
		client:     client,
		tableName:  tableName,
		marshalMap: attributevalue.MarshalMap,
	}
}

// Create stores a new notification in DynamoDB
func (s *NotificationStoreImpl[T]) Create(ctx context.Context, notification T) error {
	sortKey, err := s.sortKey(notification)
	if err != nil {
		return err
	}

	// Convert notification to a map for DynamoDB
	item, err := s.marshalMap(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// add sort key attributes
	item["notifieeId"] = &types.AttributeValueMemberS{Value: string(notification.GetNotifieeId())}
	item["sortKey"] = &types.AttributeValueMemberS{Value: sortKey}

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
func (s *NotificationStoreImpl[T]) List(ctx context.Context, userId string, opts notification.ListOptions) ([]T, error) {
	return s.queryWithOptions(ctx, userId, opts)
}

// Get retrieves a specific notification by ID from DynamoDB
func (s *NotificationStoreImpl[T]) Get(ctx context.Context, id string) (T, error) {
	var empty T

	item, err := s.findByID(ctx, id)
	if err != nil {
		return empty, err
	}

	var notification T
	if err := attributevalue.UnmarshalMap(item, &notification); err != nil {
		return empty, fmt.Errorf("failed to unmarshal notification: %w", err)
	}

	return notification, nil
}

// Update updates a notification in DynamoDB (currently only supports marking as read)
func (s *NotificationStoreImpl[T]) Update(ctx context.Context, id string, isRead bool) error {
	keys, err := s.keysByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key:       keys,
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
func (s *NotificationStoreImpl[T]) Delete(ctx context.Context, id string) error {
	keys, err := s.keysByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key:       keys,
	})
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	return nil
}

// DeleteByUser deletes all notifications for a user
func (s *NotificationStoreImpl[T]) DeleteByUser(ctx context.Context, userId string) error {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("notifieeId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userId},
		},
	}

	out, err := s.client.Query(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to query notifications for delete: %w", err)
	}

	for _, item := range out.Items {
		sortAttr, ok := item["sortKey"].(*types.AttributeValueMemberS)
		if !ok {
			continue
		}
		_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(s.tableName),
			Key: map[string]types.AttributeValue{
				"notifieeId": &types.AttributeValueMemberS{Value: userId},
				"sortKey":    &types.AttributeValueMemberS{Value: sortAttr.Value},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// queryWithOptions fetches notifications with pagination and filtering.
func (s *NotificationStoreImpl[T]) queryWithOptions(ctx context.Context, userId string, opts notification.ListOptions) ([]T, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("notifieeId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userId},
		},
		ScanIndexForward: aws.Bool(false),
	}

	if opts.Limit > 0 {
		input.Limit = aws.Int32(int32(opts.Limit))
	}

	startKey, hasStart := s.sortKeyFromID(opts.SinceID)
	endKey, hasEnd := s.sortKeyFromID(opts.UntilID)

	if hasStart && hasEnd {
		input.KeyConditionExpression = aws.String("notifieeId = :uid AND sortKey BETWEEN :start AND :end")
		input.ExpressionAttributeValues[":start"] = &types.AttributeValueMemberS{Value: startKey}
		input.ExpressionAttributeValues[":end"] = &types.AttributeValueMemberS{Value: endKey}
	} else if hasStart {
		input.KeyConditionExpression = aws.String("notifieeId = :uid AND sortKey > :start")
		input.ExpressionAttributeValues[":start"] = &types.AttributeValueMemberS{Value: startKey}
	} else if hasEnd {
		input.KeyConditionExpression = aws.String("notifieeId = :uid AND sortKey <= :end")
		input.ExpressionAttributeValues[":end"] = &types.AttributeValueMemberS{Value: endKey}
	}

	result, err := s.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications: %w", err)
	}

	var notifications []T
	for _, item := range result.Items {
		var notification T
		if err := attributevalue.UnmarshalMap(item, &notification); err != nil {
			log.Printf("failed to unmarshal notification: %v", err)
			continue
		}
		if filterTypes(notification.GetType(), opts.IncludeTypes, opts.ExcludeTypes) {
			notifications = append(notifications, notification)
		}
	}

	return notifications, nil
}

// sortKey builds a deterministic sort key using createdAt if present, otherwise aid-derived time.
func (s *NotificationStoreImpl[T]) sortKey(notification T) (string, error) {
	createdAt := notification.GetCreatedAt()
	if createdAt.IsZero() {
		if t, ok := model.ParseAid(notification.GetID()); ok {
			createdAt = t
		}
	}
	if createdAt.IsZero() {
		return "", fmt.Errorf("notification missing createdAt and aid parse failed")
	}
	return fmt.Sprintf("%013d#%s", createdAt.UnixMilli(), notification.GetID()), nil
}

// sortKeyFromID attempts to derive sort key boundary from id by parsing aid.
func (s *NotificationStoreImpl[T]) sortKeyFromID(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	if t, ok := model.ParseAid(model.Aid(id)); ok {
		return fmt.Sprintf("%013d#%s", t.UnixMilli(), id), true
	}
	return "", false
}

func filterTypes(t model.NotificationType, include, exclude []model.NotificationType) bool {
	if len(include) > 0 {
		for _, inc := range include {
			if t == inc {
				return true
			}
		}
		return false
	}
	for _, exc := range exclude {
		if t == exc {
			return false
		}
	}
	return true
}

// keysByID looks up the primary keys via GSI for update/delete operations.
func (s *NotificationStoreImpl[T]) keysByID(ctx context.Context, id string) (map[string]types.AttributeValue, error) {
	item, err := s.findByID(ctx, id)
	if err != nil {
		return nil, err
	}

	notifieeAttr, ok := item["notifieeId"].(*types.AttributeValueMemberS)
	sortAttr, ok2 := item["sortKey"].(*types.AttributeValueMemberS)
	if !ok || !ok2 {
		return nil, fmt.Errorf("notification keys missing for id %s", id)
	}

	return map[string]types.AttributeValue{
		"notifieeId": &types.AttributeValueMemberS{Value: notifieeAttr.Value},
		"sortKey":    &types.AttributeValueMemberS{Value: sortAttr.Value},
	}, nil
}

// findByID fetches an item using the notificationId GSI.
func (s *NotificationStoreImpl[T]) findByID(ctx context.Context, id string) (map[string]types.AttributeValue, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String(notificationIdIndex),
		KeyConditionExpression: aws.String("id = :id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberS{Value: id},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query by id: %w", err)
	}
	if len(out.Items) == 0 {
		return nil, &notification.NotFoundError{Resource: "Notification", ID: id}
	}
	return out.Items[0], nil
}
