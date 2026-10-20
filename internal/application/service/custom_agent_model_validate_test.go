package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestValidateKnowledgeQAModelAcceptsTenantOwnedChatModel(t *testing.T) {
	svc := &customAgentService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"tenant-chat-1": {
					ID:        "tenant-chat-1",
					Type:      types.ModelTypeKnowledgeQA,
					IsBuiltin: false,
					Status:    types.ModelStatusActive,
				},
			},
		},
	}
	err := svc.validateKnowledgeQAModel(context.Background(), "tenant-chat-1")
	require.NoError(t, err)
}

func TestValidateKnowledgeQAModelRejectsWrongType(t *testing.T) {
	svc := &customAgentService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"embed-1": {
					ID:     "embed-1",
					Type:   types.ModelTypeEmbedding,
					Status: types.ModelStatusActive,
				},
			},
		},
	}
	err := svc.validateKnowledgeQAModel(context.Background(), "embed-1")
	require.ErrorIs(t, err, ErrKnowledgeQAModelRequired)
}

func TestValidateKnowledgeQAModelRejectsEmptyID(t *testing.T) {
	svc := &customAgentService{modelService: &stubModelService{}}
	err := svc.validateKnowledgeQAModel(context.Background(), "  ")
	require.ErrorIs(t, err, ErrKnowledgeQAModelRequired)
}
