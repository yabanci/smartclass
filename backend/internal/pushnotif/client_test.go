package pushnotif_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"smartclass/internal/pushnotif"
)

func TestClient_Noop_WhenNotConfigured(t *testing.T) {
	c := pushnotif.NewClient(pushnotif.PushConfig{}, zap.NewNop())
	err := c.Send(context.Background(), "any-token", pushnotif.Payload{Title: "Hello", Body: "World"})
	assert.NoError(t, err, "no-op client must never return an error")
}

func TestClient_Noop_WhenOnlyProjectIDSet(t *testing.T) {
	c := pushnotif.NewClient(pushnotif.PushConfig{ProjectID: "proj"}, zap.NewNop())
	err := c.Send(context.Background(), "any-token", pushnotif.Payload{Title: "t", Body: "b"})
	assert.NoError(t, err, "no credentials → no-op, no error")
}

// fakeServiceAccountJSON produces a minimal service-account JSON with a real
// RSA key so google.CredentialsFromJSON can parse it. The token_uri is pointed
// at a local httptest server so no network call goes to Google.
func fakeServiceAccountJSON(t *testing.T, tokenURL string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	sa := map[string]any{
		"type":                        "service_account",
		"project_id":                  "test-project",
		"private_key_id":              "key1",
		"private_key":                 buf.String(),
		"client_email":                "test@test-project.iam.gserviceaccount.com",
		"client_id":                   "123",
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   tokenURL,
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/test",
	}
	b, err := json.Marshal(sa)
	require.NoError(t, err)
	return b
}

func newFakeTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fake-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
}

func TestClient_ErrInvalidToken_On404(t *testing.T) {
	tokenSrv := newFakeTokenServer(t)
	defer tokenSrv.Close()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","message":"Requested entity was not found."}}`))
	}))
	defer fcmSrv.Close()

	saJSON := fakeServiceAccountJSON(t, tokenSrv.URL)
	cfg := pushnotif.PushConfig{ProjectID: "test-project", ServiceAccountJSON: saJSON}
	c := pushnotif.NewClientWithHTTP(cfg, zap.NewNop(), fcmSrv.Client(),
		fcmSrv.URL+"/v1/projects/%s/messages:send")

	err := c.Send(context.Background(), "bad-device-token", pushnotif.Payload{Title: "t", Body: "b"})
	assert.ErrorIs(t, err, pushnotif.ErrInvalidToken,
		"FCM 404 must be mapped to ErrInvalidToken so callers can clean up stale tokens")
}

func TestClient_GenericError_OnFCM500(t *testing.T) {
	tokenSrv := newFakeTokenServer(t)
	defer tokenSrv.Close()

	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer fcmSrv.Close()

	saJSON := fakeServiceAccountJSON(t, tokenSrv.URL)
	cfg := pushnotif.PushConfig{ProjectID: "proj", ServiceAccountJSON: saJSON}
	c := pushnotif.NewClientWithHTTP(cfg, zap.NewNop(), fcmSrv.Client(),
		fcmSrv.URL+"/v1/projects/%s/messages:send")

	err := c.Send(context.Background(), "some-token", pushnotif.Payload{Title: "t", Body: "b"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, pushnotif.ErrInvalidToken,
		"FCM 500 must NOT be mapped to ErrInvalidToken — it is a transient server error, not a stale token")
}
