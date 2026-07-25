package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// GuestLinkChannelRepository persists guest link channel rows.
type GuestLinkChannelRepository interface {
	Create(ctx context.Context, ch *types.GuestLinkChannel) error
	GetByID(ctx context.Context, id string) (*types.GuestLinkChannel, error)
	GetByWebSlug(ctx context.Context, slug string) (*types.GuestLinkChannel, error)
	GetByAgent(ctx context.Context, tenantID uint64, agentID string) (*types.GuestLinkChannel, error)
	Update(ctx context.Context, ch *types.GuestLinkChannel) error
	Delete(ctx context.Context, tenantID uint64, id string) error
}

// GuestLinkChannelService manages guest link channel lifecycle.
type GuestLinkChannelService interface {
	GetByAgent(ctx context.Context, tenantID uint64, agentID string) (*types.GuestLinkChannel, error)
	Create(ctx context.Context, tenantID uint64, agentID string, req *types.GuestLinkChannel) (*types.GuestLinkChannel, error)
	Update(ctx context.Context, tenantID uint64, id string, req *types.GuestLinkChannel, enabled *bool) (*types.GuestLinkChannel, error)
	Delete(ctx context.Context, tenantID uint64, id string) error
	LookupByWebSlug(ctx context.Context, slug string) (*types.GuestLinkChannel, error)
	LookupEnabled(ctx context.Context, id string) (*types.GuestLinkChannel, error)
}
