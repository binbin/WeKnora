package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/skills"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
)

// Tool name constant for execute_skill_script

var executeSkillScriptTool = BaseTool{
	name: ToolExecuteSkillScript,
	description: `在沙箱环境中执行技能自带的脚本。

## 用法
- 用于运行技能捆绑的工具脚本
- 脚本在隔离沙箱中执行以保证安全
- 仅可执行已加载技能中的脚本
- 用户上传的文件列在当前 ` + "`<sandbox_attachments>`" + ` 块中。若脚本接受输入文件，请通过 ` + "`args`" + ` 传入其绝对路径 ` + "`/workspace/input/...`" + `
- 将 ` + "`/workspace/input`" + ` 视为只读。生成的文件只能写到 ` + "`$WEKNORA_SKILL_OUTPUT_DIR`" + `，以便收集下载

## 何时使用
- 技能指令引用了工具脚本时（如 "Run scripts/analyze_form.py"）
- 技能流程需要自动化或数据处理时
- 确定性操作中脚本执行比生成代码更可靠时

## 安全
- 脚本在权限受限的沙箱中运行
- 默认禁用网络访问
- 文件访问限制在技能目录内

## 返回
- 脚本的 stdout 与 stderr 输出
- 表示成功（0）或失败（非 0）的退出码`,
	schema: utils.GenerateSchema[ExecuteSkillScriptInput](),
}

// ExecuteSkillScriptInput defines the input parameters for the execute_skill_script tool
type ExecuteSkillScriptInput struct {
	SkillName  string   `json:"skill_name" jsonschema:"Name of the skill containing the script"`
	ScriptPath string   `json:"script_path" jsonschema:"Relative path to the script within the skill directory (e.g. scripts/analyze.py)"`
	Args       []string `json:"args,omitempty" jsonschema:"Optional command-line arguments. For file flags, pass an absolute path from the current <sandbox_attachments> block (/workspace/input/...). For in-memory data, use input instead."`
	Input      string   `json:"input,omitempty" jsonschema:"Optional input data to pass to the script via stdin. Use this when you have data in memory (e.g. JSON string) that the script should process. This is equivalent to piping data: echo 'data' | python script.py"`
}

// UnmarshalJSON accepts args as either the documented string array or a single
// command-line string. Some model providers emit a string for a single tool
// argument; accepting it here keeps that malformed-but-unambiguous call from
// failing before the script can run.
func (i *ExecuteSkillScriptInput) UnmarshalJSON(data []byte) error {
	var raw struct {
		SkillName  string          `json:"skill_name"`
		ScriptPath string          `json:"script_path"`
		Args       json.RawMessage `json:"args"`
		Input      string          `json:"input"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	i.SkillName = raw.SkillName
	i.ScriptPath = raw.ScriptPath
	i.Input = raw.Input
	i.Args = nil

	if len(raw.Args) == 0 || string(raw.Args) == "null" {
		return nil
	}

	if err := json.Unmarshal(raw.Args, &i.Args); err == nil {
		return nil
	}

	var argsString string
	if err := json.Unmarshal(raw.Args, &argsString); err != nil {
		return fmt.Errorf("args must be a string or an array of strings: %w", err)
	}

	// A string is interpreted as a conventional space-separated command line.
	// The tool schema continues to advertise []string, so well-formed calls are
	// unaffected; this is only a compatibility fallback for model output.
	i.Args = strings.Fields(argsString)
	return nil
}

// ExecuteSkillScriptTool allows the agent to execute skill scripts in a sandbox
type ExecuteSkillScriptTool struct {
	BaseTool
	skillManager *skills.Manager
}

// NewExecuteSkillScriptTool creates a new execute_skill_script tool instance
func NewExecuteSkillScriptTool(skillManager *skills.Manager) *ExecuteSkillScriptTool {
	return &ExecuteSkillScriptTool{
		BaseTool:     executeSkillScriptTool,
		skillManager: skillManager,
	}
}

// Execute executes the execute_skill_script tool
func (t *ExecuteSkillScriptTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	logger.Infof(ctx, "[Tool][ExecuteSkillScript] Execute started")

	// Parse input
	var input ExecuteSkillScriptInput
	if err := json.Unmarshal(args, &input); err != nil {
		logger.Errorf(ctx, "[Tool][ExecuteSkillScript] Failed to parse args: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, nil
	}

	// Validate required fields
	if input.SkillName == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "skill_name is required",
		}, nil
	}

	if input.ScriptPath == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "script_path is required",
		}, nil
	}

	// Check if skill manager is available
	if t.skillManager == nil || !t.skillManager.IsEnabled() {
		return &types.ToolResult{
			Success: false,
			Error:   "Skills are not enabled",
		}, nil
	}

	// Execute the script in sandbox
	logger.Infof(ctx, "[Tool][ExecuteSkillScript] Executing script: %s/%s with args: %v, input length: %d",
		input.SkillName, input.ScriptPath, input.Args, len(input.Input))

	result, err := t.skillManager.ExecuteScript(ctx, input.SkillName, input.ScriptPath, input.Args, input.Input)
	if err != nil {
		logger.Errorf(ctx, "[Tool][ExecuteSkillScript] Script execution failed: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Script execution failed: %v", err),
		}, nil
	}

	// Build output
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("=== Script Execution: %s/%s ===\n\n", input.SkillName, input.ScriptPath))

	if len(input.Args) > 0 {
		builder.WriteString(fmt.Sprintf("**Arguments**: %v\n", input.Args))
	}

	builder.WriteString(fmt.Sprintf("**Exit Code**: %d\n", result.ExitCode))
	builder.WriteString(fmt.Sprintf("**Duration**: %v\n\n", result.Duration))

	if result.Killed {
		builder.WriteString("**Warning**: Script was terminated (timeout or killed)\n\n")
	}

	if result.Stdout != "" {
		builder.WriteString("## Standard Output\n\n")
		builder.WriteString("```\n")
		builder.WriteString(result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("```\n\n")
	}

	if result.Stderr != "" {
		builder.WriteString("## Standard Error\n\n")
		builder.WriteString("```\n")
		builder.WriteString(result.Stderr)
		if !strings.HasSuffix(result.Stderr, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("```\n\n")
	}

	if result.Error != "" {
		builder.WriteString("## Error\n\n")
		builder.WriteString(result.Error)
		builder.WriteString("\n")
	}

	// Determine success based on exit code
	success := result.IsSuccess()

	resultData := map[string]interface{}{
		"skill_name":  input.SkillName,
		"script_path": input.ScriptPath,
		"args":        input.Args,
		"exit_code":   result.ExitCode,
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"duration_ms": result.Duration.Milliseconds(),
		"killed":      result.Killed,
	}

	logger.Infof(ctx, "[Tool][ExecuteSkillScript] Script completed with exit code: %d", result.ExitCode)

	return &types.ToolResult{
		Success: success,
		Output:  builder.String(),
		Data:    resultData,
		Error: func() string {
			if !success {
				if result.Error != "" {
					return result.Error
				}
				return fmt.Sprintf("Script exited with code %d", result.ExitCode)
			}
			return ""
		}(),
	}, nil
}

// Cleanup releases any resources
func (t *ExecuteSkillScriptTool) Cleanup(ctx context.Context) error {
	return nil
}
