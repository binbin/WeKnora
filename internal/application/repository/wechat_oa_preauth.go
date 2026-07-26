package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// WeChatOAPreauthRepository persists OA pre-auth sessions.
type WeChatOAPreauthRepository interface {
	Create(ctx context.Context, row *types.WeChatOAPreauth) error
	GetByID(ctx context.Context, id string) (*types.WeChatOAPreauth, error)
	GetByState(ctx context.Context, state string) (*types.WeChatOAPreauth, error)
	Update(ctx context.Context, row *types.WeChatOAPreauth) error
}

type wechatOAPreauthRepository struct {
	db *gorm.DB
}

// NewWeChatOAPreauthRepository constructs the repository.
func NewWeChatOAPreauthRepository(db *gorm.DB) WeChatOAPreauthRepository {
	return &wechatOAPreauthRepository{db: db}
}

func (repo *wechatOAPreauthRepository) Create(
	ctx context.Context,
	row *types.WeChatOAPreauth,
) error {
	return repo.db.WithContext(ctx).Create(row).Error
}

func (repo *wechatOAPreauthRepository) GetByID(
	ctx context.Context,
	id string,
) (*types.WeChatOAPreauth, error) {
	var row types.WeChatOAPreauth
	err := repo.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (repo *wechatOAPreauthRepository) GetByState(
	ctx context.Context,
	state string,
) (*types.WeChatOAPreauth, error) {
	var row types.WeChatOAPreauth
	err := repo.db.WithContext(ctx).Where("state = ?", state).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (repo *wechatOAPreauthRepository) Update(
	ctx context.Context,
	row *types.WeChatOAPreauth,
) error {
	return repo.db.WithContext(ctx).Save(row).Error
}
