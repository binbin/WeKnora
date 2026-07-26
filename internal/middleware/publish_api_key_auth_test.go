package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type fakeAgentPublishAPIKeyService struct {
	key *types.AgentPublishAPIKey
	err error
}

func (f *fakeAgentPublishAPIKeyService) Create(
	ctx context.Context, req interfaces.AgentPublishAPIKeyCreateRequest,
) (*interfaces.AgentPublishAPIKeyCreateResult, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeAgentPublishAPIKeyService) Authenticate(
	ctx context.Context, token string,
) (*types.AgentPublishAPIKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.key, nil
}

func (f *fakeAgentPublishAPIKeyService) ListByAgent(
	ctx context.Context, tenantID uint64, agentID string,
) ([]*types.AgentPublishAPIKey, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeAgentPublishAPIKeyService) Revoke(
	ctx context.Context, tenantID uint64, agentID string, keyID uint64,
) error {
	return errors.New("not implemented")
}

var _ interfaces.AgentPublishAPIKeyService = (*fakeAgentPublishAPIKeyService)(nil)

func TestIsNoAuthAPIChatCompletions(t *testing.T) {
	if !isNoAuthAPI("/api/v1/chat/completions", http.MethodPost) {
		t.Fatal("POST /api/v1/chat/completions must be in noAuthAPI")
	}
	if isNoAuthAPI("/api/v1/chat/completions", http.MethodGet) {
		t.Fatal("GET /api/v1/chat/completions must not be noAuth")
	}
}

func TestPublishAPIKeyAuthMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAgentPublishAPIKeyService{}
	tenantSvc := &fakeTenantService{tenant: &types.Tenant{ID: 7}}

	r := gin.New()
	r.POST("/api/v1/chat/completions", PublishAPIKeyAuth(svc, tenantSvc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	assertOpenAPIError(t, w.Body.Bytes(), "unauthorized", "missing bearer token")
}

func TestPublishAPIKeyAuthInvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeAgentPublishAPIKeyService{
		err: errors.New("not found"),
	}
	tenantSvc := &fakeTenantService{tenant: &types.Tenant{ID: 7}}

	r := gin.New()
	r.POST("/api/v1/chat/completions", PublishAPIKeyAuth(svc, tenantSvc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer wkpub_bad")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	assertOpenAPIError(t, w.Body.Bytes(), "unauthorized", "invalid api key")
}

func TestPublishAPIKeyAuthValidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	svc := &fakeAgentPublishAPIKeyService{
		key: &types.AgentPublishAPIKey{
			ID:        99,
			TenantID:  7,
			AgentID:   "agent-abc",
			Name:      "test",
			KeyPrefix: "wkpub_xxxx",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	tenantSvc := &fakeTenantService{tenant: &types.Tenant{ID: 7, Name: "demo"}}

	var nextCalled bool
	r := gin.New()
	r.POST("/api/v1/chat/completions", PublishAPIKeyAuth(svc, tenantSvc), func(c *gin.Context) {
		nextCalled = true

		pub, ok := types.AgentPublishAPIKeyContextFromContext(c.Request.Context())
		if !ok || pub.KeyID != 99 || pub.TenantID != 7 || pub.AgentID != "agent-abc" {
			t.Errorf("publish ctx = %#v, ok=%v", pub, ok)
		}
		principal, ok := types.PrincipalFromContext(c.Request.Context())
		if !ok ||
			principal.Type != types.PrincipalAPIPublish ||
			principal.ID != "99" {
			t.Errorf("principal = %#v, ok=%v", principal, ok)
		}
		tenantID, ok := types.TenantIDFromContext(c.Request.Context())
		if !ok || tenantID != 7 {
			t.Errorf("tenant id = %d, ok=%v", tenantID, ok)
		}
		if _, ok := types.TenantAPIKeyScopeFromContext(c.Request.Context()); ok {
			t.Error("TenantAPIKeyScope must not be set for publish keys")
		}
		owner := types.SessionOwnerIDFromContext(c.Request.Context())
		wantOwner := "api_publish_key:7:99"
		if owner != wantOwner {
			t.Errorf("session owner = %q, want %q", owner, wantOwner)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer wkpub_valid_token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !nextCalled {
		t.Fatal("expected Next handler to run")
	}
}

func assertOpenAPIError(t *testing.T, body []byte, code, message string) {
	t.Helper()
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal error body: %v; body=%s", err, string(body))
	}
	if payload.Error.Type != "invalid_request_error" &&
		payload.Error.Type != "authentication_error" {
		// Accept either OpenAI-style type; code is the stable signal.
		if payload.Error.Type == "" {
			t.Fatalf("missing error.type; body=%s", string(body))
		}
	}
	if payload.Error.Code != code {
		t.Fatalf("error.code = %q, want %q; body=%s", payload.Error.Code, code, string(body))
	}
	if payload.Error.Message != message {
		t.Fatalf(
			"error.message = %q, want %q; body=%s",
			payload.Error.Message, message, string(body),
		)
	}
}
