package agent

import (
	"strings"

	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/searchutil"
	"github.com/Tencent/WeKnora/internal/types"
)

const agentRetrievedImageRequirementMarker = "## 检索图片输出要求"

const agentRetrievedImageSystemRequirement = `

## 检索图片输出要求
本轮工具结果包含 Markdown 图片。默认将检索段落附带的图片视为相关。
- 除非用户明确要求纯文本输出，或所有检索图片都明显与答案无关，最终回答必须至少包含一张从工具结果原样复制的相关 Markdown 图片。
- 完整保留 Markdown 图片语法及其 URL，绝不要编造、缩短、规范化或替换 URL。
- 必须使用 ASCII 半角括号，格式为 ![alt](url)；绝不要使用全角（或）。
- 将每张图片紧挨放在其所支撑段落之后。
- 当多张检索图片分别支撑不同段落时，应在对应段落中分别插入，不要只插第一张就结束。
- 结束前请静默检查：只要本要求适用，答案中就应包含 Markdown 图片。`

func stepContainsMarkdownImage(step types.AgentStep) bool {
	for _, toolCall := range step.ToolCalls {
		if toolCall.Result != nil &&
			toolCall.Result.Success &&
			searchutil.MarkdownImageRegex.MatchString(toolCall.Result.Output) {
			return true
		}
	}
	return false
}

func appendAgentRetrievedImageRequirement(messages []chat.Message) []chat.Message {
	for i := range messages {
		if messages[i].Role != "system" {
			continue
		}
		if !strings.Contains(messages[i].Content, agentRetrievedImageRequirementMarker) {
			messages[i].Content = strings.TrimRight(messages[i].Content, " \t\r\n") + agentRetrievedImageSystemRequirement
		}
		break
	}
	return messages
}
