package handler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// openAIChatMessage is one item in an OpenAI-compatible messages array.
type openAIChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// lastUserQuery returns the text of the last user message in messages.
// P0: string content, or multimodal arrays with text parts concatenated.
func lastUserQuery(messages []openAIChatMessage) (string, error) {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role != "user" {
			continue
		}
		text, err := openAIMessageContentText(messages[index].Content)
		if err != nil {
			return "", err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return "", fmt.Errorf("empty user message content")
		}
		return text, nil
	}
	return "", fmt.Errorf("no user message found")
}

func openAIMessageContentText(content any) (string, error) {
	switch value := content.(type) {
	case nil:
		return "", fmt.Errorf("message content is required")
	case string:
		return value, nil
	case json.Number:
		return value.String(), nil
	case []any:
		return concatOpenAITextParts(value)
	case []map[string]any:
		parts := make([]any, 0, len(value))
		for _, part := range value {
			parts = append(parts, part)
		}
		return concatOpenAITextParts(parts)
	default:
		return "", fmt.Errorf("unsupported message content type")
	}
}

func concatOpenAITextParts(parts []any) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("message content is required")
	}
	var builder strings.Builder
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok {
			return "", fmt.Errorf("unsupported message content part")
		}
		partType, _ := partMap["type"].(string)
		if partType != "" && partType != "text" {
			return "", fmt.Errorf(
				"unsupported multimodal content type %q", partType,
			)
		}
		text, _ := partMap["text"].(string)
		builder.WriteString(text)
	}
	return builder.String(), nil
}
