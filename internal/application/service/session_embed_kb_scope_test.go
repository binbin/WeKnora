package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// denyAllReadableKBService simulates OrgUnit browse denial for every KB id
// (embed Viewer + empty active unit against bound KBs).
type denyAllReadableKBService struct {
	interfaces.KnowledgeBaseService
	listed []*types.KnowledgeBase
}

func (s *denyAllReadableKBService) FilterReadableKnowledgeBaseIDs(
	_ context.Context, _ []string,
) ([]string, error) {
	return []string{}, nil
}

func (s *denyAllReadableKBService) ListKnowledgeBases(
	_ context.Context,
) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *denyAllReadableKBService) ListKnowledgeBasesByTenantID(
	_ context.Context, _ uint64,
) ([]*types.KnowledgeBase, error) {
	return s.listed, nil
}

func TestResolveKnowledgeBases_EmbedKeepsAgentPresetDespiteOrgFilter(t *testing.T) {
	svc := &sessionService{
		knowledgeBaseService: &denyAllReadableKBService{},
	}
	ctx := types.WithPrincipal(
		context.Background(),
		types.EmbedSessionPrincipal(10000, "ch-1", "sess-1"),
	)

	kbIDs, knowledgeIDs, err := svc.resolveKnowledgeBases(ctx, &types.QARequest{
		Session: &types.Session{TenantID: 10000},
		CustomAgent: &types.CustomAgent{
			TenantID: 10000,
			Config: types.CustomAgentConfig{
				KBSelectionMode: "selected",
				KnowledgeBases:  []string{"kb-bound"},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveKnowledgeBases returned error: %v", err)
	}
	if len(kbIDs) != 1 || kbIDs[0] != "kb-bound" {
		t.Fatalf("kbIDs = %#v, want agent-preset kb-bound for embed", kbIDs)
	}
	if len(knowledgeIDs) != 0 {
		t.Fatalf("knowledgeIDs = %#v, want empty", knowledgeIDs)
	}
}

func TestResolveKnowledgeBases_WorkspaceStillAppliesOrgFilterOnAgentPreset(t *testing.T) {
	svc := &sessionService{
		knowledgeBaseService: &denyAllReadableKBService{},
	}
	ctx := types.WithPrincipal(context.Background(), types.Principal{
		Type: types.PrincipalWebUser,
		ID:   "user-1",
	})

	kbIDs, _, err := svc.resolveKnowledgeBases(ctx, &types.QARequest{
		Session: &types.Session{TenantID: 10000},
		CustomAgent: &types.CustomAgent{
			TenantID: 10000,
			Config: types.CustomAgentConfig{
				KBSelectionMode: "selected",
				KnowledgeBases:  []string{"kb-bound"},
			},
		},
	})
	if err != nil {
		t.Fatalf("resolveKnowledgeBases returned error: %v", err)
	}
	if len(kbIDs) != 0 {
		t.Fatalf("kbIDs = %#v, want empty after workspace org filter", kbIDs)
	}
}

func TestResolveKnowledgeBasesFromAgent_EmbedAllUsesTenantList(t *testing.T) {
	svc := &sessionService{
		knowledgeBaseService: &denyAllReadableKBService{
			listed: []*types.KnowledgeBase{
				{ID: "kb-a", TenantID: 10000},
				{ID: "kb-b", TenantID: 10000},
			},
		},
	}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(10000))
	ctx = types.WithPrincipal(
		ctx,
		types.EmbedSessionPrincipal(10000, "ch-1", "sess-1"),
	)
	agent := &types.CustomAgent{
		TenantID: 10000,
		Config: types.CustomAgentConfig{
			KBSelectionMode: "all",
			AgentMode:       "normal",
		},
	}

	kbIDs := svc.resolveKnowledgeBasesFromAgent(ctx, agent, 10000)
	if len(kbIDs) != 2 {
		t.Fatalf("kbIDs = %#v, want both tenant KBs for embed all-mode", kbIDs)
	}
}

func TestShouldSkipCallerOrgKBBrowseFilter(t *testing.T) {
	if shouldSkipCallerOrgKBBrowseFilter(context.Background()) {
		t.Fatal("empty ctx should not skip")
	}
	web := types.WithPrincipal(context.Background(), types.Principal{
		Type: types.PrincipalWebUser, ID: "u1",
	})
	if shouldSkipCallerOrgKBBrowseFilter(web) {
		t.Fatal("web user should not skip")
	}
	embed := types.WithPrincipal(
		context.Background(),
		types.EmbedSessionPrincipal(1, "ch", "sess"),
	)
	if !shouldSkipCallerOrgKBBrowseFilter(embed) {
		t.Fatal("embed session should skip")
	}
	im := types.WithPrincipal(context.Background(), types.Principal{
		Type: types.PrincipalIMUser, ID: "im:1",
	})
	if !shouldSkipCallerOrgKBBrowseFilter(im) {
		t.Fatal("IM user should skip")
	}
}
