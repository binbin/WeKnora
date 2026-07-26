package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	openAPIChatChannel               = "api"
	openAPIAgentCompleteWait         = 2 * time.Second
	openAPIChatCompletionObject      = "chat.completion"
	openAPIChatCompletionChunkObject = "chat.completion.chunk"
	openAPISSEDonePayload            = "[DONE]"
	openAPIErrTypeInvalidRequest     = "invalid_request_error"
	openAPIErrTypePermission         = "permission_error"
	openAPIErrTypeServer             = "server_error"
	openAPIErrTypeAuth               = "authentication_error"
)

// OpenAPIChatHandler serves OpenAI-compatible chat completions for publish keys.
type OpenAPIChatHandler struct {
	sessions interfaces.SessionService
	messages interfaces.MessageService
	agents   interfaces.CustomAgentService
}

// NewOpenAPIChatHandler constructs the OpenAI-compatible chat completions handler.
func NewOpenAPIChatHandler(
	sessions interfaces.SessionService,
	messages interfaces.MessageService,
	agents interfaces.CustomAgentService,
) *OpenAPIChatHandler {
	return &OpenAPIChatHandler{
		sessions: sessions,
		messages: messages,
		agents:   agents,
	}
}

type openAIChatCompletionRequest struct {
	Messages  []openAIChatMessage `json:"messages"`
	Stream    bool                `json:"stream"`
	Model     string              `json:"model"`
	SessionID string              `json:"session_id"`
	ChatID    string              `json:"chat_id"`
	AgentID   string              `json:"agent_id"`
}

type openAIChatCompletionResponse struct {
	ID        string                       `json:"id"`
	Object    string                       `json:"object"`
	Model     string                       `json:"model"`
	Choices   []openAIChatCompletionChoice `json:"choices"`
	Usage     openAIChatCompletionUsage    `json:"usage"`
	SessionID string                       `json:"session_id"`
}

type openAIChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      openAIChatResponseMsg `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type openAIChatResponseMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openAIChatCompletionChunk is an OpenAI-compatible SSE chunk. session_id is a
// WeKnora extension present on every chunk for client correlation.
type openAIChatCompletionChunk struct {
	ID        string                            `json:"id"`
	Object    string                            `json:"object"`
	Model     string                            `json:"model"`
	Choices   []openAIChatCompletionChunkChoice `json:"choices"`
	SessionID string                            `json:"session_id"`
}

type openAIChatCompletionChunkChoice struct {
	Index        int             `json:"index"`
	Delta        openAIChatDelta `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

type openAIChatDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type openAPIStreamPiece struct {
	content string
	done    bool
	err     error
}

// ChatCompletions handles POST /api/v1/chat/completions (non-stream P0).
func (h *OpenAPIChatHandler) ChatCompletions(c *gin.Context) {
	ctx := c.Request.Context()
	pubCtx, ok := types.AgentPublishAPIKeyContextFromContext(ctx)
	if !ok || pubCtx.KeyID == 0 || pubCtx.AgentID == "" {
		writeOpenAPIError(
			c,
			http.StatusUnauthorized,
			openAPIErrTypeAuth,
			"unauthorized",
			"missing publish api key context",
		)
		return
	}

	var req openAIChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeOpenAPIError(
			c,
			http.StatusBadRequest,
			openAPIErrTypeInvalidRequest,
			"invalid_request",
			"invalid request body",
		)
		return
	}
	if len(req.Messages) == 0 {
		writeOpenAPIError(
			c,
			http.StatusBadRequest,
			openAPIErrTypeInvalidRequest,
			"invalid_request",
			"messages is required",
		)
		return
	}

	bodyAgentID := strings.TrimSpace(req.AgentID)
	if bodyAgentID != "" && bodyAgentID != pubCtx.AgentID {
		writeOpenAPIError(
			c,
			http.StatusForbidden,
			openAPIErrTypePermission,
			"agent_unavailable",
			"agent is not available for this api key",
		)
		return
	}

	agent, err := h.agents.GetAgentByID(ctx, pubCtx.AgentID)
	if err != nil || agent == nil {
		logger.Warnf(
			ctx,
			"openapi chat: agent unavailable (agent_id=%s): %v",
			pubCtx.AgentID,
			err,
		)
		writeOpenAPIError(
			c,
			http.StatusForbidden,
			openAPIErrTypePermission,
			"agent_unavailable",
			"agent is not available for this api key",
		)
		return
	}

	query, err := lastUserQuery(req.Messages)
	if err != nil {
		writeOpenAPIError(
			c,
			http.StatusBadRequest,
			openAPIErrTypeInvalidRequest,
			"invalid_request",
			err.Error(),
		)
		return
	}

	session, err := h.resolveOpenAPISession(ctx, pubCtx, req, agent)
	if err != nil {
		writeOpenAPISessionError(c, err)
		return
	}

	if req.Stream {
		// Task 7 owns SSE streaming; keep a clear placeholder path.
		h.chatCompletionsStream(c, &req, pubCtx, session, agent, query)
		return
	}

	answer, err := h.runNonStreamQA(ctx, session, agent, query)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": session.ID,
			"agent_id":   agent.ID,
		})
		writeOpenAPIError(
			c,
			http.StatusInternalServerError,
			openAPIErrTypeServer,
			"server_error",
			"failed to generate completion",
		)
		return
	}

	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = agent.Name
	}
	c.JSON(http.StatusOK, openAIChatCompletionResponse{
		ID:     "chatcmpl-" + uuid.New().String(),
		Object: openAPIChatCompletionObject,
		Model:  modelName,
		Choices: []openAIChatCompletionChoice{
			{
				Index: 0,
				Message: openAIChatResponseMsg{
					Role:    "assistant",
					Content: answer,
				},
				FinishReason: "stop",
			},
		},
		Usage: openAIChatCompletionUsage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
		SessionID: session.ID,
	})
}

// chatCompletionsStream writes OpenAI-compatible SSE chunks for stream=true.
func (h *OpenAPIChatHandler) chatCompletionsStream(
	c *gin.Context,
	req *openAIChatCompletionRequest,
	_ types.AgentPublishAPIKeyContext,
	session *types.Session,
	agent *types.CustomAgent,
	query string,
) {
	ctx := c.Request.Context()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeOpenAPIError(
			c,
			http.StatusInternalServerError,
			openAPIErrTypeServer,
			"server_error",
			"streaming is not supported",
		)
		return
	}

	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = agent.Name
	}
	completionID := "chatcmpl-" + uuid.New().String()

	setOpenAPISSEHeaders(c)
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()

	answer, err := h.runStreamQA(
		ctx, c, flusher, session, agent, query, completionID, modelName,
	)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": session.ID,
			"agent_id":   agent.ID,
		})
		// Headers already sent; best-effort error notice then terminate.
		_ = writeOpenAPISSEData(c, flusher, map[string]any{
			"error": map[string]string{
				"message": "failed to generate completion",
				"type":    openAPIErrTypeServer,
				"code":    "server_error",
			},
		})
		_ = writeOpenAPISSEDone(c, flusher)
		return
	}
	logger.Infof(
		ctx,
		"openapi chat stream completed: session=%s answer_len=%d",
		session.ID,
		len(answer),
	)
}

func setOpenAPISSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
}

func writeOpenAPISSEData(
	c *gin.Context, flusher http.Flusher, payload any,
) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func writeOpenAPISSEDone(c *gin.Context, flusher http.Flusher) error {
	if _, err := fmt.Fprintf(
		c.Writer, "data: %s\n\n", openAPISSEDonePayload,
	); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func (h *OpenAPIChatHandler) writeOpenAPIChunk(
	c *gin.Context,
	flusher http.Flusher,
	completionID, modelName, sessionID string,
	delta openAIChatDelta,
	finishReason *string,
) error {
	return writeOpenAPISSEData(c, flusher, openAIChatCompletionChunk{
		ID:     completionID,
		Object: openAPIChatCompletionChunkObject,
		Model:  modelName,
		Choices: []openAIChatCompletionChunkChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
		SessionID: sessionID,
	})
}

func (h *OpenAPIChatHandler) runStreamQA(
	ctx context.Context,
	c *gin.Context,
	flusher http.Flusher,
	session *types.Session,
	agent *types.CustomAgent,
	query, completionID, modelName string,
) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pieces := make(chan openAPIStreamPiece, 64)
	eventBus := event.NewEventBus()
	var answerMu sync.Mutex
	var answerBuilder strings.Builder
	completeDone := make(chan struct{})
	var completeOnce sync.Once
	closeComplete := func() {
		completeOnce.Do(func() { close(completeDone) })
	}

	sendPiece := func(piece openAPIStreamPiece) {
		select {
		case pieces <- piece:
		case <-ctx.Done():
		}
	}

	h.registerOpenAPIStreamHandlers(
		eventBus, &answerMu, &answerBuilder, sendPiece, closeComplete,
	)

	useAgent := agent != nil && agent.IsAgentMode()
	userMsg, assistantMsg, err := h.createOpenAPIQAMessages(
		ctx, session.ID, query,
	)
	if err != nil {
		return "", err
	}

	go h.runOpenAPIQA(
		ctx, session, agent, query, userMsg.ID, assistantMsg.ID,
		useAgent, eventBus, sendPiece, closeComplete,
	)

	if err := h.writeOpenAPIChunk(
		c, flusher, completionID, modelName, session.ID,
		openAIChatDelta{Role: "assistant"}, nil,
	); err != nil {
		return "", err
	}

	streamErr, err := h.consumeOpenAPIStreamPieces(
		ctx, c, flusher, pieces, completionID, modelName, session.ID,
	)
	if err != nil {
		assistantMsg.Content = "抱歉，回答已被取消。"
		assistantMsg.IsCompleted = true
		_ = h.messages.UpdateMessage(
			context.WithoutCancel(ctx), assistantMsg,
		)
		return "", err
	}

	if useAgent {
		waitForOpenAPIAgentComplete(ctx, completeDone, session.ID)
	}

	answerMu.Lock()
	answer := answerBuilder.String()
	answerMu.Unlock()

	if answer == "" && streamErr != nil {
		return "", streamErr
	}
	if answer == "" {
		answer = "抱歉，我暂时无法回答这个问题。"
		_ = h.writeOpenAPIChunk(
			c, flusher, completionID, modelName, session.ID,
			openAIChatDelta{Content: answer}, nil,
		)
	}

	finishReasonStop := "stop"
	if err := h.writeOpenAPIChunk(
		c, flusher, completionID, modelName, session.ID,
		openAIChatDelta{}, &finishReasonStop,
	); err != nil {
		return answer, err
	}
	if err := writeOpenAPISSEDone(c, flusher); err != nil {
		return answer, err
	}

	assistantMsg.Content = answer
	assistantMsg.IsCompleted = true
	if err := h.messages.UpdateMessage(ctx, assistantMsg); err != nil {
		logger.Warnf(
			ctx,
			"openapi chat: failed to update assistant message: %v",
			err,
		)
	}
	return answer, nil
}

func (h *OpenAPIChatHandler) registerOpenAPIStreamHandlers(
	eventBus *event.EventBus,
	answerMu *sync.Mutex,
	answerBuilder *strings.Builder,
	sendPiece func(openAPIStreamPiece),
	closeComplete func(),
) {
	eventBus.On(event.EventAgentFinalAnswer, func(
		_ context.Context, evt event.Event,
	) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			return nil
		}
		if data.Content != "" {
			answerMu.Lock()
			answerBuilder.WriteString(data.Content)
			answerMu.Unlock()
			sendPiece(openAPIStreamPiece{content: data.Content})
		}
		if data.Done {
			sendPiece(openAPIStreamPiece{done: true})
		}
		return nil
	})

	eventBus.On(event.EventError, func(
		_ context.Context, evt event.Event,
	) error {
		data, ok := evt.Data.(event.ErrorData)
		if !ok {
			return nil
		}
		sendPiece(openAPIStreamPiece{
			err: fmt.Errorf("QA pipeline error: %s", data.Error),
		})
		closeComplete()
		return nil
	})

	eventBus.On(event.EventAgentComplete, func(
		_ context.Context, evt event.Event,
	) error {
		data, ok := evt.Data.(event.AgentCompleteData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		needsFallback := answerBuilder.Len() == 0 &&
			strings.TrimSpace(data.FinalAnswer) != ""
		fallback := data.FinalAnswer
		answerMu.Unlock()
		if needsFallback {
			answerMu.Lock()
			answerBuilder.WriteString(fallback)
			answerMu.Unlock()
			sendPiece(openAPIStreamPiece{content: fallback})
		}
		sendPiece(openAPIStreamPiece{done: true})
		closeComplete()
		return nil
	})
}

func (h *OpenAPIChatHandler) createOpenAPIQAMessages(
	ctx context.Context, sessionID, query string,
) (*types.Message, *types.Message, error) {
	requestID := uuid.New().String()
	userMsg, err := h.messages.CreateMessage(ctx, &types.Message{
		SessionID:   sessionID,
		Role:        "user",
		Content:     query,
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: true,
		Channel:     openAPIChatChannel,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create user message: %w", err)
	}
	assistantMsg, err := h.messages.CreateMessage(ctx, &types.Message{
		SessionID:   sessionID,
		Role:        "assistant",
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: false,
		Channel:     openAPIChatChannel,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create assistant message: %w", err)
	}
	return userMsg, assistantMsg, nil
}

func (h *OpenAPIChatHandler) runOpenAPIQA(
	ctx context.Context,
	session *types.Session,
	agent *types.CustomAgent,
	query, userMsgID, assistantMsgID string,
	useAgent bool,
	eventBus *event.EventBus,
	sendPiece func(openAPIStreamPiece),
	closeComplete func(),
) {
	qaReq := &types.QARequest{
		Session:            session,
		Query:              query,
		AssistantMessageID: assistantMsgID,
		CustomAgent:        agent,
		UserMessageID:      userMsgID,
		WebSearchEnabled: agent != nil &&
			agent.Config.WebSearchEnabled,
	}
	var runErr error
	if useAgent {
		runErr = h.sessions.AgentQA(ctx, qaReq, eventBus)
	} else {
		runErr = h.sessions.KnowledgeQA(ctx, qaReq, eventBus)
	}
	if runErr != nil {
		sendPiece(openAPIStreamPiece{
			err: fmt.Errorf("QA execution error: %w", runErr),
		})
		closeComplete()
	}
}

func (h *OpenAPIChatHandler) consumeOpenAPIStreamPieces(
	ctx context.Context,
	c *gin.Context,
	flusher http.Flusher,
	pieces <-chan openAPIStreamPiece,
	completionID, modelName, sessionID string,
) (error, error) {
	for {
		select {
		case piece := <-pieces:
			if piece.err != nil {
				return piece.err, nil
			}
			if piece.content != "" {
				if err := h.writeOpenAPIChunk(
					c, flusher, completionID, modelName, sessionID,
					openAIChatDelta{Content: piece.content}, nil,
				); err != nil {
					return nil, err
				}
			}
			if piece.done {
				return nil, nil
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("QA cancelled: %w", ctx.Err())
		}
	}
}

func (h *OpenAPIChatHandler) resolveOpenAPISession(
	ctx context.Context,
	pubCtx types.AgentPublishAPIKeyContext,
	req openAIChatCompletionRequest,
	agent *types.CustomAgent,
) (*types.Session, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(req.ChatID)
	}
	ownerID := types.SessionOwnerIDFromContext(ctx)
	state := buildOpenAPILastRequestState(pubCtx.AgentID, agent)

	if sessionID == "" {
		created, err := h.sessions.CreateSession(ctx, &types.Session{
			TenantID:         pubCtx.TenantID,
			UserID:           ownerID,
			Title:            "API Chat",
			Description:      "openai chat completions",
			LastRequestState: state,
		})
		if err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}
		return created, nil
	}

	session, err := h.sessions.GetOwnedSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, errOpenAPISessionForbidden
	}
	if session.TenantID != pubCtx.TenantID {
		return nil, errOpenAPISessionForbidden
	}
	if session.UserID != "" && ownerID != "" && session.UserID != ownerID {
		return nil, errOpenAPISessionForbidden
	}
	if session.LastRequestState != nil &&
		session.LastRequestState.AgentID != "" &&
		session.LastRequestState.AgentID != pubCtx.AgentID {
		return nil, errOpenAPISessionForbidden
	}
	if session.LastRequestState == nil ||
		session.LastRequestState.AgentID == "" {
		if err := h.sessions.UpdateSessionLastRequestState(
			ctx, session.ID, state,
		); err != nil {
			logger.Warnf(
				ctx,
				"openapi chat: failed to set session agent state: %v",
				err,
			)
		} else {
			session.LastRequestState = state
		}
	}
	return session, nil
}

func buildOpenAPILastRequestState(
	agentID string, agent *types.CustomAgent,
) *types.SessionLastRequestState {
	state := &types.SessionLastRequestState{AgentID: agentID}
	if agent == nil {
		return state
	}
	if state.AgentID == "" {
		state.AgentID = agent.ID
	}
	state.AgentEnabled = agent.IsAgentMode()
	state.ModelID = agent.Config.ModelID
	state.WebSearchEnabled = agent.Config.WebSearchEnabled
	if len(agent.Config.KnowledgeBases) > 0 {
		state.KnowledgeBaseIDs = append(
			[]string(nil), agent.Config.KnowledgeBases...,
		)
	}
	return state
}

func (h *OpenAPIChatHandler) runNonStreamQA(
	ctx context.Context,
	session *types.Session,
	agent *types.CustomAgent,
	query string,
) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventBus := event.NewEventBus()
	var answerMu sync.Mutex
	var answerBuilder strings.Builder
	var qaErr error
	done := make(chan struct{})
	completeDone := make(chan struct{})
	var closeOnce sync.Once
	var completeOnce sync.Once
	closeDone := func() { closeOnce.Do(func() { close(done) }) }
	closeComplete := func() {
		completeOnce.Do(func() { close(completeDone) })
	}

	eventBus.On(event.EventAgentFinalAnswer, func(
		_ context.Context, evt event.Event,
	) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		answerBuilder.WriteString(data.Content)
		answerMu.Unlock()
		if data.Done {
			closeDone()
		}
		return nil
	})

	eventBus.On(event.EventError, func(
		_ context.Context, evt event.Event,
	) error {
		data, ok := evt.Data.(event.ErrorData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		qaErr = fmt.Errorf("QA pipeline error: %s", data.Error)
		answerMu.Unlock()
		closeDone()
		closeComplete()
		return nil
	})

	eventBus.On(event.EventAgentComplete, func(
		_ context.Context, evt event.Event,
	) error {
		data, ok := evt.Data.(event.AgentCompleteData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		if answerBuilder.Len() == 0 &&
			strings.TrimSpace(data.FinalAnswer) != "" {
			answerBuilder.WriteString(data.FinalAnswer)
		}
		answerMu.Unlock()
		closeComplete()
		closeDone()
		return nil
	})

	useAgent := agent != nil && agent.IsAgentMode()
	requestID := uuid.New().String()

	userMsg, err := h.messages.CreateMessage(ctx, &types.Message{
		SessionID:   session.ID,
		Role:        "user",
		Content:     query,
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: true,
		Channel:     openAPIChatChannel,
	})
	if err != nil {
		return "", fmt.Errorf("create user message: %w", err)
	}

	assistantMsg, err := h.messages.CreateMessage(ctx, &types.Message{
		SessionID:   session.ID,
		Role:        "assistant",
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: false,
		Channel:     openAPIChatChannel,
	})
	if err != nil {
		return "", fmt.Errorf("create assistant message: %w", err)
	}

	go func() {
		qaReq := &types.QARequest{
			Session:            session,
			Query:              query,
			AssistantMessageID: assistantMsg.ID,
			CustomAgent:        agent,
			UserMessageID:      userMsg.ID,
			WebSearchEnabled: agent != nil &&
				agent.Config.WebSearchEnabled,
		}
		var runErr error
		if useAgent {
			runErr = h.sessions.AgentQA(ctx, qaReq, eventBus)
		} else {
			runErr = h.sessions.KnowledgeQA(ctx, qaReq, eventBus)
		}
		if runErr != nil {
			answerMu.Lock()
			qaErr = fmt.Errorf("QA execution error: %w", runErr)
			answerMu.Unlock()
			closeDone()
			closeComplete()
		}
	}()

	select {
	case <-done:
		if useAgent {
			waitForOpenAPIAgentComplete(ctx, completeDone, session.ID)
		}
	case <-ctx.Done():
		assistantMsg.Content = "抱歉，回答已被取消。"
		assistantMsg.IsCompleted = true
		_ = h.messages.UpdateMessage(
			context.WithoutCancel(ctx), assistantMsg,
		)
		return "", fmt.Errorf("QA cancelled: %w", ctx.Err())
	}

	answerMu.Lock()
	answer := answerBuilder.String()
	qaError := qaErr
	answerMu.Unlock()

	if answer == "" && qaError != nil {
		return "", qaError
	}
	if answer == "" {
		answer = "抱歉，我暂时无法回答这个问题。"
	}

	assistantMsg.Content = answer
	assistantMsg.IsCompleted = true
	if err := h.messages.UpdateMessage(ctx, assistantMsg); err != nil {
		logger.Warnf(
			ctx,
			"openapi chat: failed to update assistant message: %v",
			err,
		)
	}
	return answer, nil
}

func waitForOpenAPIAgentComplete(
	ctx context.Context, completeDone <-chan struct{}, sessionID string,
) {
	timer := time.NewTimer(openAPIAgentCompleteWait)
	defer timer.Stop()
	select {
	case <-completeDone:
	case <-ctx.Done():
		logger.Warnf(
			ctx,
			"openapi chat: context ended before agent complete: session=%s",
			sessionID,
		)
	case <-timer.C:
		logger.Warnf(
			ctx,
			"openapi chat: timed out waiting for agent complete: session=%s",
			sessionID,
		)
	}
}

var errOpenAPISessionForbidden = fmt.Errorf("session_forbidden")

func writeOpenAPISessionError(c *gin.Context, err error) {
	if err == errOpenAPISessionForbidden {
		writeOpenAPIError(
			c,
			http.StatusForbidden,
			openAPIErrTypePermission,
			"session_forbidden",
			"session is not available for this api key",
		)
		return
	}
	logger.ErrorWithFields(c.Request.Context(), err, nil)
	writeOpenAPIError(
		c,
		http.StatusInternalServerError,
		openAPIErrTypeServer,
		"server_error",
		"failed to resolve session",
	)
}

func writeOpenAPIError(
	c *gin.Context, status int, typ, code, message string,
) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    typ,
			"code":    code,
		},
	})
}
