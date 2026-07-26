package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/models/utils"
	"github.com/google/uuid"
)

const treeRAGCloudRerankPath = "/api/v1/rerank"

// TreeRAGCloudReranker 实现 rerank.Reranker 接口，对接 TreeRAGCloud /api/v1/rerank
type TreeRAGCloudReranker struct {
	modelName       string
	remoteModelName string
	modelID         string
	appID           string
	apiKey          string
	baseURL         string
	client          *http.Client
}

// NewTreeRAGCloudReranker 构造 TreeRAGCloudReranker
func NewTreeRAGCloudReranker(config *RerankerConfig) (*TreeRAGCloudReranker, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("TreeRAGCloud reranker: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("TreeRAGCloud reranker: AppSecret is required")
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if err := validateRerankBaseURL(baseURL); err != nil {
		return nil, err
	}
	remoteModelName := ""
	if config.ExtraConfig != nil {
		remoteModelName = strings.TrimSpace(config.ExtraConfig["remote_model_name"])
	}
	return &TreeRAGCloudReranker{
		modelName:       config.ModelName,
		remoteModelName: remoteModelName,
		modelID:         config.ModelID,
		appID:           config.AppID,
		apiKey:          config.AppSecret,
		baseURL:         baseURL,
		client:          newRerankHTTPClient(60 * time.Second),
	}, nil
}

type treeRAGCloudRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type treeRAGCloudRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       struct {
			Text string `json:"text"`
		} `json:"document"`
	} `json:"results"`
}

func (r *TreeRAGCloudReranker) Rerank(ctx context.Context, query string, documents []string) ([]RankResult, error) {
	reqBody := treeRAGCloudRerankRequest{
		Model:     r.effectiveModelName(),
		Query:     query,
		Documents: documents,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("weknoracloud reranker: marshal: %w", err)
	}

	requestID := uuid.New().String()
	headers := utils.Sign(r.appID, r.apiKey, requestID, string(bodyBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+treeRAGCloudRerankPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("weknoracloud reranker: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weknoracloud reranker: do request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("weknoracloud reranker: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weknoracloud reranker: status %d: %s", resp.StatusCode, string(respBytes))
	}

	var rerankResp treeRAGCloudRerankResponse
	if err := json.Unmarshal(respBytes, &rerankResp); err != nil {
		return nil, fmt.Errorf("weknoracloud reranker: unmarshal: %w", err)
	}

	results := make([]RankResult, 0, len(rerankResp.Results))
	for _, item := range rerankResp.Results {
		results = append(results, RankResult{
			Index:          item.Index,
			RelevanceScore: item.RelevanceScore,
			Document:       DocumentInfo{Text: item.Document.Text},
		})
	}
	return results, nil
}

func (r *TreeRAGCloudReranker) effectiveModelName() string {
	if r.remoteModelName != "" {
		return r.remoteModelName
	}
	return r.modelName
}

func (r *TreeRAGCloudReranker) GetModelName() string { return r.modelName }
func (r *TreeRAGCloudReranker) GetModelID() string   { return r.modelID }
