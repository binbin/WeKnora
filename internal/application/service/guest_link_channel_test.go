package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubGuestLinkChannelRepo is an in-memory fake implementing
// interfaces.GuestLinkChannelRepository for service-level unit tests.
type stubGuestLinkChannelRepo struct {
	interfaces.GuestLinkChannelRepository
	byAgent map[string]*types.GuestLinkChannel
	bySlug  map[string]*types.GuestLinkChannel
	byID    map[string]*types.GuestLinkChannel
}

func newStubGuestLinkChannelRepo() *stubGuestLinkChannelRepo {
	return &stubGuestLinkChannelRepo{
		byAgent: map[string]*types.GuestLinkChannel{},
		bySlug:  map[string]*types.GuestLinkChannel{},
		byID:    map[string]*types.GuestLinkChannel{},
	}
}

func (r *stubGuestLinkChannelRepo) Create(_ context.Context, ch *types.GuestLinkChannel) error {
	if ch.ID == "" {
		ch.ID = "guest-link-" + ch.AgentID
	}
	cp := *ch
	r.byAgent[agentKey(ch.TenantID, ch.AgentID)] = &cp
	r.bySlug[ch.WebSlug] = &cp
	r.byID[ch.ID] = &cp
	return nil
}

func (r *stubGuestLinkChannelRepo) GetByID(_ context.Context, id string) (*types.GuestLinkChannel, error) {
	ch, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *ch
	return &cp, nil
}

func (r *stubGuestLinkChannelRepo) GetByWebSlug(_ context.Context, slug string) (*types.GuestLinkChannel, error) {
	ch, ok := r.bySlug[slug]
	if !ok {
		return nil, nil
	}
	cp := *ch
	return &cp, nil
}

func (r *stubGuestLinkChannelRepo) GetByAgent(
	_ context.Context, tenantID uint64, agentID string,
) (*types.GuestLinkChannel, error) {
	ch, ok := r.byAgent[agentKey(tenantID, agentID)]
	if !ok {
		return nil, nil
	}
	cp := *ch
	return &cp, nil
}

func (r *stubGuestLinkChannelRepo) Update(_ context.Context, ch *types.GuestLinkChannel) error {
	cp := *ch
	r.byAgent[agentKey(ch.TenantID, ch.AgentID)] = &cp
	r.bySlug[ch.WebSlug] = &cp
	r.byID[ch.ID] = &cp
	return nil
}

func agentKey(tenantID uint64, agentID string) string {
	return fmt.Sprintf("%d:%s", tenantID, agentID)
}

// newGuestLinkServiceForTenant builds the service with an agent service that
// only owns agents of tenantID, so ownership checks behave like production.
func newGuestLinkServiceForTenant(
	repo interfaces.GuestLinkChannelRepository, tenantID uint64,
) interfaces.GuestLinkChannelService {
	return NewGuestLinkChannelService(repo, &stubAgentForEmbed{
		agent: &types.CustomAgent{TenantID: tenantID, Name: "客服助手"},
	})
}

func TestGuestLinkCreateRejectsSecondForSameAgent(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	_, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{Name: "First"})
	if err != nil {
		t.Fatalf("first Create() error = %v, want nil", err)
	}

	_, err = svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{Name: "Second"})
	if !errors.Is(err, ErrGuestLinkExists) {
		t.Fatalf("second Create() error = %v, want ErrGuestLinkExists", err)
	}
}

func TestGuestLinkCreateAllocatesSlug(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	created, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{Name: "First"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.WebSlug == "" {
		t.Fatal("created.WebSlug is empty, want a generated slug")
	}
	if len(created.WebSlug) > 16 {
		t.Fatalf("created.WebSlug length = %d, want <= 16", len(created.WebSlug))
	}
}

func TestGuestLinkCreateDefaultsTitleToAgentName(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	created, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Name != "客服助手" {
		t.Fatalf("Name = %q, want agent name", created.Name)
	}
	if created.PageTitle != "客服助手" {
		t.Fatalf("PageTitle = %q, want agent name", created.PageTitle)
	}
}

func TestGuestLinkCreateKeepsExplicitTitle(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	created, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{
		Name:      "官网客服",
		PageTitle: "在线咨询",
	})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Name != "官网客服" || created.PageTitle != "在线咨询" {
		t.Fatalf("got name=%q page_title=%q", created.Name, created.PageTitle)
	}
}

// TestGuestLinkCreateSignsSessionHandles guards the forgeable-handle
// regression: without a per-channel secret the HMAC key was "", so anyone
// knowing channel id + session id could mint a valid handle.
func TestGuestLinkCreateSignsSessionHandles(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	created, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{Name: "First"})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.SessionSecret == "" {
		t.Fatal("created.SessionSecret is empty, want a generated secret")
	}

	const sessionID = "11111111-2222-3333-4444-555555555555"
	ch := created.AsEmbedChannel()
	sig := SignEmbedSessionHandle(ch, sessionID)
	if sig == "" {
		t.Fatal("guest link handles must be signable")
	}
	if !VerifyEmbedSessionHandle(ch, sessionID, sig) {
		t.Fatal("guest link handle should verify against its own channel")
	}

	// A forger who knows channel id + session id but not the secret (the
	// pre-fix state: empty HMAC key) must not produce a valid handle.
	unkeyed := created.AsEmbedChannel()
	unkeyed.PublishToken = ""
	if forged := SignEmbedSessionHandle(unkeyed, sessionID); forged != "" {
		t.Fatalf("signing with an empty key must fail closed, got %q", forged)
	}
	if VerifyEmbedSessionHandle(unkeyed, sessionID, sig) {
		t.Fatal("handle must not verify without the channel secret")
	}
}

func TestGuestLinkCreateSecretsAreUniquePerChannel(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	first, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{})
	if err != nil {
		t.Fatalf("Create(agent-1) error = %v, want nil", err)
	}
	second, err := svc.Create(context.Background(), 1, "agent-2", &types.GuestLinkChannel{})
	if err != nil {
		t.Fatalf("Create(agent-2) error = %v, want nil", err)
	}
	if first.SessionSecret == second.SessionSecret {
		t.Fatal("each guest link must get its own session secret")
	}
}

func TestGuestLinkCreateRejectsForeignAgent(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	// The agent service only ever resolves agents owned by tenant 1.
	svc := newGuestLinkServiceForTenant(repo, 1)

	_, err := svc.Create(context.Background(), 2, "agent-1", &types.GuestLinkChannel{})
	if err == nil {
		t.Fatal("Create() with an agent from another tenant must fail")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.HTTPCode != http.StatusNotFound {
		t.Fatalf("Create() error = %v, want a 404 AppError", err)
	}
}

func TestGuestLinkCreateRejectsUnknownAgent(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := NewGuestLinkChannelService(repo, &stubAgentForEmbed{agent: nil})

	if _, err := svc.Create(context.Background(), 1, "missing-agent", &types.GuestLinkChannel{}); err == nil {
		t.Fatal("Create() with an unknown agent must fail")
	}
}

func TestGuestLinkUpdateKeepsFlagsWhenNil(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	created, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{
		Name: "First", Enabled: true, ShowSuggestedQuestions: true, AllowWebSearch: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	updated, err := svc.Update(
		context.Background(), 1, created.ID,
		&types.GuestLinkChannel{Name: "Renamed"}, nil, nil, nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Name != "Renamed" {
		t.Fatalf("updated.Name = %q, want Renamed", updated.Name)
	}
	if !updated.ShowSuggestedQuestions || !updated.AllowWebSearch || !updated.Enabled {
		t.Fatalf("nil flags must keep stored values, got %#v", updated)
	}
}

func TestGuestLinkUpdateAllowsUnlimitedRateLimits(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	created, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{
		Name: "First", RateLimitPerMinute: 30, RateLimitPerDay: 10000,
	})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	zero := 0
	updated, err := svc.Update(
		context.Background(), 1, created.ID,
		&types.GuestLinkChannel{Name: created.Name},
		nil, nil, nil, nil, &zero, &zero,
	)
	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.RateLimitPerMinute != 0 || updated.RateLimitPerDay != 0 {
		t.Fatalf(
			"rate limits = %d/%d, want 0/0 (unlimited)",
			updated.RateLimitPerMinute, updated.RateLimitPerDay,
		)
	}

	// Nil rate-limit pointers must keep the unlimited values.
	kept, err := svc.Update(
		context.Background(), 1, created.ID,
		&types.GuestLinkChannel{Name: "Still unlimited"},
		nil, nil, nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if kept.RateLimitPerMinute != 0 || kept.RateLimitPerDay != 0 {
		t.Fatalf(
			"nil rate limits must keep 0/0, got %d/%d",
			kept.RateLimitPerMinute, kept.RateLimitPerDay,
		)
	}
}

func TestGuestLinkLookupByWebSlugNotFound(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	_, err := svc.LookupByWebSlug(context.Background(), "missing")
	if !errors.Is(err, ErrGuestLinkNotFound) {
		t.Fatalf("LookupByWebSlug() error = %v, want ErrGuestLinkNotFound", err)
	}
}

func TestGuestLinkLookupByWebSlugDisabled(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := newGuestLinkServiceForTenant(repo, 1)

	created, err := svc.Create(context.Background(), 1, "agent-1", &types.GuestLinkChannel{
		Name: "First", Enabled: false,
	})
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	_, err = svc.LookupByWebSlug(context.Background(), created.WebSlug)
	if !errors.Is(err, ErrGuestLinkDisabled) {
		t.Fatalf("LookupByWebSlug() error = %v, want ErrGuestLinkDisabled", err)
	}
}
