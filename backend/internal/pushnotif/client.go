package pushnotif

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ErrInvalidToken is returned when FCM reports UNREGISTERED or INVALID_ARGUMENT
// for the device token. The caller should remove the token from the database.
var ErrInvalidToken = errors.New("pushnotif: invalid or unregistered FCM token")

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// Payload is the FCM HTTP v1 message content.
type Payload struct {
	Title string
	Body  string
	Data  map[string]string
}

// PushConfig holds project-level FCM configuration resolved from environment.
type PushConfig struct {
	ProjectID          string
	ServiceAccountJSON []byte // raw JSON credentials; nil = no-op mode
}

// ConfigFromEnv resolves PushConfig from environment variables.
// Priority: FIREBASE_SERVICE_ACCOUNT_JSON > FIREBASE_SERVICE_ACCOUNT_PATH.
// If FIREBASE_PROJECT_ID is empty or credentials are absent, returns a zero
// config which causes Client to operate in no-op mode.
// log may be nil (a nop logger is used in that case).
func ConfigFromEnv(log *zap.Logger) PushConfig {
	if log == nil {
		log = zap.NewNop()
	}
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		return PushConfig{}
	}
	raw := []byte(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"))
	if len(raw) == 0 {
		if path := os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH"); path != "" {
			var err error
			// #nosec G304,G703 — path is operator-controlled via env var, used once at startup.
			raw, err = os.ReadFile(path)
			if err != nil {
				log.Warn("pushnotif: FIREBASE_SERVICE_ACCOUNT_PATH is set but unreadable; FCM will run in no-op mode",
					zap.String("path", path),
					zap.Error(err))
				raw = nil
			}
		}
	}
	return PushConfig{ProjectID: projectID, ServiceAccountJSON: raw}
}

// Client sends FCM HTTP v1 messages. If ServiceAccountJSON is empty it becomes
// a no-op that logs at INFO level, so the service starts with zero configuration.
type Client struct {
	cfg            PushConfig
	log            *zap.Logger
	httpClient     *http.Client
	fcmURLTemplate string // empty = use the default FCM endpoint

	mu          sync.Mutex
	tokenSource oauth2.TokenSource
}

// NewClient constructs a Client. cfg is typically obtained from ConfigFromEnv.
func NewClient(cfg PushConfig, log *zap.Logger) *Client {
	if log == nil {
		log = zap.NewNop()
	}
	return &Client{
		cfg:        cfg,
		log:        log,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// NewClientWithHTTP is used in tests to inject a custom HTTP client and FCM URL template.
// The urlTemplate must contain a single %s placeholder for the project ID.
func NewClientWithHTTP(cfg PushConfig, log *zap.Logger, httpClient *http.Client, urlTemplate string) *Client {
	c := NewClient(cfg, log)
	c.httpClient = httpClient
	c.fcmURLTemplate = urlTemplate
	return c
}

func (c *Client) isNoop() bool {
	return c.cfg.ProjectID == "" || len(c.cfg.ServiceAccountJSON) == 0
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokenSource == nil {
		cfg, err := google.JWTConfigFromJSON(c.cfg.ServiceAccountJSON, fcmScope)
		if err != nil {
			return "", fmt.Errorf("pushnotif: parse service account: %w", err)
		}
		c.tokenSource = cfg.TokenSource(ctx)
	}
	tok, err := c.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("pushnotif: oauth2 token: %w", err)
	}
	return tok.AccessToken, nil
}

// Send dispatches a push notification to a single FCM device token.
// Returns ErrInvalidToken when FCM reports the token is unregistered or invalid.
func (c *Client) Send(ctx context.Context, deviceToken string, p Payload) error {
	if c.isNoop() {
		c.log.Info("pushnotif: no-op (FCM not configured)",
			zap.String("token_prefix", safePrefix(deviceToken)),
			zap.String("title", p.Title))
		return nil
	}

	accessToken, err := c.accessToken(ctx)
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"token": deviceToken,
			"notification": map[string]string{
				"title": p.Title,
				"body":  p.Body,
			},
			"data": p.Data,
		},
	})
	if err != nil {
		return fmt.Errorf("pushnotif: marshal: %w", err)
	}

	tmpl := c.fcmURLTemplate
	if tmpl == "" {
		tmpl = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	}
	urlStr := fmt.Sprintf(tmpl, c.cfg.ProjectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pushnotif: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushnotif: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	// FCM returns 404 for UNREGISTERED tokens, 400 with INVALID_ARGUMENT for malformed ones.
	if resp.StatusCode == http.StatusNotFound ||
		(resp.StatusCode == http.StatusBadRequest &&
			strings.Contains(respStr, "INVALID_ARGUMENT")) {
		return ErrInvalidToken
	}

	return fmt.Errorf("pushnotif: FCM %d: %s", resp.StatusCode, respStr)
}

func safePrefix(token string) string {
	if len(token) > 8 {
		return token[:8] + "..."
	}
	return token
}
