package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// guestLinkWebSlugBytes mirrors embedWebSlugBytes: ~8 chars base64url, short
// enough for /w/:slug links.
const guestLinkWebSlugBytes = 6

// guestLinkSessionSecretBytes matches embedTokenBytes: the secret keys the
// HMAC binding chat sessions to the channel (see SignEmbedSessionHandle).
const guestLinkSessionSecretBytes = 32

var (
	ErrGuestLinkExists   = errors.New("guest link already exists for agent")
	ErrGuestLinkDisabled = errors.New("guest link is disabled")
	ErrGuestLinkNotFound = errors.New("guest link not found")
)

type guestLinkChannelService struct {
	repo         interfaces.GuestLinkChannelRepository
	agentService interfaces.CustomAgentService
}

// NewGuestLinkChannelService constructs the guest link channel service.
func NewGuestLinkChannelService(
	repo interfaces.GuestLinkChannelRepository,
	agentService interfaces.CustomAgentService,
) interfaces.GuestLinkChannelService {
	return &guestLinkChannelService{repo: repo, agentService: agentService}
}

func generateGuestLinkWebSlug() (string, error) {
	buf := make([]byte, guestLinkWebSlugBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateGuestLinkSessionSecret() (string, error) {
	buf := make([]byte, guestLinkSessionSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "gls_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

// ensureAgentOwned mirrors embedChannelService.ensureAgentOwned so a foreign
// or mistyped agent id can never get a guest link attached to it.
func (s *guestLinkChannelService) ensureAgentOwned(
	ctx context.Context, tenantID uint64, agentID string,
) error {
	if agentID == "" {
		return apperrors.NewBadRequestError("agent_id is required")
	}
	agent, err := s.agentService.GetAgentByID(ctx, agentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.TenantID != tenantID {
		return apperrors.NewNotFoundError("agent not found")
	}
	return nil
}

// allocateWebSlug retries slug generation up to 8 times to dodge collisions,
// matching the embed channel service's allocateWebSlug (YAGNI: copied rather
// than extracted into a shared helper).
func (s *guestLinkChannelService) allocateWebSlug(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		slug, err := generateGuestLinkWebSlug()
		if err != nil {
			return "", err
		}
		existing, err := s.repo.GetByWebSlug(ctx, slug)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return slug, nil
		}
	}
	return "", fmt.Errorf("failed to allocate unique web slug")
}

func (s *guestLinkChannelService) GetByAgent(
	ctx context.Context, tenantID uint64, agentID string,
) (*types.GuestLinkChannel, error) {
	agentID = strings.TrimSpace(agentID)
	if err := s.ensureAgentOwned(ctx, tenantID, agentID); err != nil {
		return nil, err
	}
	return s.repo.GetByAgent(ctx, tenantID, agentID)
}

// Get returns a tenant-owned guest link by ID for admin management, without
// the Enabled check LookupEnabled applies for the public bootstrap flow.
func (s *guestLinkChannelService) Get(
	ctx context.Context, tenantID uint64, id string,
) (*types.GuestLinkChannel, error) {
	return s.getOwned(ctx, tenantID, id)
}

func (s *guestLinkChannelService) Create(
	ctx context.Context, tenantID uint64, agentID string, req *types.GuestLinkChannel,
) (*types.GuestLinkChannel, error) {
	agentID = strings.TrimSpace(agentID)
	if types.IsBuiltinAgentID(agentID) {
		return nil, apperrors.NewBadRequestError("built-in agents cannot be used for guest links")
	}
	if err := s.ensureAgentOwned(ctx, tenantID, agentID); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrGuestLinkExists
	}
	slug, err := s.allocateWebSlug(ctx)
	if err != nil {
		return nil, err
	}
	secret, err := generateGuestLinkSessionSecret()
	if err != nil {
		return nil, err
	}
	ch := &types.GuestLinkChannel{
		TenantID:               tenantID,
		AgentID:                agentID,
		Name:                   strings.TrimSpace(req.Name),
		Enabled:                req.Enabled,
		WebSlug:                slug,
		SessionSecret:          secret,
		WelcomeMessage:         req.WelcomeMessage,
		RateLimitPerMinute:     req.RateLimitPerMinute,
		RateLimitPerDay:        req.RateLimitPerDay,
		PrimaryColor:           strings.TrimSpace(req.PrimaryColor),
		PageTitle:              strings.TrimSpace(req.PageTitle),
		HeaderTitleMode:        types.NormalizeEmbedHeaderTitleMode(req.HeaderTitleMode),
		ShowSuggestedQuestions: req.ShowSuggestedQuestions,
		AllowWebSearch:         req.AllowWebSearch,
		AllowFileUpload:        req.AllowFileUpload,
		DefaultLocale:          types.NormalizeEmbedDefaultLocale(req.DefaultLocale),
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// Update replaces every string field on req (the admin form always submits the
// whole configuration, so an omitted string means "clear it"), while the
// booleans are tri-state pointers — nil keeps the stored value, matching
// embedChannelService.Update. Rate limits keep their stored value when
// non-positive, since 0 is not a usable limit.
func (s *guestLinkChannelService) Update(
	ctx context.Context, tenantID uint64, id string, req *types.GuestLinkChannel,
	enabled, showSuggested, allowWebSearch, allowFileUpload *bool,
) (*types.GuestLinkChannel, error) {
	ch, err := s.getOwned(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	ch.Name = strings.TrimSpace(req.Name)
	ch.WelcomeMessage = req.WelcomeMessage
	ch.PrimaryColor = strings.TrimSpace(req.PrimaryColor)
	ch.PageTitle = strings.TrimSpace(req.PageTitle)
	ch.HeaderTitleMode = types.NormalizeEmbedHeaderTitleMode(req.HeaderTitleMode)
	ch.DefaultLocale = types.NormalizeEmbedDefaultLocale(req.DefaultLocale)
	if showSuggested != nil {
		ch.ShowSuggestedQuestions = *showSuggested
	}
	if allowWebSearch != nil {
		ch.AllowWebSearch = *allowWebSearch
	}
	if allowFileUpload != nil {
		ch.AllowFileUpload = *allowFileUpload
	}
	if enabled != nil {
		ch.Enabled = *enabled
	}
	if req.RateLimitPerMinute > 0 {
		ch.RateLimitPerMinute = req.RateLimitPerMinute
	}
	if req.RateLimitPerDay > 0 {
		ch.RateLimitPerDay = req.RateLimitPerDay
	}
	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *guestLinkChannelService) Delete(ctx context.Context, tenantID uint64, id string) error {
	if _, err := s.getOwned(ctx, tenantID, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, tenantID, id)
}

// LookupByWebSlug resolves a guest link for the public /w/:slug surface.
func (s *guestLinkChannelService) LookupByWebSlug(
	ctx context.Context, slug string,
) (*types.GuestLinkChannel, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrGuestLinkNotFound
	}
	ch, err := s.repo.GetByWebSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, ErrGuestLinkNotFound
	}
	if !ch.Enabled {
		return nil, ErrGuestLinkDisabled
	}
	return ch, nil
}

// LookupEnabled resolves a guest link by ID, e.g. for admin preview/session flows.
func (s *guestLinkChannelService) LookupEnabled(
	ctx context.Context, id string,
) (*types.GuestLinkChannel, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrGuestLinkNotFound
	}
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, ErrGuestLinkNotFound
	}
	if !ch.Enabled {
		return nil, ErrGuestLinkDisabled
	}
	return ch, nil
}

func (s *guestLinkChannelService) getOwned(
	ctx context.Context, tenantID uint64, id string,
) (*types.GuestLinkChannel, error) {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ch == nil || ch.TenantID != tenantID {
		return nil, ErrGuestLinkNotFound
	}
	return ch, nil
}
