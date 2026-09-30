package infra

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
)

// TLSMode selects how the server listens on its port.
type TLSMode string

const (
	TLSModeTLS  TLSMode = "tls"
	TLSModeHTTP TLSMode = "http"
)

const (
	envTLSMode          = "TLS_MODE"
	envTLSCertSecretARN = "TLS_CERT_SECRET_ARN"
	envTLSServerName    = "TLS_SERVER_NAME"

	certSecretVersionStage = "AWSCURRENT"
)

// ServerConfig is the non-secret listener configuration.
type ServerConfig struct {
	Mode          TLSMode
	CertSecretARN string
	ServerName    string
}

// LoadServerConfig reads the listener configuration. An unset TLS_MODE means
// tls, which requires TLS_CERT_SECRET_ARN and TLS_SERVER_NAME and never falls
// back to plain HTTP.
func LoadServerConfig(getenv func(string) string) (ServerConfig, error) {
	switch mode := getenv(envTLSMode); mode {
	case "", string(TLSModeTLS):
	case string(TLSModeHTTP):
		return ServerConfig{Mode: TLSModeHTTP}, nil
	default:
		return ServerConfig{}, fmt.Errorf("unknown TLS_MODE %q (expected %q or %q)", mode, TLSModeTLS, TLSModeHTTP)
	}

	cfg := ServerConfig{
		Mode:          TLSModeTLS,
		CertSecretARN: getenv(envTLSCertSecretARN),
		ServerName:    getenv(envTLSServerName),
	}

	var missing []string
	if cfg.CertSecretARN == "" {
		missing = append(missing, envTLSCertSecretARN)
	}
	if cfg.ServerName == "" {
		missing = append(missing, envTLSServerName)
	}
	if len(missing) > 0 {
		return ServerConfig{}, fmt.Errorf("TLS_MODE=tls requires %s", strings.Join(missing, " and "))
	}

	return cfg, nil
}

// SecretAPI is the subset of the Secrets Manager client used to load the
// server certificate.
type SecretAPI interface {
	GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// NewSecretsManagerClient builds a client from the default AWS config chain.
func NewSecretsManagerClient(ctx context.Context) (SecretAPI, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return secretsmanager.NewFromConfig(cfg), nil
}

// LoadedCertificate is a verified server certificate and its non-secret
// metadata.
type LoadedCertificate struct {
	Certificate       tls.Certificate
	VersionID         string
	FingerprintSHA256 string
	Serial            string
	NotAfter          time.Time
}

type certificateSecretValue struct {
	CertificatePem string `json:"certificatePem"`
	PrivateKeyPem  string `json:"privateKeyPem"`
}

// LoadServerCertificate reads the AWSCURRENT certificate secret and verifies
// the key pair, SAN, validity period and chain. A nil roots pool means the
// system roots. Errors describe only the kind of failure and never include
// the secret contents.
func LoadServerCertificate(ctx context.Context, api SecretAPI, secretARN, serverName string, roots *x509.CertPool, now time.Time) (LoadedCertificate, error) {
	out, err := api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretARN),
		VersionStage: aws.String(certSecretVersionStage),
	})
	if err != nil {
		// A response that failed to decode keeps part of the body (the secret)
		// in DeserializationError.Snapshot, so drop it from the chain.
		var deserialization *smithy.DeserializationError
		if errors.As(err, &deserialization) {
			return LoadedCertificate{}, errors.New("get TLS certificate secret: response could not be decoded")
		}
		return LoadedCertificate{}, fmt.Errorf("get TLS certificate secret: %w", err)
	}
	if out == nil {
		return LoadedCertificate{}, errors.New("TLS certificate secret response is empty")
	}
	versionID := aws.ToString(out.VersionId)
	if versionID == "" {
		return LoadedCertificate{}, errors.New("TLS certificate secret has no version id")
	}
	if aws.ToString(out.SecretString) == "" {
		return LoadedCertificate{}, errors.New("TLS certificate secret string is empty")
	}

	var secret certificateSecretValue
	if err := json.Unmarshal([]byte(*out.SecretString), &secret); err != nil {
		return LoadedCertificate{}, errors.New("TLS certificate secret is not a valid JSON object")
	}
	if secret.CertificatePem == "" {
		return LoadedCertificate{}, errors.New("TLS certificate secret has no certificatePem")
	}
	if secret.PrivateKeyPem == "" {
		return LoadedCertificate{}, errors.New("TLS certificate secret has no privateKeyPem")
	}

	cert, err := tls.X509KeyPair([]byte(secret.CertificatePem), []byte(secret.PrivateKeyPem))
	if err != nil {
		return LoadedCertificate{}, errors.New("TLS certificate secret does not contain a matching certificate and private key pair")
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return LoadedCertificate{}, errors.New("TLS certificate secret has an unparsable leaf certificate")
	}
	cert.Leaf = leaf

	intermediates := x509.NewCertPool()
	for _, der := range cert.Certificate[1:] {
		intermediate, err := x509.ParseCertificate(der)
		if err != nil {
			return LoadedCertificate{}, errors.New("TLS certificate secret has an unparsable chain certificate")
		}
		intermediates.AddCert(intermediate)
	}

	if _, err := leaf.Verify(x509.VerifyOptions{
		DNSName:       serverName,
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		return LoadedCertificate{}, fmt.Errorf("verify TLS certificate: %w", err)
	}

	fingerprint := sha256.Sum256(leaf.Raw)
	return LoadedCertificate{
		Certificate:       cert,
		VersionID:         versionID,
		FingerprintSHA256: hex.EncodeToString(fingerprint[:]),
		Serial:            leaf.SerialNumber.Text(16),
		NotAfter:          leaf.NotAfter,
	}, nil
}

// NewServerTLSConfig serves the in-memory certificate with TLS 1.2 or later.
func NewServerTLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}

// LogCertificateLoaded writes one JSON line with the non-secret certificate
// metadata that the issuer uses to confirm a rollout.
func LogCertificateLoaded(w io.Writer, c LoadedCertificate, taskARN string) error {
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "tls certificate loaded", 0)
	record.AddAttrs(
		slog.String("secretVersionId", c.VersionID),
		slog.String("certFingerprintSha256", c.FingerprintSHA256),
		slog.String("certNotAfter", c.NotAfter.UTC().Format(time.RFC3339)),
		slog.String("certSerial", c.Serial),
		slog.String("taskArn", taskARN),
	)

	return slog.NewJSONHandler(w, nil).Handle(context.Background(), record)
}
