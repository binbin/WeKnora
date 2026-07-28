package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/im"
	"github.com/Tencent/WeKnora/internal/im/wechat_oa"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// WeChatOAHandler manages OA pre-auth bind APIs and Cloud callbacks.
type WeChatOAHandler struct {
	imService    *im.Service
	preauthRepo  repository.WeChatOAPreauthRepository
	tenantRepo   interfaces.TenantRepository
	cloudFactory wechat_oa.CloudClientFactory
}

// NewWeChatOAHandler wires OA bind handlers.
func NewWeChatOAHandler(
	imService *im.Service,
	preauthRepo repository.WeChatOAPreauthRepository,
	tenantRepo interfaces.TenantRepository,
) *WeChatOAHandler {
	return &WeChatOAHandler{
		imService:   imService,
		preauthRepo: preauthRepo,
		tenantRepo:  tenantRepo,
		cloudFactory: func(ctx context.Context, tenantID uint64) (wechat_oa.CloudClient, error) {
			tenant, err := tenantRepo.GetTenantByID(ctx, tenantID)
			if err != nil || tenant == nil {
				return nil, fmt.Errorf("load tenant: %w", err)
			}
			if tenant.Credentials == nil {
				return nil, fmt.Errorf("weknoracloud_credentials_required")
			}
			creds := tenant.Credentials.GetTreeRAGCloud()
			if creds == nil {
				return nil, fmt.Errorf("weknoracloud_credentials_required")
			}
			return wechat_oa.NewHTTPCloudClient(
				"", creds.AppID, creds.AppSecret, nil,
			), nil
		},
	}
}

// CreatePreAuth POST /api/v1/agents/:id/wechat-oa/preauth
func (handler *WeChatOAHandler) CreatePreAuth(c *gin.Context) {
	ctx := c.Request.Context()
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id is required"})
		return
	}
	tenantID, ok := ctx.Value(types.TenantIDContextKey).(uint64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	baseURL := resolveOACallbackBaseURL()
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "callback_base_url_required"})
		return
	}
	if err := validateOACallbackBaseURL(baseURL); err != nil {
		logger.Warnf(ctx, "[WeChatOA] invalid callback base URL %q: %v", baseURL, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "callback_base_url_invalid"})
		return
	}

	cloud, err := handler.cloudFactory(ctx, tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "weknoracloud_credentials_required"})
		return
	}

	state, err := randomHex(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create state"})
		return
	}

	preauthResp, err := cloud.CreatePreAuth(ctx, wechat_oa.PreAuthRequest{
		InstanceBaseURL: baseURL,
		TenantID:        tenantID,
		AgentID:         agentID,
		State:           state,
	})
	if err != nil {
		logger.Errorf(ctx, "[WeChatOA] CreatePreAuth cloud error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create preauth"})
		return
	}

	row := &types.WeChatOAPreauth{
		TenantID:       tenantID,
		AgentID:        agentID,
		CloudPreauthID: preauthResp.PreAuthID,
		State:          state,
		Status:         "wait",
		QRCodeURL:      preauthResp.QRCodeURL,
		CallbackSecret: preauthResp.CallbackSecret,
		ExpiresAt:      preauthResp.ExpiresAt,
	}
	if row.ExpiresAt.IsZero() {
		row.ExpiresAt = time.Now().Add(30 * time.Minute)
	}
	if err := handler.preauthRepo.Create(ctx, row); err != nil {
		logger.Errorf(ctx, "[WeChatOA] save preauth: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save preauth"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"preauth_id": row.ID,
			"qrcode_url": row.QRCodeURL,
			"expires_at": row.ExpiresAt,
			"status":     row.Status,
		},
	})
}

// GetPreAuthStatus GET /api/v1/wechat-oa/preauth/:id
func (handler *WeChatOAHandler) GetPreAuthStatus(c *gin.Context) {
	ctx := c.Request.Context()
	preauthID := c.Param("id")
	tenantID, ok := ctx.Value(types.TenantIDContextKey).(uint64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	row, err := handler.preauthRepo.GetByID(ctx, preauthID)
	if err != nil || row == nil || row.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "preauth not found"})
		return
	}
	if row.Status == "wait" && time.Now().After(row.ExpiresAt) {
		row.Status = "expired"
		_ = handler.preauthRepo.Update(ctx, row)
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status":        row.Status,
			"channel_id":    row.ChannelID,
			"qrcode_url":    row.QRCodeURL,
			"error_message": row.ErrorMessage,
			"expires_at":    row.ExpiresAt,
		},
	})
}

type bindingCompleteRequest struct {
	State                  string `json:"state" binding:"required"`
	AuthorizerAppID        string `json:"authorizer_appid" binding:"required"`
	NickName               string `json:"nick_name"`
	PrincipalName          string `json:"principal_name"`
	HeadImg                string `json:"head_img"`
	ServiceType            int    `json:"service_type"`
	VerifyType             int    `json:"verify_type"`
	CloudBindingID         string `json:"cloud_binding_id" binding:"required"`
	InstanceCallbackSecret string `json:"instance_callback_secret" binding:"required"`
}

// BindingComplete POST /api/v1/im/wechat_oa/binding/complete (Cloud HMAC)
func (handler *WeChatOAHandler) BindingComplete(c *gin.Context) {
	ctx := c.Request.Context()
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))

	var req bindingCompleteRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	row, err := handler.preauthRepo.GetByState(ctx, req.State)
	if err != nil || row == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "preauth not found"})
		return
	}

	timestamp := c.GetHeader("X-WeKnora-OA-Timestamp")
	signature := c.GetHeader("X-WeKnora-OA-Signature")
	if err := wechat_oa.VerifyHMAC(
		row.CallbackSecret, timestamp, rawBody, signature, time.Now(), 5*time.Minute,
	); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "verification failed"})
		return
	}

	// Idempotent: Cloud retries after a successful bind must return the same channel.
	if row.Status == "bound" && row.ChannelID != "" {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"channel_id": row.ChannelID}})
		return
	}

	if time.Now().After(row.ExpiresAt) && row.Status != "bound" {
		row.Status = "expired"
		_ = handler.preauthRepo.Update(ctx, row)
		c.JSON(http.StatusGone, gin.H{"error": "preauth expired"})
		return
	}

	credsJSON, err := json.Marshal(map[string]any{
		"authorizer_appid":         req.AuthorizerAppID,
		"nick_name":                req.NickName,
		"principal_name":           req.PrincipalName,
		"head_img":                 req.HeadImg,
		"service_type":             req.ServiceType,
		"verify_type":              req.VerifyType,
		"cloud_binding_id":         req.CloudBindingID,
		"instance_callback_secret": req.InstanceCallbackSecret,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal credentials"})
		return
	}

	name := strings.TrimSpace(req.NickName)
	if name == "" {
		name = "微信公众号"
	}
	if handler.imService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "im service unavailable"})
		return
	}

	channel := &im.IMChannel{
		TenantID:    row.TenantID,
		AgentID:     row.AgentID,
		Platform:    "wechat_oa",
		Name:        name,
		Enabled:     true,
		Mode:        "cloud_relay",
		OutputMode:  "full",
		Credentials: types.JSON(credsJSON),
	}
	if err := handler.imService.CreateChannel(channel); err != nil {
		if strings.HasPrefix(err.Error(), "duplicate_bot:") {
			existing, resolveErr := handler.resolveDuplicateOAChannel(
				ctx, row, req.AuthorizerAppID, name, types.JSON(credsJSON),
			)
			if resolveErr != nil {
				logger.Errorf(ctx, "[WeChatOA] resolve duplicate: %v", resolveErr)
				c.JSON(http.StatusConflict, gin.H{
					"error": strings.TrimPrefix(err.Error(), "duplicate_bot: "),
				})
				return
			}
			channel = existing
		} else {
			logger.Errorf(ctx, "[WeChatOA] CreateChannel: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create channel"})
			return
		}
	}

	row.Status = "bound"
	row.ChannelID = channel.ID
	if err := handler.preauthRepo.Update(ctx, row); err != nil {
		logger.Errorf(ctx, "[WeChatOA] update preauth bound: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"channel_id": channel.ID}})
}

// resolveDuplicateOAChannel handles Cloud retries and same-agent reauth.
// Same tenant+agent: refresh credentials and return the existing channel.
// Otherwise: conflict (OA already bound elsewhere).
func (handler *WeChatOAHandler) resolveDuplicateOAChannel(
	ctx context.Context,
	row *types.WeChatOAPreauth,
	authorizerAppID, name string,
	credsJSON types.JSON,
) (*im.IMChannel, error) {
	botKey := "wechat_oa:" + strings.TrimSpace(authorizerAppID)
	existing, err := handler.imService.GetChannelByBotIdentity(botKey)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("duplicate channel not found for %s: %w", botKey, err)
	}
	if existing.TenantID != row.TenantID || existing.AgentID != row.AgentID {
		return nil, fmt.Errorf(
			"authorizer already bound to another agent/channel %s", existing.ID,
		)
	}
	existing.Name = name
	existing.Credentials = credsJSON
	existing.Enabled = true
	existing.Mode = "cloud_relay"
	existing.OutputMode = "full"
	if err := handler.imService.UpdateChannel(existing); err != nil {
		return nil, fmt.Errorf("refresh existing channel: %w", err)
	}
	logger.Infof(
		ctx,
		"[WeChatOA] reauth/idempotent bind reused channel=%s authorizer=%s",
		existing.ID,
		authorizerAppID,
	)
	return existing, nil
}

func resolveOACallbackBaseURL() string {
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv("WECHAT_OA_CALLBACK_BASE_URL")), "/"); value != "" {
		return value
	}
	return strings.TrimRight(strings.TrimSpace(os.Getenv("APP_EXTERNAL_URL")), "/")
}

// validateOACallbackBaseURL rejects URLs Cloud cannot call back to
// (localhost / loopback / missing host).
func validateOACallbackBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "0.0.0.0" {
		return fmt.Errorf("host %q is not reachable from TreeRAG Cloud", host)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return fmt.Errorf("loopback address %q is not reachable from TreeRAG Cloud", host)
	}
	return nil
}

func randomHex(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
