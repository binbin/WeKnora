package wechat_oa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/models/provider"
	modelsutils "github.com/Tencent/WeKnora/internal/models/utils"
	"github.com/google/uuid"
)

// CloudClient talks to WeKnora Cloud OA APIs for bind/unbind/send.
type CloudClient interface {
	CreatePreAuth(ctx context.Context, req PreAuthRequest) (*PreAuthResponse, error)
	GetPreAuth(ctx context.Context, cloudPreAuthID string) (*PreAuthStatus, error)
	Unbind(ctx context.Context, authorizerAppID string) error
	SendText(ctx context.Context, authorizerAppID, toUser, text string) error
}

// PreAuthRequest starts a Cloud third-party pre-auth session.
type PreAuthRequest struct {
	InstanceBaseURL string `json:"instance_base_url"`
	TenantID        uint64 `json:"tenant_id"`
	AgentID         string `json:"agent_id"`
	State           string `json:"state"`
}

// PreAuthResponse is returned by Cloud CreatePreAuth.
type PreAuthResponse struct {
	PreAuthID      string    `json:"preauth_id"`
	QRCodeURL      string    `json:"qrcode_url"`
	ExpiresAt      time.Time `json:"expires_at"`
	CallbackSecret string    `json:"callback_secret"`
}

// PreAuthStatus is returned by Cloud GetPreAuth (status sync).
type PreAuthStatus struct {
	Status            string `json:"status"`
	AuthorizerAppID   string `json:"authorizer_appid,omitempty"`
	NickName          string `json:"nick_name,omitempty"`
	PrincipalName     string `json:"principal_name,omitempty"`
	HeadImg           string `json:"head_img,omitempty"`
	CloudBindingID    string `json:"cloud_binding_id,omitempty"`
	CallbackSecret    string `json:"callback_secret,omitempty"`
	ErrorMessage      string `json:"error_message,omitempty"`
}

// HTTPCloudClient is the production Cloud OA HTTP client.
type HTTPCloudClient struct {
	baseURL    string
	appID      string
	appSecret  string
	httpClient *http.Client
}

// NewHTTPCloudClient builds a client. Empty baseURL uses WeKnoraCloudBaseURL.
func NewHTTPCloudClient(
	baseURL, appID, appSecret string,
	httpClient *http.Client,
) *HTTPCloudClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = provider.WeKnoraCloudBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPCloudClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		appID:      appID,
		appSecret:  appSecret,
		httpClient: httpClient,
	}
}

func (client *HTTPCloudClient) CreatePreAuth(
	ctx context.Context,
	req PreAuthRequest,
) (*PreAuthResponse, error) {
	var result PreAuthResponse
	if err := client.doJSON(ctx, http.MethodPost, "/api/v1/oa/preauth", req, &result); err != nil {
		return nil, err
	}
	if result.PreAuthID == "" || result.QRCodeURL == "" {
		return nil, fmt.Errorf("wechat_oa cloud: empty preauth response")
	}
	return &result, nil
}

func (client *HTTPCloudClient) GetPreAuth(
	ctx context.Context,
	cloudPreAuthID string,
) (*PreAuthStatus, error) {
	path := "/api/v1/oa/preauth/" + cloudPreAuthID
	var result PreAuthStatus
	if err := client.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (client *HTTPCloudClient) Unbind(
	ctx context.Context,
	authorizerAppID string,
) error {
	path := "/api/v1/oa/bindings/" + authorizerAppID + "/unbind"
	return client.doJSON(ctx, http.MethodPost, path, map[string]any{}, nil)
}

func (client *HTTPCloudClient) SendText(
	ctx context.Context,
	authorizerAppID, toUser, text string,
) error {
	payload := map[string]any{
		"authorizer_appid": authorizerAppID,
		"touser":           toUser,
		"msgtype":          "text",
		"text":             map[string]string{"content": text},
	}
	return client.doJSON(ctx, http.MethodPost, "/api/v1/oa/message/send", payload, nil)
}

func (client *HTTPCloudClient) doJSON(
	ctx context.Context,
	method, path string,
	body any,
	out any,
) error {
	bodyJSON := "{}"
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("wechat_oa cloud: marshal: %w", err)
		}
		bodyJSON = string(raw)
		bodyReader = bytes.NewReader(raw)
	} else if method != http.MethodGet {
		bodyReader = bytes.NewReader([]byte("{}"))
	}

	url := client.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("wechat_oa cloud: create request: %w", err)
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	requestID := uuid.New().String()
	for key, value := range modelsutils.Sign(
		client.appID, client.appSecret, requestID, bodyJSON,
	) {
		req.Header.Set(key, value)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wechat_oa cloud: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("wechat_oa cloud: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"wechat_oa cloud: status %d: %s",
			resp.StatusCode,
			string(respBody),
		)
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("wechat_oa cloud: decode: %w", err)
	}
	return nil
}
