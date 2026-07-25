package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

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

func agentKey(tenantID uint64, agentID string) string {
	return fmt.Sprintf("%d:%s", tenantID, agentID)
}

func TestGuestLinkCreateRejectsSecondForSameAgent(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := NewGuestLinkChannelService(repo)

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
	svc := NewGuestLinkChannelService(repo)

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

func TestGuestLinkLookupByWebSlugNotFound(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := NewGuestLinkChannelService(repo)

	_, err := svc.LookupByWebSlug(context.Background(), "missing")
	if !errors.Is(err, ErrGuestLinkNotFound) {
		t.Fatalf("LookupByWebSlug() error = %v, want ErrGuestLinkNotFound", err)
	}
}

func TestGuestLinkLookupByWebSlugDisabled(t *testing.T) {
	repo := newStubGuestLinkChannelRepo()
	svc := NewGuestLinkChannelService(repo)

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
