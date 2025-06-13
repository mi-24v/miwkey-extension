package notification

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mi-24v/miwkey-extension/model"
)

type NotificationHandler[T model.Notification] interface {
	GetNotifications(c echo.Context) error
	CreateNotification(c echo.Context) error
	GetNotification(c echo.Context) error
	UpdateNotification(c echo.Context) error
	DeleteNotification(c echo.Context) error
}

// NotificationHandlerImpl handles HTTP requests for notifications
type NotificationHandlerImpl[T model.Notification] struct {
	service Service[T]
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler[T model.Notification](service Service[T]) *NotificationHandlerImpl[T] {
	return &NotificationHandlerImpl[T]{
		service: service,
	}
}

// RegisterHandlers registers all notification API handlers with the Echo instance
func RegisterHandlers[T model.Notification](e *echo.Echo, service Service[T]) {
	handler := NewNotificationHandler(service)

	// Group all notification routes under /api/v1
	g := e.Group("/api/v1")

	// Register routes
	g.GET("/notifications", handler.GetNotifications)
	g.POST("/notifications", handler.CreateNotification)
	g.GET("/notifications/:notificationId", handler.GetNotification)
	g.PUT("/notifications/:notificationId", handler.UpdateNotification)
	g.DELETE("/notifications/:notificationId", handler.DeleteNotification)
}

// GetNotifications handles GET /notifications
func (h *NotificationHandlerImpl[T]) GetNotifications(c echo.Context) error {
	// Get user ID from query parameter
	userId := c.QueryParam("userId")
	if userId == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "userId is required",
		})
	}

	// Get notifications from service
	notifications, err := h.service.GetNotifications(c.Request().Context(), userId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, notifications)
}

// CreateNotification handles POST /notifications
func (h *NotificationHandlerImpl[T]) CreateNotification(c echo.Context) error {
	// Parse request body
	var notification T
	if err := c.Bind(&notification); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Create notification
	createdNotification, err := h.service.CreateNotification(c.Request().Context(), notification)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, createdNotification)
}

// GetNotification handles GET /notifications/:notificationId
func (h *NotificationHandlerImpl[T]) GetNotification(c echo.Context) error {
	// Get notification ID from path parameter
	notificationId := c.Param("notificationId")
	if notificationId == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "notificationId is required",
		})
	}

	// Get notification from service
	notification, err := h.service.GetNotification(c.Request().Context(), notificationId)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, notification)
}

// UpdateNotification handles PUT /notifications/:notificationId
func (h *NotificationHandlerImpl[T]) UpdateNotification(c echo.Context) error {
	// Get notification ID from path parameter
	notificationId := c.Param("notificationId")
	if notificationId == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "notificationId is required",
		})
	}

	// Parse request body
	var updateRequest struct {
		IsRead bool `json:"isRead"`
	}
	if err := c.Bind(&updateRequest); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Update notification
	updatedNotification, err := h.service.UpdateNotification(c.Request().Context(), notificationId, updateRequest.IsRead)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, updatedNotification)
}

// DeleteNotification handles DELETE /notifications/:notificationId
func (h *NotificationHandlerImpl[T]) DeleteNotification(c echo.Context) error {
	// Get notification ID from path parameter
	notificationId := c.Param("notificationId")
	if notificationId == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "notificationId is required",
		})
	}

	// Delete notification
	err := h.service.DeleteNotification(c.Request().Context(), notificationId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}
