package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	testOpenAPITenantID  = uint64(7)
	testOpenAPIAgentID   = "agent-publish-1"
	testOpenAPIKeyID     = uint64(99)
	testOpenAPIToken     = "wkpub_test_token_abcdefghijklmnopqrst"
	testOpenAPIAnswer    = "openapi assistant reply"
)

type openAPIFakePublishKeyService struct {
	key *types.AgentPublishAPIKey
	err error
}

func (f *openAPIFakePublishKeyService) Create(
	context.Context, interfaces.AgentPublishAPIKeyCreateRequest,
) (*interfaces.AgentPublishAPIKeyCreateResult, error) {
	return nil, errors.New("not implemented")
}

func (f *openAPIFakePublishKeyService) Authenticate(
	_ context.Context, token string,
) (*types.AgentPublishAPIKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.key == nil {
		return nil, errors.New("invalid api key")
	}
	return f.key, nil
}

func (f *openAPIFakePublishKeyService) ListByAgent(
	context.Context, uint64, string,
) ([]*types.AgentPublishAPIKey, error) {
	return nil, errors.New("not implemented")
}

func (f *openAPIFakePublishKeyService) Revoke(
	context.Context, uint64, string, uint64,
) error {
	return errors.New("not implemented")
}

var _ interfaces.AgentPublishAPIKeyService = (*openAPIFakePublishKeyService)(nil)

type openAPIFakeTenantService struct {
	interfaces.TenantService
	tenant *types.Tenant
}

func (f *openAPIFakeTenantService) GetTenantByID(
	_ context.Context, id uint64,
) (*types.Tenant, error) {
	if f.tenant == nil || f.tenant.ID != id {
		return nil, errors.New("tenant not found")
	}
	return f.tenant, nil
}

type openAPIFakeAgentService struct {
	interfaces.CustomAgentService
	agent *types.CustomAgent
	err   error
}

func (f *openAPIFakeAgentService) GetAgentByID(
	_ context.Context, id string,
) (*types.CustomAgent, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.agent == nil || f.agent.ID != id {
		return nil, errors.New("agent not found")
	}
	return f.agent, nil
}

type openAPIFakeSessionService struct {
	interfaces.SessionService
	sessions map[string]*types.Session
	created  *types.Session
	answer   string
}

func (f *openAPIFakeSessionService) CreateSession(
	_ context.Context, session *types.Session,
) (*types.Session, error) {
	created := *session
	if created.ID == "" {
		created.ID = uuid.New().String()
	}
	if f.sessions == nil {
		f.sessions = map[string]*types.Session{}
	}
	f.sessions[created.ID] = &created
	f.created = &created
	return &created, nil
}

func (f *openAPIFakeSessionService) GetOwnedSession(
	_ context.Context, id string,
) (*types.Session, error) {
	session, ok := f.sessions[id]
	if !ok {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (f *openAPIFakeSessionService) UpdateSessionLastRequestState(
	_ context.Context, sessionID string, state *types.SessionLastRequestState,
) error {
	session, ok := f.sessions[sessionID]
	if !ok {
		return errors.New("session not found")
	}
	session.LastRequestState = state
	return nil
}

func (f *openAPIFakeSessionService) KnowledgeQA(
	ctx context.Context, _ *types.QARequest, bus *event.EventBus,
) error {
	content := f.answer
	if content == "" {
		content = testOpenAPIAnswer
	}
	_ = bus.Emit(ctx, event.Event{
		Type: event.EventAgentFinalAnswer,
		Data: event.AgentFinalAnswerData{Content: content, Done: true},
	})
	_ = bus.Emit(ctx, event.Event{
		Type: event.EventAgentComplete,
		Data: event.AgentCompleteData{FinalAnswer: content},
	})
	return nil
}

func (f *openAPIFakeSessionService) AgentQA(
	ctx context.Context, req *types.QARequest, bus *event.EventBus,
) error {
	return f.KnowledgeQA(ctx, req, bus)
}

type openAPIFakeMessageService struct {
	interfaces.MessageService
	messages []*types.Message
}

func (f *openAPIFakeMessageService) CreateMessage(
	_ context.Context, message *types.Message,
) (*types.Message, error) {
	created := *message
	if created.ID == "" {
		created.ID = uuid.New().String()
	}
	f.messages = append(f.messages, &created)
	return &created, nil
}

func (f *openAPIFakeMessageService) UpdateMessage(
	_ context.Context, message *types.Message,
) error {
	return nil
}

func newOpenAPITestRouter(
	t *testing.T,
	publishSvc interfaces.AgentPublishAPIKeyService,
	tenantSvc interfaces.TenantService,
	chatHandler *OpenAPIChatHandler,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(
		"/api/v1/chat/completions",
		middleware.PublishAPIKeyAuth(publishSvc, tenantSvc),
		chatHandler.ChatCompletions,
	)
	return router
}

func openAPIValidKey() *types.AgentPublishAPIKey {
	return &types.AgentPublishAPIKey{
		ID:       testOpenAPIKeyID,
		TenantID: testOpenAPITenantID,
		AgentID:  testOpenAPIAgentID,
		Name:     "test-key",
	}
}

func openAPITestAgent() *types.CustomAgent {
	return &types.CustomAgent{
		ID:       testOpenAPIAgentID,
		TenantID: testOpenAPITenantID,
		Name:     "Publish Agent",
		Config: types.CustomAgentConfig{
			AgentMode: types.AgentModeQuickAnswer,
		},
	}
}

func postOpenAPIChat(
	router *gin.Engine, token string, body any,
) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/chat/completions", bytes.NewReader(payload),
	)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestOpenAPIChatCompletionsUnauthorized(t *testing.T) {
	handler := NewOpenAPIChatHandler(
		&openAPIFakeSessionService{},
		&openAPIFakeMessageService{},
		&openAPIFakeAgentService{agent: openAPITestAgent()},
	)
	router := newOpenAPITestRouter(
		t,
		&openAPIFakePublishKeyService{key: openAPIValidKey()},
		&openAPIFakeTenantService{
			tenant: &types.Tenant{ID: testOpenAPITenantID},
		},
		handler,
	)

	recorder := postOpenAPIChat(router, "", map[string]any{
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
	})
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assertOpenAPIChatError(t, recorder.Body.Bytes(), "unauthorized")
}

func TestOpenAPIChatCompletionsEmptyMessages(t *testing.T) {
	handler := NewOpenAPIChatHandler(
		&openAPIFakeSessionService{},
		&openAPIFakeMessageService{},
		&openAPIFakeAgentService{agent: openAPITestAgent()},
	)
	router := newOpenAPITestRouter(
		t,
		&openAPIFakePublishKeyService{key: openAPIValidKey()},
		&openAPIFakeTenantService{
			tenant: &types.Tenant{ID: testOpenAPITenantID},
		},
		handler,
	)

	recorder := postOpenAPIChat(router, testOpenAPIToken, map[string]any{
		"messages": []any{},
	})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assertOpenAPIChatError(t, recorder.Body.Bytes(), "invalid_request")
}

func TestOpenAPIChatCompletionsWrongAgentID(t *testing.T) {
	handler := NewOpenAPIChatHandler(
		&openAPIFakeSessionService{},
		&openAPIFakeMessageService{},
		&openAPIFakeAgentService{agent: openAPITestAgent()},
	)
	router := newOpenAPITestRouter(
		t,
		&openAPIFakePublishKeyService{key: openAPIValidKey()},
		&openAPIFakeTenantService{
			tenant: &types.Tenant{ID: testOpenAPITenantID},
		},
		handler,
	)

	recorder := postOpenAPIChat(router, testOpenAPIToken, map[string]any{
		"agent_id": "other-agent",
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
	})
	require.Equal(t, http.StatusForbidden, recorder.Code)
	assertOpenAPIChatError(t, recorder.Body.Bytes(), "agent_unavailable")
}

func TestOpenAPIChatCompletionsNonStreamOK(t *testing.T) {
	sessions := &openAPIFakeSessionService{answer: testOpenAPIAnswer}
	handler := NewOpenAPIChatHandler(
		sessions,
		&openAPIFakeMessageService{},
		&openAPIFakeAgentService{agent: openAPITestAgent()},
	)
	router := newOpenAPITestRouter(
		t,
		&openAPIFakePublishKeyService{key: openAPIValidKey()},
		&openAPIFakeTenantService{
			tenant: &types.Tenant{ID: testOpenAPITenantID},
		},
		handler,
	)

	recorder := postOpenAPIChat(router, testOpenAPIToken, map[string]any{
		"model": "echo-model",
		"messages": []map[string]any{
			{"role": "user", "content": "hello openapi"},
		},
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var resp openAIChatCompletionResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, openAPIChatCompletionObject, resp.Object)
	require.Equal(t, "echo-model", resp.Model)
	require.NotEmpty(t, resp.SessionID)
	require.Len(t, resp.Choices, 1)
	require.Equal(t, "assistant", resp.Choices[0].Message.Role)
	require.Equal(t, testOpenAPIAnswer, resp.Choices[0].Message.Content)
	require.Equal(t, "stop", resp.Choices[0].FinishReason)
	require.NotNil(t, sessions.created)
	require.Equal(t, testOpenAPITenantID, sessions.created.TenantID)
	require.Equal(
		t,
		testOpenAPIAgentID,
		sessions.created.LastRequestState.AgentID,
	)
}

func assertOpenAPIChatError(t *testing.T, body []byte, code string) {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	errObj, ok := payload["error"].(map[string]any)
	require.True(t, ok, "missing error object: %s", string(body))
	require.Equal(t, code, errObj["code"])
}
