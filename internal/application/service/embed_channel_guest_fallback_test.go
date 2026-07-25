package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// TestLookupEnabledChannelFallsBackToGuestLink verifies the dual-lookup used
// by EmbedAuth: when the embed_channels table misses on channelID, an
// enabled guest_link_channels row with the same ID resolves via
// AsEmbedChannel(), keeping ID/AgentID stable for the /api/v1/embed/{id}/...
// surface.
func TestLookupEnabledChannelFallsBackToGuestLink(t *testing.T) {
	embedRepo := &stubEmbedChannelRepo{} // empty: GetByID(any) -> nil, nil (miss)
	guestRepo := newStubGuestLinkChannelRepo()
	gl := &types.GuestLinkChannel{
		ID:       "shared-channel-id",
		TenantID: 7,
		AgentID:  "agent-1",
		Name:     "Guest Link",
		Enabled:  true,
	}
	if err := guestRepo.Create(context.Background(), gl); err != nil {
		t.Fatalf("guestRepo.Create() error = %v, want nil", err)
	}

	svc := &embedChannelService{repo: embedRepo, guestLinkRepo: guestRepo}

	ch, err := svc.LookupEnabledChannel(context.Background(), gl.ID)
	if err != nil {
		t.Fatalf("LookupEnabledChannel() error = %v, want nil", err)
	}
	if ch == nil {
		t.Fatal("LookupEnabledChannel() = nil, want the guest link mapped to an EmbedChannel")
	}
	if ch.ID != gl.ID {
		t.Fatalf("ch.ID = %q, want %q", ch.ID, gl.ID)
	}
	if ch.AgentID != gl.AgentID {
		t.Fatalf("ch.AgentID = %q, want %q", ch.AgentID, gl.AgentID)
	}
}

// TestLookupEnabledChannelGuestLinkDisabled verifies a disabled guest link
// still blocks access, matching embed channel disabled semantics.
func TestLookupEnabledChannelGuestLinkDisabled(t *testing.T) {
	embedRepo := &stubEmbedChannelRepo{}
	guestRepo := newStubGuestLinkChannelRepo()
	gl := &types.GuestLinkChannel{
		ID:       "disabled-channel-id",
		TenantID: 7,
		AgentID:  "agent-1",
		Enabled:  false,
	}
	if err := guestRepo.Create(context.Background(), gl); err != nil {
		t.Fatalf("guestRepo.Create() error = %v, want nil", err)
	}

	svc := &embedChannelService{repo: embedRepo, guestLinkRepo: guestRepo}

	if _, err := svc.LookupEnabledChannel(context.Background(), gl.ID); err != ErrEmbedChannelDisabled {
		t.Fatalf("LookupEnabledChannel() error = %v, want ErrEmbedChannelDisabled", err)
	}
}

// TestLookupEnabledChannelNoGuestFallback verifies old callers (nil
// guestLinkRepo, e.g. pre-existing test fakes) keep the original not-found
// behavior instead of silently swallowing the miss.
func TestLookupEnabledChannelNoGuestFallback(t *testing.T) {
	svc := &embedChannelService{repo: &stubEmbedChannelRepo{}}

	if _, err := svc.LookupEnabledChannel(context.Background(), "missing-id"); err != ErrEmbedTokenInvalid {
		t.Fatalf("LookupEnabledChannel() error = %v, want ErrEmbedTokenInvalid", err)
	}
}
