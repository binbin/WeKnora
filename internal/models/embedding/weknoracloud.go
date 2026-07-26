package embedding

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
	"github.com/Tencent/WeKnora/internal/models/utils"
	"github.com/google/uuid"
)

const treeRAGCloudEmbedPath = "/api/v1/embeddings"

// TreeRAGCloudEmbedder 实现 embedding.Embedder 接口，对接 TreeRAGCloud /api/v1/embeddings
type TreeRAGCloudEmbedder struct {
	modelName                 string
	remoteModelName           string
	modelID                   string
	appID                     string
	apiKey                    string
	baseURL                   string
	dimensions                int
	supportsDimensionOverride bool
	client                    *http.Client
	EmbedderPooler
}

// NewTreeRAGCloudEmbedder 构造 TreeRAGCloudEmbedder
func NewTreeRAGCloudEmbedder(config Config) (*TreeRAGCloudEmbedder, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("TreeRAGCloud embedder: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("TreeRAGCloud embedder: AppSecret is required")
	}
	remoteModelName := ""
	if config.ExtraConfig != nil {
		remoteModelName = strings.TrimSpace(config.ExtraConfig["remote_model_name"])
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = provider.TreeRAGCloudBaseURL
	}
	if err := validateEmbeddingBaseURL(baseURL); err != nil {
		return nil, err
	}
	return &TreeRAGCloudEmbedder{
		modelName:                 config.ModelName,
		remoteModelName:           remoteModelName,
		modelID:                   config.ModelID,
		appID:                     config.AppID,
		apiKey:                    config.AppSecret,
		baseURL:                   baseURL,
		dimensions:                config.Dimensions,
		supportsDimensionOverride: config.SupportsDimensionOverride,
		client:                    newEmbeddingHTTPClient(60 * time.Second),
	}, nil
}

type treeRAGCloudEmbedRequest struct {
	Model                string   `json:"model"`
	Input                []string `json:"input"`
	Dimensions           int      `json:"dimensions,omitempty"`
	TruncatePromptTokens int      `json:"truncate_prompt_tokens,omitempty"`
}

type treeRAGCloudEmbedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *TreeRAGCloudEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := e.BatchEmbed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("weknoracloud embedder: empty response")
	}
	return results[0], nil
}

func (e *TreeRAGCloudEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := treeRAGCloudEmbedRequest{Model: e.effectiveModelName(), Input: texts}
	if e.supportsDimensionOverride && e.dimensions > 0 {
		reqBody.Dimensions = e.dimensions
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("weknoracloud embedder: marshal: %w", err)
	}

	requestID := uuid.New().String()
	headers := utils.Sign(e.appID, e.apiKey, requestID, string(bodyBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+treeRAGCloudEmbedPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("weknoracloud embedder: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weknoracloud embedder: do request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("weknoracloud embedder: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weknoracloud embedder: status %d: %s", resp.StatusCode, string(respBytes))
	}

	var embedResp treeRAGCloudEmbedResponse
	if err := json.Unmarshal(respBytes, &embedResp); err != nil {
		return nil, fmt.Errorf("weknoracloud embedder: unmarshal: %w", err)
	}

	result := make([][]float32, len(texts))
	for _, item := range embedResp.Data {
		if item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	return result, nil
}

func (e *TreeRAGCloudEmbedder) BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error) {
	return e.BatchEmbed(ctx, texts)
}

func (e *TreeRAGCloudEmbedder) SetSupportsDimensionOverride(supported bool) {
	e.supportsDimensionOverride = supported
}

func (e *TreeRAGCloudEmbedder) effectiveModelName() string {
	if e.remoteModelName != "" {
		return e.remoteModelName
	}
	return e.modelName
}

func (e *TreeRAGCloudEmbedder) GetModelName() string { return e.modelName }
func (e *TreeRAGCloudEmbedder) GetModelID() string   { return e.modelID }
func (e *TreeRAGCloudEmbedder) GetDimensions() int   { return e.dimensions }
