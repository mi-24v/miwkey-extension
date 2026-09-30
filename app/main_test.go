package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mi-24v/miwkey-extension/internal/infra"
	"github.com/mi-24v/miwkey-extension/internal/tlstest"
	"github.com/mi-24v/miwkey-extension/model"
	"github.com/mi-24v/miwkey-extension/notification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServerName = "ext.example.test"

type stubNotificationService struct{}

func (stubNotificationService) CreateNotification(context.Context, model.BaseNotification) (model.BaseNotification, error) {
	return model.BaseNotification{}, errors.New("not implemented")
}

func (stubNotificationService) GetNotifications(context.Context, string, notification.ListOptions) ([]model.BaseNotification, error) {
	return nil, errors.New("not implemented")
}

func (stubNotificationService) GetNotification(context.Context, string) (model.BaseNotification, error) {
	return model.BaseNotification{}, errors.New("not implemented")
}

func (stubNotificationService) UpdateNotification(context.Context, string, bool) (model.BaseNotification, error) {
	return model.BaseNotification{}, errors.New("not implemented")
}

func (stubNotificationService) DeleteNotification(context.Context, string) error {
	return errors.New("not implemented")
}

func (stubNotificationService) DeleteNotificationsByUser(context.Context, string) error {
	return errors.New("not implemented")
}

func TestHealthzDoesNotRequireAuthorization(t *testing.T) {
	e := newHTTPServer("shared-secret", stubNotificationService{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestNotificationRoutesStillRequireAuthorization(t *testing.T) {
	e := newHTTPServer("shared-secret", stubNotificationService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?userId=user-id", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRunHealthCheckSucceedsWhenHealthzIsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := runHealthCheck(context.Background(), server.URL, nil)

	assert.NoError(t, err)
}

func TestRunHealthCheckFailsWhenHealthzIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := runHealthCheck(context.Background(), server.URL, nil)

	assert.Error(t, err)
}

type tlsTestServer struct {
	ca   *tlstest.CA
	addr string
}

func startTLSServer(t *testing.T) tlsTestServer {
	t.Helper()

	return startTLSServerWithHandler(t, newHTTPServer("shared-secret", stubNotificationService{}))
}

func startTLSServerWithHandler(t *testing.T, handler http.Handler) tlsTestServer {
	t.Helper()

	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := newServer(handler, infra.NewServerTLSConfig(cert))
	done := make(chan error, 1)
	go func() { done <- serve(srv, ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		assert.ErrorIs(t, <-done, http.ErrServerClosed)
	})

	return tlsTestServer{ca: ca, addr: ln.Addr().String()}
}

func verifiedClient(tlsConfig *tls.Config) *http.Client {
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 5 * time.Second,
	}
}

func TestServeTLSServesHealthzToVerifiedClient(t *testing.T) {
	server := startTLSServer(t)
	client := verifiedClient(&tls.Config{ServerName: testServerName, RootCAs: server.ca.Pool()})

	res, err := client.Get("https://" + server.addr + "/healthz")
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusNoContent, res.StatusCode)
	require.NotNil(t, res.TLS)
	assert.GreaterOrEqual(t, res.TLS.Version, uint16(tls.VersionTLS12))
}

func TestServeTLSKeepsNotificationRoutesAuthenticated(t *testing.T) {
	server := startTLSServer(t)
	client := verifiedClient(&tls.Config{ServerName: testServerName, RootCAs: server.ca.Pool()})

	res, err := client.Get("https://" + server.addr + "/api/v1/notifications?userId=user-id")
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func TestServeTLSRejectsPlaintext(t *testing.T) {
	server := startTLSServer(t)
	client := verifiedClient(nil)

	res, err := client.Get("http://" + server.addr + "/healthz")
	require.NoError(t, err, "Go's TLS server answers plaintext HTTP with 400")
	defer res.Body.Close()

	assert.NotEqual(t, http.StatusNoContent, res.StatusCode)
	assert.False(t, res.StatusCode >= 300 && res.StatusCode < 400, "plaintext request must not be redirected, got %d", res.StatusCode)
	assert.Empty(t, res.Header.Get("Location"))
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestServeTLSAdvertisesMinimumTLS12(t *testing.T) {
	server := startTLSServer(t)

	conn, err := tls.Dial("tcp", server.addr, &tls.Config{
		ServerName: testServerName,
		RootCAs:    server.ca.Pool(),
		MinVersion: tls.VersionTLS10,
		MaxVersion: tls.VersionTLS11,
	})
	if conn != nil {
		_ = conn.Close()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol version")

	conn, err = tls.Dial("tcp", server.addr, &tls.Config{
		ServerName: testServerName,
		RootCAs:    server.ca.Pool(),
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS12,
	})
	require.NoError(t, err)
	_ = conn.Close()
}

// loopbackHealthCheck returns the production TLS healthcheck target pointed at
// the test listener, trusting only the test CA.
func loopbackHealthCheck(t *testing.T, server tlsTestServer, serverName string) (string, *tls.Config) {
	t.Helper()

	url, tlsConfig := healthCheckTarget(infra.ServerConfig{Mode: infra.TLSModeTLS, CertSecretARN: "arn:x", ServerName: serverName})
	require.NotNil(t, tlsConfig)
	tlsConfig.RootCAs = server.ca.Pool()
	return strings.Replace(url, "127.0.0.1:8080", server.addr, 1), tlsConfig
}

func TestRunHealthCheckTLSSucceedsOnLoopbackWithServerName(t *testing.T) {
	server := startTLSServer(t)
	url, tlsConfig := loopbackHealthCheck(t, server, testServerName)

	assert.NoError(t, runHealthCheck(context.Background(), url, tlsConfig))
}

func TestRunHealthCheckTLSFailsOnServerNameMismatch(t *testing.T) {
	server := startTLSServer(t)
	url, tlsConfig := loopbackHealthCheck(t, server, "other.example.test")

	err := runHealthCheck(context.Background(), url, tlsConfig)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestRunHealthCheckTLSFailsOnUntrustedRoot(t *testing.T) {
	server := startTLSServer(t)
	url, tlsConfig := loopbackHealthCheck(t, server, testServerName)
	tlsConfig.RootCAs = tlstest.NewCA(t).Pool()

	err := runHealthCheck(context.Background(), url, tlsConfig)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate")
}

func TestRunHealthCheckTLSDoesNotFollowRedirects(t *testing.T) {
	var plaintextHits atomic.Int32
	plaintext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plaintextHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer plaintext.Close()

	server := startTLSServerWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plaintext.URL+"/healthz", http.StatusFound)
	}))
	url, tlsConfig := loopbackHealthCheck(t, server, testServerName)

	err := runHealthCheck(context.Background(), url, tlsConfig)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 302")
	assert.Zero(t, plaintextHits.Load(), "a verified TLS healthcheck must not continue over plaintext")
}

func TestHealthCheckTargetUsesLoopbackAndServerName(t *testing.T) {
	url, tlsConfig := healthCheckTarget(infra.ServerConfig{Mode: infra.TLSModeTLS, CertSecretARN: "arn:x", ServerName: testServerName})

	assert.Equal(t, "https://127.0.0.1:8080/healthz", url)
	require.NotNil(t, tlsConfig)
	assert.Equal(t, testServerName, tlsConfig.ServerName)
	assert.Nil(t, tlsConfig.RootCAs)
	assert.False(t, tlsConfig.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)

	url, tlsConfig = healthCheckTarget(infra.ServerConfig{Mode: infra.TLSModeHTTP})

	assert.Equal(t, "http://127.0.0.1:8080/healthz", url)
	assert.Nil(t, tlsConfig)
}

func testLoadedCertificate(t *testing.T) infra.LoadedCertificate {
	t.Helper()

	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return infra.LoadedCertificate{
		Certificate:       cert,
		VersionID:         "version-1",
		FingerprintSHA256: "ab12",
		Serial:            "1f",
		NotAfter:          time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

func TestListenAndAnnounceLogsAfterBind(t *testing.T) {
	loaded := testLoadedCertificate(t)
	srv := newServer(http.NotFoundHandler(), infra.NewServerTLSConfig(loaded.Certificate))
	addr := freeAddr(t)
	var out bytes.Buffer

	ln, err := listenAndAnnounce(addr, srv, &loaded, func(context.Context) string {
		conn, err := net.Dial("tcp", addr)
		if assert.NoError(t, err, "listener must be bound before the task ARN is fetched") {
			_ = conn.Close()
		}
		assert.Empty(t, out.String())
		return "arn:aws:ecs:ap-northeast-1:123456789012:task/c/abc"
	}, &out)
	require.NoError(t, err)
	defer ln.Close()

	assert.Equal(t, addr, ln.Addr().String())
	assert.Equal(t, 1, strings.Count(out.String(), "\n"))
	var line map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &line))
	assert.Equal(t, "tls certificate loaded", line["msg"])
	assert.Equal(t, "version-1", line["secretVersionId"])
	assert.Equal(t, "ab12", line["certFingerprintSha256"])
	assert.Equal(t, "2026-12-01T00:00:00Z", line["certNotAfter"])
	assert.Equal(t, "1f", line["certSerial"])
	assert.Equal(t, "arn:aws:ecs:ap-northeast-1:123456789012:task/c/abc", line["taskArn"])
}

func TestListenAndAnnounceThenServeTLSPresentsLoggedCertificate(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	sum := sha256.Sum256(cert.Certificate[0])
	loaded := infra.LoadedCertificate{Certificate: cert, VersionID: "v", FingerprintSHA256: hex.EncodeToString(sum[:]), Serial: "1", NotAfter: time.Now()}

	srv := newServer(newHTTPServer("shared-secret", stubNotificationService{}), infra.NewServerTLSConfig(cert))
	var out bytes.Buffer
	ln, err := listenAndAnnounce("127.0.0.1:0", srv, &loaded, func(context.Context) string { return "arn" }, &out)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- serve(srv, ln) }()
	defer func() {
		_ = srv.Close()
		assert.ErrorIs(t, <-done, http.ErrServerClosed)
	}()

	var line map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &line))
	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{ServerName: testServerName, RootCAs: ca.Pool()})
	require.NoError(t, err)
	defer conn.Close()
	served := sha256.Sum256(conn.ConnectionState().PeerCertificates[0].Raw)
	assert.Equal(t, line["certFingerprintSha256"], hex.EncodeToString(served[:]))
	assert.Equal(t, uint16(tls.VersionTLS13), conn.ConnectionState().Version)
}

func TestListenAndAnnounceBindFailureEmitsNoLog(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupied.Close()

	loaded := testLoadedCertificate(t)
	srv := newServer(http.NotFoundHandler(), infra.NewServerTLSConfig(loaded.Certificate))
	var out bytes.Buffer
	fetched := false

	ln, err := listenAndAnnounce(occupied.Addr().String(), srv, &loaded, func(context.Context) string {
		fetched = true
		return "arn"
	}, &out)

	require.Error(t, err)
	assert.Nil(t, ln)
	assert.False(t, fetched)
	assert.Empty(t, out.String())
}

func TestListenAndAnnounceWithoutCertificateInTLSConfigEmitsNoLog(t *testing.T) {
	loaded := testLoadedCertificate(t)
	srv := newServer(http.NotFoundHandler(), &tls.Config{MinVersion: tls.VersionTLS12})
	addr := freeAddr(t)
	var out bytes.Buffer
	fetched := false

	ln, err := listenAndAnnounce(addr, srv, &loaded, func(context.Context) string {
		fetched = true
		return "arn"
	}, &out)

	require.Error(t, err)
	assert.Nil(t, ln)
	assert.False(t, fetched)
	assert.Empty(t, out.String())
	probe, err := net.Listen("tcp", addr)
	require.NoError(t, err, "no listener may remain after a failed TLS preparation")
	_ = probe.Close()
}

func TestListenAndAnnounceUsesFreshMetadataContext(t *testing.T) {
	loaded := testLoadedCertificate(t)
	srv := newServer(http.NotFoundHandler(), infra.NewServerTLSConfig(loaded.Certificate))
	var out bytes.Buffer

	// The certificate-loading context is already finished by the time the
	// listener is bound; the metadata lookup must not inherit it.
	certCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	cancel()
	<-certCtx.Done()

	ln, err := listenAndAnnounce("127.0.0.1:0", srv, &loaded, func(ctx context.Context) string {
		assert.NoError(t, ctx.Err())
		_, hasDeadline := ctx.Deadline()
		assert.False(t, hasDeadline)
		return "arn"
	}, &out)
	require.NoError(t, err)
	defer ln.Close()

	assert.Contains(t, out.String(), `"taskArn":"arn"`)
}

func TestListenAndAnnounceMetadataTimeoutLogsUnknown(t *testing.T) {
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer metadata.Close()
	getenv := func(key string) string {
		if key == "ECS_CONTAINER_METADATA_URI_V4" {
			return metadata.URL
		}
		return ""
	}

	loaded := testLoadedCertificate(t)
	srv := newServer(http.NotFoundHandler(), infra.NewServerTLSConfig(loaded.Certificate))
	var out bytes.Buffer

	start := time.Now()
	ln, err := listenAndAnnounce("127.0.0.1:0", srv, &loaded, func(ctx context.Context) string {
		return infra.FetchTaskARN(ctx, metadata.Client(), getenv)
	}, &out)
	elapsed := time.Since(start)
	require.NoError(t, err)
	defer ln.Close()

	assert.Less(t, elapsed, 2500*time.Millisecond)
	assert.Contains(t, out.String(), `"taskArn":"unknown"`)
}

func TestListenAndAnnounceHTTPModeEmitsNoLog(t *testing.T) {
	srv := newServer(http.NotFoundHandler(), nil)
	var out bytes.Buffer
	fetched := false

	ln, err := listenAndAnnounce("127.0.0.1:0", srv, nil, func(context.Context) string {
		fetched = true
		return "arn"
	}, &out)
	require.NoError(t, err)
	defer ln.Close()

	assert.False(t, fetched)
	assert.Empty(t, out.String())
}

func TestServeHTTPModeServesHealthz(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := newServer(newHTTPServer("shared-secret", stubNotificationService{}), nil)
	done := make(chan error, 1)
	go func() { done <- serve(srv, ln) }()
	defer func() {
		_ = srv.Close()
		assert.ErrorIs(t, <-done, http.ErrServerClosed)
	}()

	assert.NoError(t, runHealthCheck(context.Background(), "http://"+ln.Addr().String()+"/healthz", nil))
}
