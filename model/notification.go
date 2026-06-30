package model

import (
	"context"
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

// Notification types
const (
	NotificationTypeNote                  NotificationType = "note"
	NotificationTypeFollow                NotificationType = "follow"
	NotificationTypeMention               NotificationType = "mention"
	NotificationTypeReply                 NotificationType = "reply"
	NotificationTypeRenote                NotificationType = "renote"
	NotificationTypeQuote                 NotificationType = "quote"
	NotificationTypeReaction              NotificationType = "reaction"
	NotificationTypePollEnded             NotificationType = "pollEnded"
	NotificationTypeScheduledNotePosted   NotificationType = "scheduledNotePosted"
	NotificationTypeScheduledNotePostFail NotificationType = "scheduledNotePostFailed"
	NotificationTypeReceiveFollowRequest  NotificationType = "receiveFollowRequest"
	NotificationTypeFollowRequestAccepted NotificationType = "followRequestAccepted"
	NotificationTypeRoleAssigned          NotificationType = "roleAssigned"
	NotificationTypeChatRoomInvitation    NotificationType = "chatRoomInvitationReceived"
	NotificationTypeAchievementEarned     NotificationType = "achievementEarned"
	NotificationTypeExportCompleted       NotificationType = "exportCompleted"
	NotificationTypeLogin                 NotificationType = "login"
	NotificationTypeCreateToken           NotificationType = "createToken"
	NotificationTypeApp                   NotificationType = "app"
	NotificationTypeTest                  NotificationType = "test"
	NotificationTypeGroupReaction         NotificationType = "reaction:grouped"
	NotificationTypeGroupRenote           NotificationType = "renote:grouped"
)

// BaseNotification contains common fields for all notification types
type BaseNotification struct {
	ID         Aid              `json:"id" dynamodbav:"id"`
	Type       NotificationType `json:"type" dynamodbav:"type"`
	CreatedAt  time.Time        `json:"createdAt" dynamodbav:"createdAt"`
	NotifieeId Aid              `json:"notifieeId" dynamodbav:"notifieeId"`
	NotifierId Aid              `json:"notifierId" dynamodbav:"notifierId"`
	IsRead     bool             `json:"isRead" dynamodbav:"isRead"`

	NoteId           Aid            `json:"noteId,omitempty" dynamodbav:"noteId,omitempty"`
	TargetNoteId     Aid            `json:"targetNoteId,omitempty" dynamodbav:"targetNoteId,omitempty"`
	NoteDraftId      Aid            `json:"noteDraftId,omitempty" dynamodbav:"noteDraftId,omitempty"`
	RoleId           Aid            `json:"roleId,omitempty" dynamodbav:"roleId,omitempty"`
	InvitationId     string         `json:"invitationId,omitempty" dynamodbav:"invitationId,omitempty"`
	Message          *string        `json:"message,omitempty" dynamodbav:"message,omitempty"`
	Reaction         string         `json:"reaction,omitempty" dynamodbav:"reaction,omitempty"`
	Achievement      Achievement    `json:"achievement,omitempty" dynamodbav:"achievement,omitempty"`
	ExportedEntity   string         `json:"exportedEntity,omitempty" dynamodbav:"exportedEntity,omitempty"`
	FileId           Aid            `json:"fileId,omitempty" dynamodbav:"fileId,omitempty"`
	CustomBody       string         `json:"customBody,omitempty" dynamodbav:"customBody,omitempty"`
	CustomHeader     *string        `json:"customHeader,omitempty" dynamodbav:"customHeader,omitempty"`
	CustomIcon       *string        `json:"customIcon,omitempty" dynamodbav:"customIcon,omitempty"`
	AppAccessTokenId *string        `json:"appAccessTokenId,omitempty" dynamodbav:"appAccessTokenId,omitempty"`
	Reactions        []ReactionUser `json:"reactions,omitempty" dynamodbav:"reactions,omitempty"`
	UserIds          []Aid          `json:"userIds,omitempty" dynamodbav:"userIds,omitempty"`
}

// FollowNotification represents a follow notification
type FollowNotification struct {
	BaseNotification
}

// MentionNotification represents a mention notification
type MentionNotification struct {
	BaseNotification
	NoteId Aid `json:"noteId"`
}

// ReplyNotification represents a reply notification
type ReplyNotification struct {
	BaseNotification
	NoteId Aid `json:"noteId"`
}

// RenoteNotification represents a renote notification
type RenoteNotification struct {
	BaseNotification
	NoteId Aid `json:"noteId"`
}

// QuoteNotification represents a quote notification
type QuoteNotification struct {
	BaseNotification
	NoteId Aid `json:"noteId"`
}

// ReactionNotification represents a reaction notification
type ReactionNotification struct {
	BaseNotification
	NoteId   Aid    `json:"noteId"`
	Reaction string `json:"reaction"`
}

// PollEndedNotification represents a poll ended notification
type PollEndedNotification struct {
	BaseNotification
	NoteId Aid `json:"noteId"`
}

// ReceiveFollowRequestNotification represents a receive follow request notification
type ReceiveFollowRequestNotification struct {
	BaseNotification
}

// FollowRequestAcceptedNotification represents a follow request accepted notification
type FollowRequestAcceptedNotification struct {
	BaseNotification
}

// RoleAssignedNotification represents a role assigned notification
type RoleAssignedNotification struct {
	BaseNotification
	RoleId Aid `json:"roleId"`
}

// Achievement is a type alias for string
type Achievement string

// AchievementEarnedNotification represents an achievement earned notification
type AchievementEarnedNotification struct {
	BaseNotification
	Achievement Achievement `json:"achievement"`
}

// AppNotification represents an app notification
type AppNotification struct {
	BaseNotification
	CustomBody       string `json:"customBody"`
	CustomHeader     string `json:"customHeader"`
	CustomIcon       string `json:"customIcon"`
	AppAccessTokenId string `json:"appAccessTokenId"`
}

// TestNotification represents a test notification
type TestNotification struct {
	BaseNotification
}

// ReactionUser represents a user who reacted
type ReactionUser struct {
	UserId   Aid    `json:"userId"`
	Reaction string `json:"reaction"`
}

// GroupReactionNotification represents a grouped reaction notification
type GroupReactionNotification struct {
	BaseNotification
	NoteId    Aid            `json:"noteId"`
	Reactions []ReactionUser `json:"reactions"`
}

// GroupRenoteNotification represents a grouped renote notification
type GroupRenoteNotification struct {
	BaseNotification
	NoteId  Aid   `json:"noteId"`
	UserIds []Aid `json:"userIds"`
}

// Notification is an interface that all notification types implement
type Notification interface {
	GetID() Aid
	GetType() NotificationType
	GetCreatedAt() time.Time
	GetNotifierId() Aid
	GetNotifieeId() Aid
	IsReadStatus() bool
}

// GetID returns the notification ID
func (n BaseNotification) GetID() Aid {
	return n.ID
}

// GetType returns the notification type
func (n BaseNotification) GetType() NotificationType {
	return n.Type
}

// GetCreatedAt returns the notification creation time
func (n BaseNotification) GetCreatedAt() time.Time {
	return n.CreatedAt
}

// GetNotifierId returns the notifier ID
func (n BaseNotification) GetNotifierId() Aid {
	return n.NotifierId
}

// GetNotifieeId returns the notification receiver ID
func (n BaseNotification) GetNotifieeId() Aid {
	return n.NotifieeId
}

// IsReadStatus returns whether the notification has been read
func (n BaseNotification) IsReadStatus() bool {
	return n.IsRead
}

// CreateAidNotification interface defines methods for creating different types of notifications
type CreateAidNotification interface {
	NoteNotification(ctx context.Context, notification MentionNotification)
	FollowNotification(ctx context.Context, notification FollowNotification)
	MentionNotification(ctx context.Context, notification MentionNotification)
	ReplyNotification(ctx context.Context, notification ReplyNotification)
	RenoteNotification(ctx context.Context, notification RenoteNotification)
	QuoteNotification(ctx context.Context, notification QuoteNotification)
	ReactionNotification(ctx context.Context, notification ReactionNotification)
	PollEndedNotification(ctx context.Context, notification PollEndedNotification)
	ReceiveFollowRequestNotification(ctx context.Context, notification ReceiveFollowRequestNotification)
	FollowRequestAcceptedNotification(ctx context.Context, notification FollowRequestAcceptedNotification)
	RoleAssignedNotification(ctx context.Context, notification RoleAssignedNotification)
	AchievementEarnedNotification(ctx context.Context, notification AchievementEarnedNotification)
	AppNotification(ctx context.Context, notification AppNotification)
	TestNotification(ctx context.Context, notification TestNotification)
	GroupReactionNotification(ctx context.Context, notification GroupReactionNotification)
	GroupRenoteNotification(ctx context.Context, notification GroupRenoteNotification)
}
