package types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WeChatOAPreauth stores a pending Cloud pre-auth QR binding session.
type WeChatOAPreauth struct {
	ID             string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID       uint64    `json:"tenant_id"`
	AgentID        string    `json:"agent_id"`
	CloudPreauthID string    `json:"cloud_preauth_id"`
	State          string    `json:"state"`
	Status         string    `json:"status"`
	QRCodeURL      string    `json:"qrcode_url"`
	CallbackSecret string    `json:"-"`
	ChannelID      string    `json:"channel_id"`
	ErrorMessage   string    `json:"error_message"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (WeChatOAPreauth) TableName() string { return "wechat_oa_preauths" }

func (row *WeChatOAPreauth) BeforeCreate(tx *gorm.DB) error {
	if row.ID == "" {
		row.ID = uuid.New().String()
	}
	return nil
}
