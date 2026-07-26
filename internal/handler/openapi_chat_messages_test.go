package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLastUserMessage(t *testing.T) {
	query, err := lastUserQuery([]openAIChatMessage{
		{Role: "system", Content: "x"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "second"},
	})
	require.NoError(t, err)
	require.Equal(t, "second", query)
}

func TestLastUserMessageMissing(t *testing.T) {
	_, err := lastUserQuery(nil)
	require.Error(t, err)
}

func TestLastUserMessageMultimodalTextParts(t *testing.T) {
	query, err := lastUserQuery([]openAIChatMessage{
		{
			Role: "user",
			Content: []any{
				map[string]any{"type": "text", "text": "hello "},
				map[string]any{"type": "text", "text": "world"},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "hello world", query)
}

func TestLastUserMessageRejectsImagePart(t *testing.T) {
	_, err := lastUserQuery([]openAIChatMessage{
		{
			Role: "user",
			Content: []any{
				map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url": "https://example.com/a.png",
					},
				},
			},
		},
	})
	require.Error(t, err)
}
