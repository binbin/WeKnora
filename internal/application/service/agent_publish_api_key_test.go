package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// fakeAgentPublishAPIKeyRepo is an in-memory repository for service tests.
type fakeAgentPublishAPIKeyRepo struct {
	mu      sync.Mutex
	nextID  uint64
	byID    map[uint64]*types.AgentPublishAPIKey
	byHash  map[string]uint64
	touched []uint64
}

func newFakeAgentPublishAPIKeyRepo() *fakeAgentPublishAPIKeyRepo {
	return &fakeAgentPublishAPIKeyRepo{
		nextID: 1,
		byID:   make(map[uint64]*types.AgentPublishAPIKey),
		byHash: make(map[string]uint64),
	}
}

func (r *fakeAgentPublishAPIKeyRepo) Create(
	_ context.Context, key *types.AgentPublishAPIKey,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *key
	cloned.ID = r.nextID
	r.nextID++
	now := time.Now().UTC()
	cloned.CreatedAt = now
	cloned.UpdatedAt = now
	r.byID[cloned.ID] = &cloned
	r.byHash[cloned.KeyHash] = cloned.ID
	*key = cloned
	return nil
}

func (r *fakeAgentPublishAPIKeyRepo) GetByHash(
	_ context.Context, hash string,
) (*types.AgentPublishAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byHash[hash]
	if !ok {
		return nil, apprepo.ErrAgentPublishAPIKeyNotFound
	}
	cloned := *r.byID[id]
	return &cloned, nil
}

func (r *fakeAgentPublishAPIKeyRepo) ListByAgent(
	_ context.Context, tenantID uint64, agentID string,
) ([]*types.AgentPublishAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*types.AgentPublishAPIKey
	for _, key := range r.byID {
		if key.TenantID != tenantID || key.AgentID != agentID {
			continue
		}
		if key.RevokedAt != nil {
			continue
		}
		cloned := *key
		out = append(out, &cloned)
	}
	return out, nil
}

func (r *fakeAgentPublishAPIKeyRepo) Revoke(
	_ context.Context, tenantID uint64, agentID string, keyID uint64,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, ok := r.byID[keyID]
	if !ok || key.TenantID != tenantID || key.AgentID != agentID {
		return apprepo.ErrAgentPublishAPIKeyNotFound
	}
	if key.RevokedAt != nil {
		return apprepo.ErrAgentPublishAPIKeyNotFound
	}
	now := time.Now().UTC()
	key.RevokedAt = &now
	return nil
}

func (r *fakeAgentPublishAPIKeyRepo) UpdateLastUsed(
	_ context.Context, keyID uint64, at time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, ok := r.byID[keyID]
	if !ok || key.RevokedAt != nil {
		return apprepo.ErrAgentPublishAPIKeyNotFound
	}
	key.LastUsedAt = &at
	r.touched = append(r.touched, keyID)
	return nil
}

func TestAgentPublishAuthenticateRejectsRevokedAndExpired(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentPublishAPIKeyRepo()
	svc := NewAgentPublishAPIKeyService(repo)

	result, err := svc.Create(ctx, interfaces.AgentPublishAPIKeyCreateRequest{
		TenantID:  1,
		AgentID:   "agent-1",
		Name:      "valid",
		CreatedBy: "user-1",
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(result.Token, types.AgentPublishAPIKeyPrefix))

	got, err := svc.Authenticate(ctx, result.Token)
	require.NoError(t, err)
	require.Equal(t, result.APIKey.ID, got.ID)

	_, err = svc.Authenticate(ctx, "")
	require.Error(t, err)

	_, err = svc.Authenticate(ctx, "sk-not-a-publish-key")
	require.Error(t, err)
	require.True(
		t,
		errors.Is(err, apprepo.ErrAgentPublishAPIKeyNotFound),
		"wrong prefix should map to not-found: %v",
		err,
	)

	expiredAt := time.Now().UTC().Add(-time.Hour)
	expiredResult, err := svc.Create(ctx, interfaces.AgentPublishAPIKeyCreateRequest{
		TenantID:  1,
		AgentID:   "agent-1",
		Name:      "expired",
		CreatedBy: "user-1",
		ExpiresAt: &expiredAt,
	})
	require.NoError(t, err)
	_, err = svc.Authenticate(ctx, expiredResult.Token)
	require.Error(t, err)
	require.True(t, errors.Is(err, apprepo.ErrAgentPublishAPIKeyNotFound))

	require.NoError(t, svc.Revoke(ctx, 1, "agent-1", result.APIKey.ID))
	_, err = svc.Authenticate(ctx, result.Token)
	require.Error(t, err)
	require.True(t, errors.Is(err, apprepo.ErrAgentPublishAPIKeyNotFound))
}

func TestAgentPublishCreateTokenHasPrefixAndHashLookup(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentPublishAPIKeyRepo()
	svc := NewAgentPublishAPIKeyService(repo)

	result, err := svc.Create(ctx, interfaces.AgentPublishAPIKeyCreateRequest{
		TenantID: 1, AgentID: "agent-1", Name: "test", CreatedBy: "user-1",
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(result.Token, "wkpub_"))
	require.Equal(
		t,
		result.APIKey.KeyPrefix,
		result.Token[:min(12, len(result.Token))],
	)
	got, err := svc.Authenticate(ctx, result.Token)
	require.NoError(t, err)
	require.Equal(t, result.APIKey.ID, got.ID)
}

func TestAgentPublishListByAgentOmitsRevoked(t *testing.T) {
	ctx := context.Background()
	repo := newFakeAgentPublishAPIKeyRepo()
	svc := NewAgentPublishAPIKeyService(repo)

	active, err := svc.Create(ctx, interfaces.AgentPublishAPIKeyCreateRequest{
		TenantID: 1, AgentID: "agent-1", Name: "active", CreatedBy: "user-1",
	})
	require.NoError(t, err)
	revoked, err := svc.Create(ctx, interfaces.AgentPublishAPIKeyCreateRequest{
		TenantID: 1, AgentID: "agent-1", Name: "revoked", CreatedBy: "user-1",
	})
	require.NoError(t, err)
	require.NoError(t, svc.Revoke(ctx, 1, "agent-1", revoked.APIKey.ID))

	keys, err := svc.ListByAgent(ctx, 1, "agent-1")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, active.APIKey.ID, keys[0].ID)
}
