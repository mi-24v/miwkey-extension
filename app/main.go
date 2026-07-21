package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mi-24v/miwkey-extension/internal/db/dynamo"
	"github.com/mi-24v/miwkey-extension/internal/infra"
	"github.com/mi-24v/miwkey-extension/model"
	"github.com/mi-24v/miwkey-extension/notification"
)

const healthCheckURL = "http://127.0.0.1:8080/healthz"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := runHealthCheck(context.Background(), healthCheckURL); err != nil {
			log.Printf("healthcheck failed: %v", err)
			os.Exit(1)
		}
		return
	}

	// Auth
	secret, err := infra.LoadAuthSecret(context.Background())
	if err != nil {
		log.Fatalf("Failed to load auth secret: %v", err)
	}

	// Initialize notification service with BaseNotification as the generic type
	notificationService := notification.NewService[model.BaseNotification]()

	store, err := dynamo.InitDynamoDB[model.BaseNotification](context.Background())
	if err != nil {
		log.Fatalf("Failed to initialize DynamoDB: %v", err)
	}

	// Register store with service
	notification.RegisterStore(notificationService, store)

	e := newHTTPServer(secret, notificationService)

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

func newHTTPServer(secret string, notificationService notification.Service[model.BaseNotification]) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.GET("/healthz", handleHealthz)

	api := e.Group("/api/v1")
	api.Use(infra.NewAuthMiddleware(secret))
	notification.RegisterHandlersWithGroup[model.BaseNotification](api, notificationService)

	return e
}

func handleHealthz(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

func runHealthCheck(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send healthcheck request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("healthcheck returned status %d", res.StatusCode)
	}

	return nil
}
