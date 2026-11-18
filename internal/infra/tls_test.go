package infra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
	"github.com/mi-24v/miwkey-extension/internal/tlstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecretARN  = "arn:aws:secretsmanager:ap-northeast-1:123456789012:secret:ext-tls"
	testServerName = "ext.example.test"
	testVersionID  = "11111111-2222-3333-4444-555555555555"
)

type fakeSecretAPI struct {
	inputs []*secretsmanager.GetSecretValueInput
	out    *secretsmanager.GetSecretValueOutput
	err    error
}

func (f *fakeSecretAPI) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.inputs = append(f.inputs, in)
	return f.out, f.err
}

func certificateSecret(t *testing.T, certPEM, keyPEM []byte) *fakeSecretAPI {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"certificatePem": string(certPEM),
		"privateKeyPem":  string(keyPEM),
	})
	require.NoError(t, err)
	return &fakeSecretAPI{out: &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(string(body)),
		VersionId:    aws.String(testVersionID),
	}}
}

func envMap(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    ServerConfig
		wantErr []string
	}{
		{
			name:    "defaults to tls and requires both settings",
			env:     map[string]string{},
			wantErr: []string{"TLS_CERT_SECRET_ARN", "TLS_SERVER_NAME"},
		},
		{
			name:    "server name without secret ARN",
			env:     map[string]string{"TLS_SERVER_NAME": testServerName},
			wantErr: []string{"TLS_CERT_SECRET_ARN"},
		},
		{
			name:    "secret ARN without server name",
			env:     map[string]string{"TLS_CERT_SECRET_ARN": testSecretARN},
			wantErr: []string{"TLS_SERVER_NAME"},
		},
		{
			name:    "explicit tls requires settings",
			env:     map[string]string{"TLS_MODE": "tls"},
			wantErr: []string{"TLS_CERT_SECRET_ARN", "TLS_SERVER_NAME"},
		},
		{
			name: "explicit http for local development",
			env:  map[string]string{"TLS_MODE": "http"},
			want: ServerConfig{Mode: TLSModeHTTP},
		},
		{
			name:    "HTTPS is not a mode",
			env:     map[string]string{"TLS_MODE": "HTTPS", "TLS_CERT_SECRET_ARN": testSecretARN, "TLS_SERVER_NAME": testServerName},
			wantErr: []string{"unknown TLS_MODE"},
		},
		{
			name:    "plain is not a mode",
			env:     map[string]string{"TLS_MODE": "plain"},
			wantErr: []string{"unknown TLS_MODE"},
		},
		{
			name: "tls with both settings",
			env:  map[string]string{"TLS_CERT_SECRET_ARN": testSecretARN, "TLS_SERVER_NAME": testServerName},
			want: ServerConfig{Mode: TLSModeTLS, CertSecretARN: testSecretARN, ServerName: testServerName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadServerConfig(envMap(tt.env))
			if len(tt.wantErr) > 0 {
				require.Error(t, err)
				for _, fragment := range tt.wantErr {
					assert.Contains(t, err.Error(), fragment)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadServerCertificateRequestsAWSCURRENT(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	api := certificateSecret(t, certPEM, keyPEM)

	_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

	require.NoError(t, err)
	require.Len(t, api.inputs, 1)
	assert.Equal(t, testSecretARN, aws.ToString(api.inputs[0].SecretId))
	require.NotNil(t, api.inputs[0].VersionStage)
	assert.Equal(t, "AWSCURRENT", *api.inputs[0].VersionStage)
	assert.Nil(t, api.inputs[0].VersionId)
}

func TestLoadServerCertificateReturnsMetadata(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	api := certificateSecret(t, certPEM, keyPEM)

	loaded, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())
	require.NoError(t, err)

	block, _ := pem.Decode(certPEM)
	leaf, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	sum := sha256.Sum256(leaf.Raw)

	assert.Equal(t, testVersionID, loaded.VersionID)
	assert.Equal(t, hex.EncodeToString(sum[:]), loaded.FingerprintSHA256)
	assert.Equal(t, leaf.SerialNumber.Text(16), loaded.Serial)
	assert.True(t, leaf.NotAfter.Equal(loaded.NotAfter))
	require.Len(t, loaded.Certificate.Certificate, 1)
	assert.Equal(t, leaf.Raw, loaded.Certificate.Certificate[0])
	assert.NotNil(t, loaded.Certificate.PrivateKey)
}

func TestLoadServerCertificateAcceptsFullchainWithIntermediate(t *testing.T) {
	ca := tlstest.NewCA(t)
	leafPEM, intermediatePEM, keyPEM := ca.IssueViaIntermediate(t, []string{testServerName})
	api := certificateSecret(t, append(append([]byte{}, leafPEM...), intermediatePEM...), keyPEM)

	loaded, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

	require.NoError(t, err)
	assert.Len(t, loaded.Certificate.Certificate, 2)
}

func TestLoadServerCertificateRejectsMissingIntermediate(t *testing.T) {
	ca := tlstest.NewCA(t)
	leafPEM, _, keyPEM := ca.IssueViaIntermediate(t, []string{testServerName})
	api := certificateSecret(t, leafPEM, keyPEM)

	_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

	require.Error(t, err)
	var unknownAuthority x509.UnknownAuthorityError
	assert.ErrorAs(t, err, &unknownAuthority)
}

func TestLoadServerCertificateRejectsKeyMismatch(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, _ := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	_, otherKeyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	api := certificateSecret(t, certPEM, otherKeyPEM)

	_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "key pair")
}

func TestLoadServerCertificateRejectsSANMismatch(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{"other.example.test"}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	api := certificateSecret(t, certPEM, keyPEM)

	_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

	require.Error(t, err)
	var hostnameErr x509.HostnameError
	assert.ErrorAs(t, err, &hostnameErr)
}

func TestLoadServerCertificateRejectsUntrustedCA(t *testing.T) {
	ca := tlstest.NewCA(t)
	otherCA := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	api := certificateSecret(t, certPEM, keyPEM)

	_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, otherCA.Pool(), time.Now())

	require.Error(t, err)
	var unknownAuthority x509.UnknownAuthorityError
	assert.ErrorAs(t, err, &unknownAuthority)
}

func TestLoadServerCertificateRejectsExpiredAndNotYetValid(t *testing.T) {
	ca := tlstest.NewCA(t)
	notBefore := time.Now().Add(-time.Hour)
	notAfter := time.Now().Add(time.Hour)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, notBefore, notAfter)

	for name, now := range map[string]time.Time{
		"expired":       notAfter.Add(time.Minute),
		"not yet valid": notBefore.Add(-time.Minute),
	} {
		t.Run(name, func(t *testing.T) {
			api := certificateSecret(t, certPEM, keyPEM)

			_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), now)

			require.Error(t, err)
			var invalid x509.CertificateInvalidError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, x509.Expired, invalid.Reason)
		})
	}
}

// malformedSecretCases returns secret responses that must never start the
// server. The valid PEM pair is included in several of them so the error
// text can be checked for leaks.
func malformedSecretCases(t *testing.T) map[string]*fakeSecretAPI {
	t.Helper()

	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	withBody := func(body *string, versionID *string) *fakeSecretAPI {
		return &fakeSecretAPI{out: &secretsmanager.GetSecretValueOutput{SecretString: body, VersionId: versionID}}
	}
	jsonBody := func(fields map[string]string) *string {
		body, err := json.Marshal(fields)
		require.NoError(t, err)
		return aws.String(string(body))
	}
	valid := jsonBody(map[string]string{"certificatePem": string(certPEM), "privateKeyPem": string(keyPEM)})

	return map[string]*fakeSecretAPI{
		"api error":              {err: errors.New("AccessDeniedException: not authorized")},
		"nil output":             {},
		"secret string nil":      withBody(nil, aws.String(testVersionID)),
		"secret string empty":    withBody(aws.String(""), aws.String(testVersionID)),
		"not JSON":               withBody(aws.String(string(certPEM)+string(keyPEM)), aws.String(testVersionID)),
		"JSON with wrong types":  withBody(aws.String(`{"certificatePem":1,"privateKeyPem":2}`), aws.String(testVersionID)),
		"certificatePem empty":   withBody(jsonBody(map[string]string{"certificatePem": "", "privateKeyPem": string(keyPEM)}), aws.String(testVersionID)),
		"certificatePem missing": withBody(jsonBody(map[string]string{"privateKeyPem": string(keyPEM)}), aws.String(testVersionID)),
		"privateKeyPem empty":    withBody(jsonBody(map[string]string{"certificatePem": string(certPEM), "privateKeyPem": ""}), aws.String(testVersionID)),
		"privateKeyPem missing":  withBody(jsonBody(map[string]string{"certificatePem": string(certPEM)}), aws.String(testVersionID)),
		"certificatePem not PEM": withBody(jsonBody(map[string]string{"certificatePem": "not a certificate", "privateKeyPem": string(keyPEM)}), aws.String(testVersionID)),
		"fields swapped":         withBody(jsonBody(map[string]string{"certificatePem": string(keyPEM), "privateKeyPem": string(certPEM)}), aws.String(testVersionID)),
		"version id missing":     withBody(valid, nil),
		"version id empty":       withBody(valid, aws.String("")),
	}
}

func TestLoadServerCertificateRejectsMalformedSecret(t *testing.T) {
	ca := tlstest.NewCA(t)

	for name, api := range malformedSecretCases(t) {
		t.Run(name, func(t *testing.T) {
			_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

			assert.Error(t, err)
		})
	}
}

func TestLoadServerCertificateErrorsNeverContainPEM(t *testing.T) {
	ca := tlstest.NewCA(t)
	cases := malformedSecretCases(t)

	certPEM, _ := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	_, otherKeyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	cases["key mismatch"] = certificateSecret(t, certPEM, otherKeyPEM)

	sanPEM, sanKeyPEM := ca.Issue(t, []string{"other.example.test"}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	cases["SAN mismatch"] = certificateSecret(t, sanPEM, sanKeyPEM)

	leafPEM, _, leafKeyPEM := ca.IssueViaIntermediate(t, []string{testServerName})
	cases["missing intermediate"] = certificateSecret(t, leafPEM, leafKeyPEM)

	for name, api := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

			require.Error(t, err)
			msg := err.Error()
			assert.NotContains(t, msg, "BEGIN")
			assert.NotContains(t, msg, "PRIVATE KEY")
			assert.NotContains(t, msg, "CERTIFICATE")
			if api.out != nil && api.out.SecretString != nil && len(*api.out.SecretString) > 64 {
				assert.NotContains(t, msg, (*api.out.SecretString)[32:64])
			}
		})
	}
}

func TestLoadServerCertificateDropsDeserializationSnapshot(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	// A 200 response whose body fails to decode keeps up to 1 KiB of the body
	// (the secret) in DeserializationError.Snapshot.
	api := &fakeSecretAPI{err: &smithy.OperationError{
		ServiceID:     "Secrets Manager",
		OperationName: "GetSecretValue",
		Err: &smithy.DeserializationError{
			Err:      errors.New("failed to decode response body, unexpected EOF"),
			Snapshot: append(append([]byte(`{"SecretString":"`), certPEM...), keyPEM...),
		},
	}}

	_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

	require.Error(t, err)
	var deserialization *smithy.DeserializationError
	assert.False(t, errors.As(err, &deserialization), "the error chain must not carry the response snapshot")
	assert.NotContains(t, err.Error(), "BEGIN")
	assert.Contains(t, err.Error(), "get TLS certificate secret")
}

func TestLoadServerCertificateKeepsAPIErrorForDiagnosis(t *testing.T) {
	ca := tlstest.NewCA(t)
	apiErr := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized"}
	api := &fakeSecretAPI{err: &smithy.OperationError{ServiceID: "Secrets Manager", OperationName: "GetSecretValue", Err: apiErr}}

	_, err := LoadServerCertificate(context.Background(), api, testSecretARN, testServerName, ca.Pool(), time.Now())

	require.Error(t, err)
	var got smithy.APIError
	require.ErrorAs(t, err, &got)
	assert.Equal(t, "AccessDeniedException", got.ErrorCode())
}

func TestNewServerTLSConfigRequiresTLS12(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	cfg := NewServerTLSConfig(cert)

	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Len(t, cfg.Certificates, 1)
	assert.Equal(t, cert.Certificate, cfg.Certificates[0].Certificate)
	assert.False(t, cfg.InsecureSkipVerify)
}

func TestLogCertificateLoadedWritesOneNonSecretJSONLine(t *testing.T) {
	ca := tlstest.NewCA(t)
	certPEM, keyPEM := ca.Issue(t, []string{testServerName}, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	loaded, err := LoadServerCertificate(context.Background(), certificateSecret(t, certPEM, keyPEM), testSecretARN, testServerName, ca.Pool(), time.Now())
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, LogCertificateLoaded(&buf, loaded, "arn:aws:ecs:ap-northeast-1:123456789012:task/c/abc"))

	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, "\n"))
	assert.True(t, strings.HasSuffix(out, "\n"))
	assert.NotContains(t, out, "BEGIN")
	assert.NotContains(t, out, "PRIVATE KEY")

	var line map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &line))
	assert.Equal(t, "tls certificate loaded", line["msg"])
	assert.Equal(t, "INFO", line["level"])
	assert.Contains(t, line, "time")
	assert.Equal(t, testVersionID, line["secretVersionId"])
	assert.Equal(t, loaded.FingerprintSHA256, line["certFingerprintSha256"])
	assert.Equal(t, loaded.NotAfter.UTC().Format(time.RFC3339), line["certNotAfter"])
	assert.Equal(t, loaded.Serial, line["certSerial"])
	assert.Equal(t, "arn:aws:ecs:ap-northeast-1:123456789012:task/c/abc", line["taskArn"])
	assert.Len(t, line, 8)
}
