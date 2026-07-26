package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const agentPublishAPIKeyPrefixLen = 12

type agentPublishAPIKeyService struct {
	repo          interfaces.AgentPublishAPIKeyRepository
	lastUsedTouch sync.Map // key ID (uint64) -> time.Time of last persisted touch
}

func NewAgentPublishAPIKeyService(
	repo interfaces.AgentPublishAPIKeyRepository,
) interfaces.AgentPublishAPIKeyService {
	return &agentPublishAPIKeyService{repo: repo}
}

func (s *agentPublishAPIKeyService) Create(
	ctx context.Context, req interfaces.AgentPublishAPIKeyCreateRequest,
) (*interfaces.AgentPublishAPIKeyCreateResult, error) {
	if req.TenantID == 0 {
		return nil, errors.New("tenant_id is required")
	}
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		return nil, errors.New("agent_id is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	token, err := generateAgentPublishAPIKeyToken()
	if err != nil {
		return nil, err
	}
	expiresAt := req.ExpiresAt
	if expiresAt != nil {
		utc := expiresAt.UTC()
		expiresAt = &utc
	}
	keyPrefix := token
	if len(token) > agentPublishAPIKeyPrefixLen {
		keyPrefix = token[:agentPublishAPIKeyPrefixLen]
	}
	key := &types.AgentPublishAPIKey{
		TenantID:  req.TenantID,
		AgentID:   agentID,
		Name:      name,
		KeyPrefix: keyPrefix,
		KeyHash:   hashAgentPublishAPIKey(token),
		APIKey:    token,
		ExpiresAt: expiresAt,
		CreatedBy: strings.TrimSpace(req.CreatedBy),
	}
	if err := s.repo.Create(ctx, key); err != nil {
		return nil, err
	}
	return &interfaces.AgentPublishAPIKeyCreateResult{
		APIKey: key,
		Token:  token,
	}, nil
}

func (s *agentPublishAPIKeyService) Authenticate(
	ctx context.Context, token string,
) (*types.AgentPublishAPIKey, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, apprepo.ErrAgentPublishAPIKeyNotFound
	}
	if !strings.HasPrefix(token, types.AgentPublishAPIKeyPrefix) {
		return nil, apprepo.ErrAgentPublishAPIKeyNotFound
	}
	key, err := s.repo.GetByHash(ctx, hashAgentPublishAPIKey(token))
	if err != nil {
		return nil, err
	}
	if key.RevokedAt != nil {
		return nil, apprepo.ErrAgentPublishAPIKeyNotFound
	}
	if key.ExpiresAt != nil && time.Now().UTC().After(key.ExpiresAt.UTC()) {
		return nil, apprepo.ErrAgentPublishAPIKeyNotFound
	}
	s.touchLastUsedAsync(key.ID)
	return key, nil
}

func (s *agentPublishAPIKeyService) ListByAgent(
	ctx context.Context, tenantID uint64, agentID string,
) ([]*types.AgentPublishAPIKey, error) {
	return s.repo.ListByAgent(ctx, tenantID, agentID)
}

func (s *agentPublishAPIKeyService) Revoke(
	ctx context.Context, tenantID uint64, agentID string, keyID uint64,
) error {
	return s.repo.Revoke(ctx, tenantID, agentID, keyID)
}

// touchLastUsedAsync persists last_used_at at most once per key per
// apiKeyLastUsedMinInterval so auth latency stays free of the hot-path UPDATE.
func (s *agentPublishAPIKeyService) touchLastUsedAsync(keyID uint64) {
	now := time.Now().UTC()
	if previous, ok := s.lastUsedTouch.Load(keyID); ok {
		if now.Sub(previous.(time.Time)) < apiKeyLastUsedMinInterval {
			return
		}
	}
	s.lastUsedTouch.Store(keyID, now)
	go func(id uint64, at time.Time) {
		if err := s.repo.UpdateLastUsed(context.Background(), id, at); err != nil {
			logger.Warnf(
				context.Background(),
				"failed to update agent publish api key last_used_at (id=%d): %v",
				id,
				err,
			)
			s.lastUsedTouch.Delete(id)
		}
	}(keyID, now)
}

func generateAgentPublishAPIKeyToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return types.AgentPublishAPIKeyPrefix +
		base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func hashAgentPublishAPIKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
