package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveCompletionTokenLimit(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, EffectiveCompletionTokenLimit(nil))
	assert.Equal(t, 0, EffectiveCompletionTokenLimit(&ChatOptions{}))
	assert.Equal(t, 128, EffectiveCompletionTokenLimit(&ChatOptions{MaxTokens: 128}))
	assert.Equal(t, 4096, EffectiveCompletionTokenLimit(&ChatOptions{
		MaxCompletionTokens: 4096,
	}))
	assert.Equal(t, 4096, EffectiveCompletionTokenLimit(&ChatOptions{
		MaxTokens:           128,
		MaxCompletionTokens: 4096,
	}))
}
