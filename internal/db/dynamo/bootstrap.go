package dynamo

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

	return store, nil
}
