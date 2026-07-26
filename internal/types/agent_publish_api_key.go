package types

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/utils"
	"gorm.io/gorm"
)

const AgentPublishAPIKeyPrefix = "wkpub_"

// AgentPublishAPIKey is a revocable publish credential bound to one agent.
// KeyHash is used for authentication lookup; APIKey is stored encrypted when
// SYSTEM_AES_KEY is set (same pattern as TenantAPIKey).
type AgentPublishAPIKey struct {
	ID         uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID   uint64     `json:"tenant_id" gorm:"index;not null"`
	AgentID    string     `json:"agent_id" gorm:"type:varchar(36);not null;index"`
	Name       string     `json:"name" gorm:"type:varchar(128);not null"`
	KeyPrefix  string     `json:"key_prefix" gorm:"type:varchar(32);not null"`
	KeyHash    string     `json:"-" gorm:"type:varchar(64);not null;uniqueIndex"`
	APIKey     string     `json:"-" gorm:"column:api_key;type:text;not null;default:''"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" gorm:"index"`
	CreatedBy  string     `json:"created_by" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (AgentPublishAPIKey) TableName() string {
	return "agent_publish_api_keys"
}

func (key *AgentPublishAPIKey) BeforeSave(tx *gorm.DB) error {
	if aesKey := utils.GetAESKey(); aesKey != nil && key.APIKey != "" {
		encrypted, err := utils.EncryptAESGCM(key.APIKey, aesKey)
		if err != nil {
			// Never fall through to storing the plaintext key: abort the
			// write so the caller sees the failure instead of silently
			// persisting an unencrypted secret.
			return fmt.Errorf(
				"encrypt agent_publish_api_keys.api_key (id=%d): %w",
				key.ID,
				err,
			)
		}
		tx.Statement.SetColumn("api_key", encrypted)
	}
	return nil
}

func (key *AgentPublishAPIKey) AfterFind(tx *gorm.DB) error {
	decrypted, err := utils.DecryptStoredSecret(key.APIKey)
	if err != nil {
		return fmt.Errorf(
			"decrypt agent_publish_api_keys.api_key (id=%d): %w",
			key.ID,
			err,
		)
	}
	key.APIKey = decrypted
	return nil
}

// MaskedKey returns wkpub_****abcd style display value from KeyPrefix.
func (key *AgentPublishAPIKey) MaskedKey() string {
	if key == nil || key.KeyPrefix == "" {
		return "—"
	}
	return key.KeyPrefix + "****"
}

// AgentPublishAPIKeyContext is the request-context projection for a publish key.
type AgentPublishAPIKeyContext struct {
	KeyID    uint64
	TenantID uint64
	AgentID  string
}

type agentPublishAPIKeyContextKey struct{}

func WithAgentPublishAPIKeyContext(
	ctx context.Context, value AgentPublishAPIKeyContext,
) context.Context {
	return context.WithValue(ctx, agentPublishAPIKeyContextKey{}, value)
}

func AgentPublishAPIKeyContextFromContext(
	ctx context.Context,
) (AgentPublishAPIKeyContext, bool) {
	if ctx == nil {
		return AgentPublishAPIKeyContext{}, false
	}
	value, ok := ctx.Value(agentPublishAPIKeyContextKey{}).(AgentPublishAPIKeyContext)
	return value, ok
}
