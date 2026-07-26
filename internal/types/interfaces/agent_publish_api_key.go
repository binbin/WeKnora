package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentPublishAPIKeyCreateRequest is the input for creating a publish API key.
type AgentPublishAPIKeyCreateRequest struct {
	TenantID  uint64
	AgentID   string
	Name      string
	CreatedBy string
	ExpiresAt *time.Time
}

// AgentPublishAPIKeyCreateResult returns the persisted row plus the one-time
// plaintext token shown to the caller at creation.
type AgentPublishAPIKeyCreateResult struct {
	APIKey *types.AgentPublishAPIKey
	Token  string // plaintext, once
}

// AgentPublishAPIKeyRepository persists agent publish API keys.
type AgentPublishAPIKeyRepository interface {
	Create(ctx context.Context, key *types.AgentPublishAPIKey) error
	GetByHash(ctx context.Context, hash string) (*types.AgentPublishAPIKey, error)
	ListByAgent(
		ctx context.Context, tenantID uint64, agentID string,
	) ([]*types.AgentPublishAPIKey, error)
	Revoke(
		ctx context.Context, tenantID uint64, agentID string, keyID uint64,
	) error
	UpdateLastUsed(ctx context.Context, keyID uint64, at time.Time) error
}

// AgentPublishAPIKeyService manages agent publish API key lifecycle and auth.
type AgentPublishAPIKeyService interface {
	Create(
		ctx context.Context, req AgentPublishAPIKeyCreateRequest,
	) (*AgentPublishAPIKeyCreateResult, error)
	Authenticate(
		ctx context.Context, token string,
	) (*types.AgentPublishAPIKey, error)
	ListByAgent(
		ctx context.Context, tenantID uint64, agentID string,
	) ([]*types.AgentPublishAPIKey, error)
	Revoke(
		ctx context.Context, tenantID uint64, agentID string, keyID uint64,
	) error
}
