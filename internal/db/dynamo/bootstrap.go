package dynamo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mi-24v/miwkey-extension/model"
)

// InitDynamoDB initializes the DynamoDB client and creates the notification store
func InitDynamoDB[T model.Notification](ctx context.Context) (*NotificationStoreImpl[T], error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Get table name from environment variable or use default
	tableName := os.Getenv("DYNAMODB_NOTIFICATION_TABLE")
	if tableName == "" {
		tableName = "Notifications"
	}

	// Create notification store
	store := NewNotificationStore[T](client, tableName)

	// Optionally ensure table and GSI exist (opt-in to avoid unexpected infra changes)
	if os.Getenv("DYNAMODB_AUTO_CREATE") == "true" {
		if err := ensureNotificationTable(ctx, client, tableName); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// ensureNotificationTable creates the notifications table with the required GSI if it does not exist.
func ensureNotificationTable(ctx context.Context, client *dynamodb.Client, tableName string) error {
	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(tableName)})
	if err == nil {
		return nil
	}

	var nfe *types.ResourceNotFoundException
	if !errors.As(err, &nfe) {
		return fmt.Errorf("describe table: %w", err)
	}

	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("notifieeId"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("sortKey"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("notifieeId"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("sortKey"), KeyType: types.KeyTypeRange},
		},
		BillingMode: types.BillingModePayPerRequest,
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName:  aws.String(notificationIdIndex),
				KeySchema:  []types.KeySchemaElement{{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash}},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(tableName)}, time.Minute); err != nil {
		return fmt.Errorf("wait for table to be active: %w", err)
	}

	return nil
}
