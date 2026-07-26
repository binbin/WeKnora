package handler

import (
	"bytes"
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

// adminGuestLinkSvc is a minimal in-memory fake of
// interfaces.GuestLinkChannelService for admin CRUD handler tests.
type adminGuestLinkSvc struct {
	interfaces.GuestLinkChannelService
	byAgent map[string]*types.GuestLinkChannel
	byID    map[string]*types.GuestLinkChannel
}

func newAdminGuestLinkSvc() *adminGuestLinkSvc {
	return &adminGuestLinkSvc{
		byAgent: map[string]*types.GuestLinkChannel{},
		byID:    map[string]*types.GuestLinkChannel{},
	}
}

func (f *adminGuestLinkSvc) GetByAgent(
	_ context.Context, tenantID uint64, agentID string,
) (*types.GuestLinkChannel, error) {
	gl, ok := f.byAgent[agentID]
	if !ok || gl.TenantID != tenantID {
		return nil, nil
	}
	return gl, nil
}

func (f *adminGuestLinkSvc) Create(
	_ context.Context, tenantID uint64, agentID string, req *types.GuestLinkChannel,
) (*types.GuestLinkChannel, error) {
	if _, exists := f.byAgent[agentID]; exists {
		return nil, service.ErrGuestLinkExists
	}
	cp := *req
	cp.ID = "guest-" + agentID
	cp.TenantID = tenantID
	cp.AgentID = agentID
	cp.WebSlug = "slug-" + agentID
	f.byAgent[agentID] = &cp
	f.byID[cp.ID] = &cp
	return &cp, nil
}

func (f *adminGuestLinkSvc) Get(
	_ context.Context, tenantID uint64, id string,
) (*types.GuestLinkChannel, error) {
	gl, ok := f.byID[id]
	if !ok || gl.TenantID != tenantID {
		return nil, service.ErrGuestLinkNotFound
	}
	return gl, nil
}

func (f *adminGuestLinkSvc) Update(
	_ context.Context, tenantID uint64, id string, req *types.GuestLinkChannel,
	enabled, showSuggested, allowWebSearch, allowFileUpload *bool,
	rateLimitPerMinute, rateLimitPerDay *int,
) (*types.GuestLinkChannel, error) {
	gl, ok := f.byID[id]
	if !ok || gl.TenantID != tenantID {
		return nil, service.ErrGuestLinkNotFound
	}
	gl.Name = req.Name
	gl.WelcomeMessage = req.WelcomeMessage
	if enabled != nil {
		gl.Enabled = *enabled
	}
	if showSuggested != nil {
		gl.ShowSuggestedQuestions = *showSuggested
	}
	if allowWebSearch != nil {
		gl.AllowWebSearch = *allowWebSearch
	}
	if allowFileUpload != nil {
		gl.AllowFileUpload = *allowFileUpload
	}
	if rateLimitPerMinute != nil {
		gl.RateLimitPerMinute = *rateLimitPerMinute
	}
	if rateLimitPerDay != nil {
		gl.RateLimitPerDay = *rateLimitPerDay
	}
	return gl, nil
}

func (f *adminGuestLinkSvc) Delete(_ context.Context, tenantID uint64, id string) error {
	gl, ok := f.byID[id]
	if !ok || gl.TenantID != tenantID {
		return service.ErrGuestLinkNotFound
	}
	delete(f.byID, id)
	delete(f.byAgent, gl.AgentID)
	return nil
}

func newAdminGuestLinkRouter(svc interfaces.GuestLinkChannelService, tenantID uint64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewGuestLinkChannelHandler(svc, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Next()
	})
	r.GET("/api/v1/agents/:id/guest-links", h.GetGuestLinkByAgent)
	r.POST("/api/v1/agents/:id/guest-links", h.CreateGuestLink)
	r.GET("/api/v1/guest-links/:id", h.GetGuestLink)
	r.PUT("/api/v1/guest-links/:id", h.UpdateGuestLink)
	r.DELETE("/api/v1/guest-links/:id", h.DeleteGuestLink)
	return r
}

func TestGetGuestLinkByAgentReturnsNullWhenNoneExists(t *testing.T) {
	r := newAdminGuestLinkRouter(newAdminGuestLinkSvc(), 7)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-1/guest-links", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    any  `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data != nil {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestCreateGuestLinkSuccess(t *testing.T) {
	r := newAdminGuestLinkRouter(newAdminGuestLinkSvc(), 7)

	body, _ := json.Marshal(map[string]any{"name": "Support Link"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/guest-links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ID      string `json:"id"`
			WebSlug string `json:"web_slug"`
			WebURL  string `json:"web_url"`
			Name    string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Data.Name != "Support Link" || resp.Data.WebSlug == "" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Data.WebURL == "" {
		t.Fatalf("expected web_url to be populated, got empty")
	}
}

func TestCreateGuestLinkConflict(t *testing.T) {
	svc := newAdminGuestLinkSvc()
	r := newAdminGuestLinkRouter(svc, 7)

	body, _ := json.Marshal(map[string]any{"name": "First"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/guest-links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, want 201, body = %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/guest-links", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second create status = %d, want 409, body = %s", w2.Code, w2.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "guest_link_exists" {
		t.Fatalf("error = %q, want guest_link_exists", resp.Error)
	}
}

func TestGetGuestLinkNotFound(t *testing.T) {
	r := newAdminGuestLinkRouter(newAdminGuestLinkSvc(), 7)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/guest-links/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestUpdateAndDeleteGuestLink(t *testing.T) {
	svc := newAdminGuestLinkSvc()
	r := newAdminGuestLinkRouter(svc, 7)

	createBody, _ := json.Marshal(map[string]any{"name": "First"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/agents/agent-1/guest-links", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	updateBody, _ := json.Marshal(map[string]any{"name": "Renamed", "enabled": false})
	updateReq := httptest.NewRequest(
		http.MethodPut, "/api/v1/guest-links/"+created.Data.ID, bytes.NewReader(updateBody),
	)
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200, body = %s", updateW.Code, updateW.Body.String())
	}
	var updated struct {
		Data struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(updateW.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Data.Name != "Renamed" || updated.Data.Enabled {
		t.Fatalf("unexpected update response: %#v", updated)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/guest-links/"+created.Data.ID, nil)
	deleteW := httptest.NewRecorder()
	r.ServeHTTP(deleteW, deleteReq)
	if deleteW.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200, body = %s", deleteW.Code, deleteW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/guest-links/"+created.Data.ID, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusNotFound {
		t.Fatalf("post-delete get status = %d, want 404, body = %s", getW.Code, getW.Body.String())
	}
}
