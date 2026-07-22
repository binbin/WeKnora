package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

var sequentialThinkingTool = BaseTool{
	name: ToolThinking,
	description: `通过逐步思考进行动态、反思式问题解决的详细工具。

本工具帮助以灵活、可演进的思考过程分析问题。每个想法可建立在先前洞见之上，也可质疑或修正先前洞见。

## 何时使用
- 将复杂问题拆成步骤
- 需要预留修正空间的规划与设计
- 可能需要中途纠偏的分析
- 一开始范围不完全清晰的问题
- 需要多步求解的问题
- 需在多步间保持上下文的任务
- 需要过滤无关信息的情形

## 关键特性
- 可随进展调高或调低 total_thoughts
- 可质疑或修正先前想法
- 看似结束后仍可继续添加想法
- 可表达不确定性并探索替代路径
- 不必线性推进——可分支或回溯
- 生成解决方案假设，并基于思维链步骤验证
- 重复直到满意
- 思考完成后，以纯文本回复给出答案并停止（不再调用工具）。绝不要把最终答案直接写进 thought。

## 参数说明
- **thought**：当前思考步骤（分析、修正、提问、改换思路、假设与验证等）
  **关键——对用户友好的思考**：用自然、易懂的语言写思考。思考过程中绝不要提及工具名（如 "grep_chunks"、"knowledge_search"、"web_search"）。改用普通描述：
  - ❌ 差："我会用 grep_chunks 搜关键词，再用 knowledge_search 做语义理解"
  - ✅ 好："我会先在知识库中搜索关键词，再探索相关概念"
  像向用户解释推理那样写思考，聚焦要找什么、为什么，而不是用哪个工具。
- **next_thought_needed**：是否还需要继续思考
- **thought_number** / **total_thoughts**：当前序号与预估总数（可调整）
- **is_revision** / **revises_thought**：是否修正先前想法及对应序号
- **branch_from_thought** / **branch_id**：分支点与分支标识
- **needs_more_thoughts**：到达终点时是否仍需更多思考

## 最佳实践
1. 先估计所需想法数，并随时调整
2. 勇于质疑或修正先前想法
3. 需要时即使「结束」也可继续加想法
4. 有不确定就明确表达
5. 标记修正或分支的想法
6. 忽略与当前步骤无关的信息
7. 适时生成解决方案假设并验证
8. 重复直到满意
9. 仅在真正完成且答案满意时将 next_thought_needed 设为 false
10. 绝不要把最终答案写进 thought；完成后用纯文本回复并停止`,
	schema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "thought": {
      "type": "string",
      "description": "Your current thinking step. Write in natural, user-friendly language. NEVER mention tool names (like \"grep_chunks\", \"knowledge_search\", \"web_search\", etc.). Instead, describe actions in plain language (e.g., \"I'll search for key terms\" instead of \"I'll use grep_chunks\"). Focus on WHAT you're trying to find and WHY, not HOW (which tools you'll use)."
    },
    "next_thought_needed": {
      "type": "boolean",
      "description": "Whether another thought step is needed"
    },
    "thought_number": {
      "type": "integer",
      "description": "Current thought number (numeric value, e.g., 1, 2, 3)",
      "minimum": 1
    },
    "total_thoughts": {
      "type": "integer",
      "description": "Estimated total thoughts needed (numeric value, e.g., 5, 10)",
      "minimum": 1
    },
    "is_revision": {
      "type": "boolean",
      "description": "Whether this revises previous thinking"
    },
    "revises_thought": {
      "type": "integer",
      "description": "Which thought is being reconsidered",
      "minimum": 1
    },
    "branch_from_thought": {
      "type": "integer",
      "description": "Branching point thought number",
      "minimum": 1
    },
    "branch_id": {
      "type": "string",
      "description": "Branch identifier"
    },
    "needs_more_thoughts": {
      "type": "boolean",
      "description": "If more thoughts are needed"
    }
  },
  "required": ["thought", "next_thought_needed", "thought_number", "total_thoughts"]
}`),
}

// SequentialThinkingTool is a dynamic and reflective problem-solving tool
// This tool helps analyze problems through a flexible thinking process that can adapt and evolve
type SequentialThinkingTool struct {
	BaseTool
	thoughtHistory []SequentialThinkingInput
	branches       map[string][]SequentialThinkingInput
}

// SequentialThinkingInput defines the input parameters for sequential thinking tool
type SequentialThinkingInput struct {
	Thought           string `json:"thought"`
	NextThoughtNeeded bool   `json:"next_thought_needed"`
	ThoughtNumber     int    `json:"thought_number"`
	TotalThoughts     int    `json:"total_thoughts"`
	IsRevision        bool   `json:"is_revision,omitempty"`
	RevisesThought    *int   `json:"revises_thought,omitempty"`
	BranchFromThought *int   `json:"branch_from_thought,omitempty"`
	BranchID          string `json:"branch_id,omitempty"`
	NeedsMoreThoughts bool   `json:"needs_more_thoughts,omitempty"`
}

// NewSequentialThinkingTool creates a new sequential thinking tool instance
func NewSequentialThinkingTool() *SequentialThinkingTool {
	return &SequentialThinkingTool{
		BaseTool:       sequentialThinkingTool,
		thoughtHistory: make([]SequentialThinkingInput, 0),
		branches:       make(map[string][]SequentialThinkingInput),
	}
}

// Execute executes the sequential thinking tool
func (t *SequentialThinkingTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	logger.Infof(ctx, "[Tool][SequentialThinking] Execute started")

	// Parse args from json.RawMessage
	var input SequentialThinkingInput
	if err := json.Unmarshal(args, &input); err != nil {
		logger.Errorf(ctx, "[Tool][SequentialThinking] Failed to parse args: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	// Validate and parse input
	if err := t.validate(input); err != nil {
		logger.Errorf(ctx, "[Tool][SequentialThinking] Validation failed: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Validation failed: %v", err),
		}, err
	}

	// Adjust totalThoughts if thoughtNumber exceeds it
	if input.ThoughtNumber > input.TotalThoughts {
		input.TotalThoughts = input.ThoughtNumber
	}

	// Add to thought history
	t.thoughtHistory = append(t.thoughtHistory, input)

	// Handle branching
	if input.BranchFromThought != nil && input.BranchID != "" {
		if t.branches[input.BranchID] == nil {
			t.branches[input.BranchID] = make([]SequentialThinkingInput, 0)
		}
		t.branches[input.BranchID] = append(t.branches[input.BranchID], input)
	}

	logger.Debugf(ctx, "[Tool][SequentialThinking] %s", input.Thought)

	// Prepare response data
	branchKeys := make([]string, 0, len(t.branches))
	for k := range t.branches {
		branchKeys = append(branchKeys, k)
	}

	incomplete := input.NextThoughtNeeded || input.NeedsMoreThoughts ||
		input.ThoughtNumber < input.TotalThoughts

	responseData := map[string]interface{}{
		"thought_number":         input.ThoughtNumber,
		"total_thoughts":         input.TotalThoughts,
		"next_thought_needed":    input.NextThoughtNeeded,
		"branches":               branchKeys,
		"thought_history_length": len(t.thoughtHistory),
		"display_type":           "thinking",
		"thought":                input.Thought,
		"incomplete_steps":       incomplete,
	}

	logger.Infof(
		ctx,
		"[Tool][SequentialThinking] Execute completed - Thought %d/%d",
		input.ThoughtNumber,
		input.TotalThoughts,
	)

	outputMsg := "Thought process recorded"
	if incomplete {
		outputMsg = "Thought process recorded - unfinished steps remain, continue exploring and calling tools"
	}

	return &types.ToolResult{
		Success: true,
		Output:  outputMsg,
		Data:    responseData,
	}, nil
}

// validate validates the input thought data
func (t *SequentialThinkingTool) validate(data SequentialThinkingInput) error {
	// Validate thought (required)
	if data.Thought == "" {
		return fmt.Errorf("invalid thought: must be a non-empty string")
	}

	// Validate thoughtNumber (required)
	if data.ThoughtNumber < 1 {
		return fmt.Errorf("invalid thoughtNumber: must be >= 1")
	}

	// Validate totalThoughts (required)
	if data.TotalThoughts < 1 {
		return fmt.Errorf("invalid totalThoughts: must be >= 1")
	}

	return nil
}
