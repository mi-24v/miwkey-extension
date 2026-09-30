package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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

const (
	listenAddr         = ":8080"
	healthCheckAddr    = "127.0.0.1:8080"
	certificateTimeout = 10 * time.Second
)

func main() {
	serverConfig, err := infra.LoadServerConfig(os.Getenv)

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		// The healthcheck only needs the non-secret settings and never reads
		// the certificate secret.
		if err != nil {
			log.Printf("healthcheck failed: invalid server config: %v", err)
			os.Exit(1)
		}
		url, tlsConfig := healthCheckTarget(serverConfig)
		if err := runHealthCheck(context.Background(), url, tlsConfig); err != nil {
			log.Printf("healthcheck failed: %v", err)
			os.Exit(1)
		}
		return
	}

	if err != nil {
		log.Fatalf("Invalid server config: %v", err)
	}

	// TLS certificate
	var loaded *infra.LoadedCertificate
	var tlsConfig *tls.Config
	if serverConfig.Mode == infra.TLSModeTLS {
		cert, err := loadCertificate(serverConfig)
		if err != nil {
			log.Fatalf("Failed to load TLS certificate: %v", err)
		}
		loaded = &cert
		tlsConfig = infra.NewServerTLSConfig(cert.Certificate)
	} else {
		log.Println("WARNING: TLS_MODE=http serves plaintext HTTP; use it only for local development")
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
	srv := newServer(e, tlsConfig)

	ln, err := listenAndAnnounce(listenAddr, srv, loaded, func(ctx context.Context) string {
		return infra.FetchTaskARN(ctx, http.DefaultClient, os.Getenv)
	}, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Start server
	go func() {
		if err := serve(srv, ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server stopped: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("Server gracefully stopped")
}

// loadCertificate reads and verifies the certificate secret against the
// system roots within certificateTimeout.
func loadCertificate(cfg infra.ServerConfig) (infra.LoadedCertificate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), certificateTimeout)
	defer cancel()

	api, err := infra.NewSecretsManagerClient(ctx)
	if err != nil {
		return infra.LoadedCertificate{}, err
	}

	return infra.LoadServerCertificate(ctx, api, cfg.CertSecretARN, cfg.ServerName, nil, time.Now())
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

// newServer serves TLS when tlsConfig is set and plain HTTP otherwise.
func newServer(handler http.Handler, tlsConfig *tls.Config) *http.Server {
	return &http.Server{
		Addr:      listenAddr,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}
}

func serve(srv *http.Server, ln net.Listener) error {
	if srv.TLSConfig != nil {
		return srv.ServeTLS(ln, "", "")
	}
	return srv.Serve(ln)
}

// listenAndAnnounce binds the listener and, in TLS mode, logs the certificate
// metadata only after the bind and the TLS setup have succeeded. The task ARN
// lookup gets a fresh context so it never inherits the finished
// certificate-loading context.
func listenAndAnnounce(addr string, srv *http.Server, loaded *infra.LoadedCertificate, fetchTaskARN func(context.Context) string, logOut io.Writer) (net.Listener, error) {
	if loaded != nil && (srv.TLSConfig == nil || len(srv.TLSConfig.Certificates) == 0) {
		return nil, errors.New("TLS config has no server certificate")
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	if loaded != nil {
		if err := infra.LogCertificateLoaded(logOut, *loaded, fetchTaskARN(context.Background())); err != nil {
			_ = ln.Close()
			return nil, fmt.Errorf("log TLS certificate metadata: %w", err)
		}
	}

	return ln, nil
}

// healthCheckTarget returns the loopback healthcheck URL. In TLS mode the
// server certificate is verified against the system roots for ServerName.
func healthCheckTarget(cfg infra.ServerConfig) (string, *tls.Config) {
	if cfg.Mode == infra.TLSModeHTTP {
		return "http://" + healthCheckAddr + "/healthz", nil
	}

	return "https://" + healthCheckAddr + "/healthz", &tls.Config{
		ServerName: cfg.ServerName,
		MinVersion: tls.VersionTLS12,
	}
}

func runHealthCheck(ctx context.Context, url string, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}

	client := http.DefaultClient
	if tlsConfig != nil {
		// No proxy (nil Proxy) and no redirects, so the result always comes
		// from the verified TLS connection to the loopback listener.
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		defer transport.CloseIdleConnections()
		client = &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send healthcheck request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("healthcheck returned status %d", res.StatusCode)
	}

	return nil
}
