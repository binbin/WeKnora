package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
)

var todoWriteTool = BaseTool{
	name: ToolTodoWrite,
	description: `使用本工具为检索与研究任务创建并管理结构化任务列表。有助于跟踪进度、组织复杂检索，并向用户展示工作的完整性。

**关键——只关注检索任务**：
- 本工具用于跟踪「检索」与「研究」任务（如搜索知识库、获取文档、收集信息）
- 不要把摘要或综合任务放进 todo_write——那些由 thinking 工具处理
- 合适任务示例：「在知识库中搜索 X」「检索关于 Y 的信息」「比较 A 与 B」
- 应排除的任务示例：「总结发现」「生成最终答案」「综合结果」——这些交给 thinking 工具

## 何时使用
在以下场景主动使用：
1. 复杂多步任务——需要 3 个或以上不同步骤
2. 非平凡、复杂任务——需要仔细规划或多次操作
3. 用户明确要求待办列表
4. 用户提供多项任务（编号或逗号分隔）
5. 收到新指令后——立即把需求记为待办
6. 开始某任务时——开始前先标为 in_progress（理想情况下同时只有一个 in_progress）
7. 完成某任务后——标为 completed，并加入实施中发现的后续任务

## 何时不要使用
1. 只有一个简单直接的任务
2. 任务过于琐碎，跟踪无组织收益
3. 纯对话或纯信息问答

若只有一个琐碎任务，直接做即可，不必使用本工具。

## 任务状态与管理
1. **状态**：pending（未开始）/ in_progress（进行中，同时限一个）/ completed（已完成）
2. **管理**：实时更新；完成后立即标记；同时仅一个 in_progress；完成当前再开新任务；删除不再相关的任务
3. **完成标准**：仅在完全完成后才标 completed；遇阻保持 in_progress 并新建描述阻塞的任务
4. **拆分**：创建具体、可执行的检索任务；复杂需求拆成小步；名称聚焦要检索/研究什么；**不要**包含摘要/综合任务

**重要**：todo_write 中所有检索任务完成后，用 thinking 工具综合发现并生成最终答案。todo_write 跟踪「检索什么」，thinking 负责「如何综合与呈现」。

有疑问时优先使用本工具。主动做任务管理能体现关注度，并确保完成全部检索要求。`,
	schema: utils.GenerateSchema[TodoWriteInput](),
}

// TodoWriteTool implements a planning tool for complex tasks
// This is an optional tool that helps organize multi-step research
type TodoWriteTool struct {
	BaseTool
}

// TodoWriteInput defines the input parameters for todo_write tool
type TodoWriteInput struct {
	Task  string     `json:"task,omitempty" jsonschema:"The complex task or question you need to create a plan for"`
	Steps []PlanStep `json:"steps" jsonschema:"Array of research plan steps with status tracking"`
}

// PlanStep represents a single step in the research plan
type PlanStep struct {
	ID          string `json:"id" jsonschema:"Unique identifier for this step (e.g., 'step1', 'step2')"`
	Description string `json:"description" jsonschema:"Clear description of what to investigate or accomplish in this step"`
	Status      string `json:"status" jsonschema:"Current status: pending (not started), in_progress (executing), completed (finished)"`
}

// NewTodoWriteTool creates a new todo_write tool instance
func NewTodoWriteTool() *TodoWriteTool {
	return &TodoWriteTool{
		BaseTool: todoWriteTool,
	}
}

// Execute executes the todo_write tool
func (t *TodoWriteTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	// Parse args from json.RawMessage
	var input TodoWriteInput
	if err := json.Unmarshal(args, &input); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	if input.Task == "" {
		input.Task = "No task description provided"
	}

	// Parse plan steps
	planSteps := input.Steps

	// Generate formatted output
	output := generatePlanOutput(input.Task, planSteps)

	// Prepare structured data for response
	stepsJSON, _ := json.Marshal(planSteps)

	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"task":         input.Task,
			"steps":        planSteps,
			"steps_json":   string(stepsJSON),
			"total_steps":  len(planSteps),
			"plan_created": true,
			"display_type": "plan",
		},
	}, nil
}

// Helper function to safely get string field from map
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// Helper function to safely get string array field from map
func getStringArrayField(m map[string]interface{}, key string) []string {
	if val, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, item := range val {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	// Handle legacy string format for backward compatibility
	if val, ok := m[key].(string); ok && val != "" {
		return []string{val}
	}
	return []string{}
}

// generatePlanOutput generates a formatted plan output
func generatePlanOutput(task string, steps []PlanStep) string {
	output := "Plan created\n\n"
	output += fmt.Sprintf("**Task**: %s\n\n", task)

	if len(steps) == 0 {
		output += "Note: No specific steps provided. It is recommended to create 3-7 retrieval tasks for systematic research.\n\n"
		output += "Suggested retrieval workflow (focused on retrieval tasks, excluding summarization):\n"
		output += "1. Use grep_chunks to search keywords and locate relevant documents\n"
		output += "2. Use knowledge_search for semantic search to retrieve relevant content\n"
		output += "3. Use list_knowledge_chunks to get the full content of key documents\n"
		output += "4. Use web_search to get supplementary information (if needed)\n"
		output += "\nNote: Summarization and synthesis are handled by the thinking tool. Do not add summarization tasks here.\n"
		return output
	}

	// Count task statuses
	pendingCount := 0
	inProgressCount := 0
	completedCount := 0
	for _, step := range steps {
		switch step.Status {
		case "pending":
			pendingCount++
		case "in_progress":
			inProgressCount++
		case "completed":
			completedCount++
		}
	}
	totalCount := len(steps)
	remainingCount := pendingCount + inProgressCount

	output += "**Plan Steps**:\n\n"

	// Display all steps in order
	for i, step := range steps {
		output += formatPlanStep(i+1, step)
	}

	// Add summary and emphasis on remaining tasks
	output += "\n=== Task Progress ===\n"
	output += fmt.Sprintf("Total: %d tasks\n", totalCount)
	output += fmt.Sprintf("✅ Completed: %d\n", completedCount)
	output += fmt.Sprintf("🔄 In Progress: %d\n", inProgressCount)
	output += fmt.Sprintf("⏳ Pending: %d\n", pendingCount)

	output += "\n=== ⚠️ Important Reminder ===\n"
	if remainingCount > 0 {
		output += fmt.Sprintf("**%d tasks remaining!**\n\n", remainingCount)
		output += "**All tasks must be completed before summarizing or drawing conclusions.**\n\n"
		output += "Next steps:\n"
		if inProgressCount > 0 {
			output += "- Continue completing tasks currently in progress\n"
		}
		if pendingCount > 0 {
			output += fmt.Sprintf("- Start processing %d pending tasks\n", pendingCount)
			output += "- Complete each task in order, do not skip\n"
		}
		output += "- After completing each task, update todo_write to mark it as completed\n"
		output += "- Only generate the final summary after all tasks are completed\n"
	} else {
		output += "✅ **All tasks completed!**\n\n"
		output += "You can now:\n"
		output += "- Synthesize findings from all tasks\n"
		output += "- Generate a complete final answer or report\n"
		output += "- Ensure all aspects have been thoroughly researched\n"
	}

	return output
}

// formatPlanStep formats a single plan step for output
func formatPlanStep(index int, step PlanStep) string {
	statusEmoji := map[string]string{
		"pending":     "⏳",
		"in_progress": "🔄",
		"completed":   "✅",
		"skipped":     "⏭️",
	}

	emoji, ok := statusEmoji[step.Status]
	if !ok {
		emoji = "⏳"
	}

	output := fmt.Sprintf("  %d. %s [%s] %s\n", index, emoji, step.Status, step.Description)

	// if len(step.ToolsToUse) > 0 {
	// 	output += fmt.Sprintf("     工具: %s\n", strings.Join(step.ToolsToUse, ", "))
	// }

	return output
}
