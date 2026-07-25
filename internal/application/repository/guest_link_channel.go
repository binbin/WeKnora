package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type guestLinkChannelRepository struct {
	db *gorm.DB
}

func NewGuestLinkChannelRepository(db *gorm.DB) interfaces.GuestLinkChannelRepository {
	return &guestLinkChannelRepository{db: db}
}

func (r *guestLinkChannelRepository) Create(ctx context.Context, ch *types.GuestLinkChannel) error {
	return r.db.WithContext(ctx).Create(ch).Error
}

func (r *guestLinkChannelRepository) GetByID(ctx context.Context, id string) (*types.GuestLinkChannel, error) {
	var ch types.GuestLinkChannel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *guestLinkChannelRepository) GetByWebSlug(
	ctx context.Context, slug string,
) (*types.GuestLinkChannel, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, nil
	}
	var ch types.GuestLinkChannel
	err := r.db.WithContext(ctx).Where("web_slug = ?", slug).First(&ch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *guestLinkChannelRepository) GetByAgent(
	ctx context.Context, tenantID uint64, agentID string,
) (*types.GuestLinkChannel, error) {
	var ch types.GuestLinkChannel
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND agent_id = ?", tenantID, agentID).
		First(&ch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *guestLinkChannelRepository) Update(ctx context.Context, ch *types.GuestLinkChannel) error {
	return r.db.WithContext(ctx).Save(ch).Error
}

func (r *guestLinkChannelRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&types.GuestLinkChannel{}).Error
}
