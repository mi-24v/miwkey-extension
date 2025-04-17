package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mi-24v/miwkey-extension/internal/db/dynamo"
	"github.com/mi-24v/miwkey-extension/model"
	"github.com/mi-24v/miwkey-extension/notification"
)

func main() {
	// Create a new Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Initialize notification service with BaseNotification as the generic type
	notificationService := notification.NewService[model.BaseNotification]()

	// Register DynamoDB store with the service
	err := dynamo.RegisterNotificationStore(context.Background(), notificationService)
	if err != nil {
		e.Logger.Fatalf("Failed to register notification store: %v", err)
	}

	// Register handlers
	notification.RegisterHandlers[model.BaseNotification](e, notificationService)

	// Start server
	go func() {
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}

	log.Println("Server gracefully stopped")
}
