package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseNotificationPreservesReactionPayloadFields(t *testing.T) {
	body := []byte(`{
		"id": "notification-id",
		"type": "reaction",
		"createdAt": "2026-07-01T00:00:00Z",
		"notifieeId": "target-user-id",
		"notifierId": "actor-user-id",
		"isRead": true,
		"noteId": "note-id",
		"reaction": ":test:"
	}`)

	var notification BaseNotification
	require.NoError(t, json.Unmarshal(body, &notification))

	encoded, err := json.Marshal(notification)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(encoded, &got))
	assert.Equal(t, "note-id", got["noteId"])
	assert.Equal(t, ":test:", got["reaction"])
}

func TestBaseNotificationOmitsEmptyNotifierId(t *testing.T) {
	notification := BaseNotification{
		ID:         "notification-id",
		Type:       NotificationTypeTest,
		NotifieeId: "target-user-id",
		IsRead:     false,
	}

	encoded, err := json.Marshal(notification)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(encoded, &got))
	assert.NotContains(t, got, "notifierId")
}
