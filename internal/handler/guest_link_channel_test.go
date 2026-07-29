package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// bootstrapGuestSvc is a minimal fake of interfaces.GuestLinkChannelService
// for BootstrapWebLink tests; embedding the nil interface lets it satisfy
// the contract while only overriding LookupByWebSlug.
type bootstrapGuestSvc struct {
	interfaces.GuestLinkChannelService
	byWebSlug map[string]*types.GuestLinkChannel
}

func (f *bootstrapGuestSvc) LookupByWebSlug(_ context.Context, slug string) (*types.GuestLinkChannel, error) {
	gl, ok := f.byWebSlug[slug]
	if !ok {
		return nil, service.ErrGuestLinkNotFound
	}
	if !gl.Enabled {
		return nil, service.ErrGuestLinkDisabled
	}
	return gl, nil
}

// bootstrapEmbedSvc is a minimal fake of interfaces.EmbedChannelService for
// BootstrapWebLink tests; overrides only IssueSessionToken and PublicConfig.
type bootstrapEmbedSvc struct {
	interfaces.EmbedChannelService
	sessionToken string
	expiresIn    int
	issueErr     error
}

func (f *bootstrapEmbedSvc) IssueSessionToken(context.Context, string) (string, int, error) {
	if f.issueErr != nil {
		return "", 0, f.issueErr
	}
	return f.sessionToken, f.expiresIn, nil
}

func (f *bootstrapEmbedSvc) PublicConfig(_ context.Context, ch *types.EmbedChannel) types.EmbedChannelPublicConfig {
	return types.EmbedChannelPublicConfig{ChannelID: ch.ID, AgentID: ch.AgentID}
}

func newBootstrapHandler(gl *types.GuestLinkChannel, embedSvc *bootstrapEmbedSvc) *GuestLinkChannelHandler {
	guestSvc := &bootstrapGuestSvc{byWebSlug: map[string]*types.GuestLinkChannel{}}
	if gl != nil {
		guestSvc.byWebSlug[gl.WebSlug] = gl
	}
	return NewGuestLinkChannelHandler(guestSvc, embedSvc)
}

func newBootstrapRequest(slug, origin, host string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/embed/web/"+slug+"/bootstrap", nil)
	req.Host = host
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestBootstrapWebLinkSameHostSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gl := &types.GuestLinkChannel{
		ID: "guest-1", TenantID: 7, AgentID: "agent-1", WebSlug: "abc123", Enabled: true,
	}
	h := newBootstrapHandler(gl, &bootstrapEmbedSvc{sessionToken: "ems_guest_token", expiresIn: 1800})

	r := gin.New()
	r.POST("/api/v1/embed/web/:slug/bootstrap", h.BootstrapWebLink)

	// middleware.HostOrigin defaults to the http:// scheme (see HostOrigin),
	// so the same-host Origin here must match that scheme.
	req := newBootstrapRequest("abc123", "http://app.example.com", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ChannelID    string `json:"channel_id"`
			SessionToken string `json:"session_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data.ChannelID != gl.ID || resp.Data.SessionToken != "ems_guest_token" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestBootstrapWebLinkAllowsSchemeMismatchSameHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gl := &types.GuestLinkChannel{
		ID: "guest-1", TenantID: 7, AgentID: "agent-1", WebSlug: "abc123", Enabled: true,
	}
	h := newBootstrapHandler(gl, &bootstrapEmbedSvc{sessionToken: "ems_guest_token", expiresIn: 1800})

	r := gin.New()
	r.POST("/api/v1/embed/web/:slug/bootstrap", h.BootstrapWebLink)

	// Nested TLS proxy: browser Origin is https, but the inner hop reports
	// http via Host / missing X-Forwarded-Proto.
	req := newBootstrapRequest("abc123", "https://app.example.com", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestBootstrapWebLinkRejectsCrossOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gl := &types.GuestLinkChannel{
		ID: "guest-1", TenantID: 7, AgentID: "agent-1", WebSlug: "abc123", Enabled: true,
	}
	h := newBootstrapHandler(gl, &bootstrapEmbedSvc{sessionToken: "ems_guest_token", expiresIn: 1800})

	r := gin.New()
	r.POST("/api/v1/embed/web/:slug/bootstrap", h.BootstrapWebLink)

	// Cross-origin: guest links have no allowed_origins, so this must be
	// rejected even though an embed channel with the same origin might allow it.
	req := newBootstrapRequest("abc123", "https://attacker.example.com", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", w.Code, w.Body.String())
	}
}

func TestBootstrapWebLinkNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newBootstrapHandler(nil, &bootstrapEmbedSvc{})

	r := gin.New()
	r.POST("/api/v1/embed/web/:slug/bootstrap", h.BootstrapWebLink)

	req := newBootstrapRequest("missing-slug", "https://app.example.com", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestBootstrapWebLinkDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gl := &types.GuestLinkChannel{
		ID: "guest-1", TenantID: 7, AgentID: "agent-1", WebSlug: "abc123", Enabled: false,
	}
	h := newBootstrapHandler(gl, &bootstrapEmbedSvc{})

	r := gin.New()
	r.POST("/api/v1/embed/web/:slug/bootstrap", h.BootstrapWebLink)

	req := newBootstrapRequest("abc123", "https://app.example.com", "app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", w.Code, w.Body.String())
	}
}
