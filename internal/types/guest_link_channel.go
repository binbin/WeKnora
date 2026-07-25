package types

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GuestLinkChannel publishes an agent chat surface reachable via a shareable
// guest link (/w/:slug), without requiring embedding on an external website.
type GuestLinkChannel struct {
	ID       string `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `json:"tenant_id" gorm:"not null;index"`
	AgentID  string `json:"agent_id" gorm:"type:varchar(36);not null"`
	Name     string `json:"name" gorm:"type:varchar(255);not null;default:''"`
	Enabled  bool   `json:"enabled" gorm:"not null;default:true"`
	WebSlug  string `json:"web_slug" gorm:"type:varchar(16);not null;default:''"`
	// SessionSecret keys the session-handle HMAC; the slug is the public
	// credential, so this one stays server-side and is never serialized.
	SessionSecret          string         `json:"-" gorm:"type:varchar(64);not null;default:''"`
	WelcomeMessage         string         `json:"welcome_message" gorm:"type:text;not null;default:''"`
	RateLimitPerMinute     int            `json:"rate_limit_per_minute" gorm:"not null;default:30"`
	RateLimitPerDay        int            `json:"rate_limit_per_day" gorm:"not null;default:10000"`
	PrimaryColor           string         `json:"primary_color" gorm:"type:varchar(32);not null;default:''"`
	PageTitle              string         `json:"page_title" gorm:"type:varchar(255);not null;default:''"`
	HeaderTitleMode        string         `json:"header_title_mode" gorm:"type:varchar(32);not null;default:'channel'"`
	ShowSuggestedQuestions bool           `json:"show_suggested_questions" gorm:"not null;default:true"`
	AllowWebSearch         bool           `json:"allow_web_search" gorm:"not null;default:false"`
	AllowFileUpload        bool           `json:"allow_file_upload" gorm:"not null;default:false"`
	DefaultLocale          string         `json:"default_locale" gorm:"type:varchar(16);not null;default:''"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	DeletedAt              gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (GuestLinkChannel) TableName() string { return "guest_link_channels" }

func (ch *GuestLinkChannel) BeforeCreate(tx *gorm.DB) error {
	if ch.ID == "" {
		ch.ID = uuid.New().String()
	}
	if ch.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if IsBuiltinAgentID(ch.AgentID) {
		return fmt.Errorf("built-in agents cannot be used for guest links")
	}
	if ch.RateLimitPerMinute <= 0 {
		ch.RateLimitPerMinute = 30
	}
	if ch.RateLimitPerDay <= 0 {
		ch.RateLimitPerDay = DefaultEmbedRateLimitPerDay
	}
	if ch.HeaderTitleMode == "" {
		ch.HeaderTitleMode = DefaultEmbedHeaderTitleMode
	}
	return nil
}

// AsEmbedChannel maps a guest link into the runtime shape used by embed handlers.
//
// PublishToken carries the guest link's SessionSecret: embed handlers use that
// field only as the HMAC key for session handles, and guest links are never
// resolvable through the publish-token lookup (LookupForEmbed reads
// embed_channels only), so the secret is never accepted as a bearer token.
//
// AllowedOrigins is deliberately left empty: guest links are not embedded on
// third-party sites, and their bootstrap endpoint trusts same-host requests
// only, so there is no allowlist to map over.
func (ch *GuestLinkChannel) AsEmbedChannel() *EmbedChannel {
	return &EmbedChannel{
		ID:                     ch.ID,
		TenantID:               ch.TenantID,
		AgentID:                ch.AgentID,
		Name:                   ch.Name,
		Enabled:                ch.Enabled,
		PublishToken:           ch.SessionSecret,
		WelcomeMessage:         ch.WelcomeMessage,
		RateLimitPerMinute:     ch.RateLimitPerMinute,
		RateLimitPerDay:        ch.RateLimitPerDay,
		PrimaryColor:           ch.PrimaryColor,
		PageTitle:              ch.PageTitle,
		HeaderTitleMode:        ch.HeaderTitleMode,
		ShowSuggestedQuestions: ch.ShowSuggestedQuestions,
		AllowWebSearch:         ch.AllowWebSearch,
		AllowFileUpload:        ch.AllowFileUpload,
		DefaultLocale:          ch.DefaultLocale,
		CreatedAt:              ch.CreatedAt,
		UpdatedAt:              ch.UpdatedAt,
	}
}
