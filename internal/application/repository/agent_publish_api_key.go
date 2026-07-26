package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrAgentPublishAPIKeyNotFound = errors.New("agent publish api key not found")

type agentPublishAPIKeyRepository struct {
	db *gorm.DB
}

func NewAgentPublishAPIKeyRepository(
	db *gorm.DB,
) interfaces.AgentPublishAPIKeyRepository {
	return &agentPublishAPIKeyRepository{db: db}
}

func (r *agentPublishAPIKeyRepository) Create(
	ctx context.Context, key *types.AgentPublishAPIKey,
) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *agentPublishAPIKeyRepository) GetByHash(
	ctx context.Context, hash string,
) (*types.AgentPublishAPIKey, error) {
	var key types.AgentPublishAPIKey
	// SkipHooks avoids AfterFind decrypt interfering with hash-only auth
	// lookups when SYSTEM_AES_KEY is unset or ciphertext is not needed.
	err := r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Where("key_hash = ?", hash).
		First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentPublishAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *agentPublishAPIKeyRepository) ListByAgent(
	ctx context.Context, tenantID uint64, agentID string,
) ([]*types.AgentPublishAPIKey, error) {
	var keys []*types.AgentPublishAPIKey
	err := r.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND agent_id = ? AND revoked_at IS NULL",
			tenantID,
			agentID,
		).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

func (r *agentPublishAPIKeyRepository) Revoke(
	ctx context.Context, tenantID uint64, agentID string, keyID uint64,
) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&types.AgentPublishAPIKey{}).
		Where(
			"id = ? AND tenant_id = ? AND agent_id = ? AND revoked_at IS NULL",
			keyID,
			tenantID,
			agentID,
		).
		Update("revoked_at", &now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAgentPublishAPIKeyNotFound
	}
	return nil
}

func (r *agentPublishAPIKeyRepository) UpdateLastUsed(
	ctx context.Context, keyID uint64, at time.Time,
) error {
	return r.db.WithContext(ctx).
		Model(&types.AgentPublishAPIKey{}).
		Where("id = ? AND revoked_at IS NULL", keyID).
		Update("last_used_at", &at).Error
}
