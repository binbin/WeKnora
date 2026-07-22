package agent

import (
	"strings"
	"testing"
)

func TestFinalAnswerImageRequirement(t *testing.T) {
	if got := finalAnswerImageRequirement(false); got != "" {
		t.Fatalf("text-only tool results should not add an image requirement: %q", got)
	}

	got := finalAnswerImageRequirement(true)
	for _, required := range []string{
		"最终回答必须至少包含一张从工具结果原样复制的相关 Markdown 图片",
		"完整保留其 URL",
		"ASCII 半角括号",
		"静默检查",
	} {
		if !strings.Contains(got, required) {
			t.Fatalf("expected %q in final-answer image requirement:\n%s", required, got)
		}
	}
}
