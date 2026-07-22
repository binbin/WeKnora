package chatpipeline

import (
	"strings"
	"testing"
)

func TestAppendRetrievedImageOutputRequirement(t *testing.T) {
	base := "Answer from retrieved evidence."
	withImage := appendRetrievedImageOutputRequirement(
		base,
		"context\n![流程图](resource://AbCdEfGhIjKlMnOpQrStUv)",
	)
	for _, required := range []string{
		base,
		"最终回答必须至少包含一张从检索上下文原样复制的相关 Markdown 图片",
		"完整复制 Markdown 图片语法及其 URL",
		"ASCII 半角括号",
		"紧挨放在其所支撑段落之后",
	} {
		if !strings.Contains(withImage, required) {
			t.Fatalf("expected %q in dynamic image requirement:\n%s", required, withImage)
		}
	}

	withoutImage := appendRetrievedImageOutputRequirement(base, "text-only retrieved context")
	if withoutImage != base {
		t.Fatalf("text-only context should not change the system prompt: %q", withoutImage)
	}
}
