package infra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

const (
	envAuthSecret     = "AUTH_SECRET"
	envAuthSecretName = "AUTH_SECRET_NAME"
)

// LoadAuthSecret resolves the shared JWT secret.
// Priority: ENV AUTH_SECRET -> Secrets Manager (AUTH_SECRET_NAME) -> error.
func LoadAuthSecret(ctx context.Context) (string, error) {
	if v := os.Getenv(envAuthSecret); v != "" {
		return v, nil
	}

	name := os.Getenv(envAuthSecretName)
	if name == "" {
		return "", errors.New("auth secret not configured (missing AUTH_SECRET or AUTH_SECRET_NAME)")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("load AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg)
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
	if err != nil {
		return "", fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString == nil {
		return "", errors.New("secret string is empty")
	}

	return *out.SecretString, nil
}

// NewAuthMiddleware returns an Echo middleware that verifies Bearer JWT with HS256.
func NewAuthMiddleware(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.NewHTTPError(http.StatusUnauthorized, "invalid signing method")
				}
				return []byte(secret), nil
			})
			if err != nil || token == nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}
			return next(c)
		}
	}
}
