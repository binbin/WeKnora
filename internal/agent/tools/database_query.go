package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"gorm.io/gorm"
)

var databaseQueryTool = BaseTool{
	name: ToolDatabaseQuery,
	description: `执行 SQL 查询以从数据库检索信息。

## 安全特性
- 自动注入 tenant_id：所有查询按登录用户的 tenant_id 自动过滤
- 自动软删除过滤：默认只包含 deleted_at IS NULL 的记录
- 只读查询：仅允许 SELECT
- 安全表：仅允许查询授权表（knowledge_bases、knowledges、chunks）

## 可用表与字段

### knowledge_bases
- id (VARCHAR)：知识库 ID
- name (VARCHAR)：知识库名称
- description (TEXT)：描述
- tenant_id (INTEGER)：所属租户 ID
- embedding_model_id, summary_model_id, rerank_model_id (VARCHAR)：模型 ID
- vlm_config (JSON)：含 enabled、model_id 等 VLM 设置
- created_at, updated_at, deleted_at (TIMESTAMP)

### knowledges（文档）
- id (VARCHAR)：文档 ID
- tenant_id (INTEGER)：所属租户 ID
- knowledge_base_id (VARCHAR)：所属知识库 ID
- type (VARCHAR)：文档类型
- title (VARCHAR)：文档标题
- description (TEXT)：描述
- source (VARCHAR)：来源位置
- parse_status (VARCHAR)：处理状态（unprocessed/processing/completed/failed）
- enable_status (VARCHAR)：启用状态（enabled/disabled）
- file_name, file_type (VARCHAR)：文件信息
- file_size, storage_size (BIGINT)：字节大小
- created_at, updated_at, processed_at, deleted_at (TIMESTAMP)

### chunks
- id (VARCHAR)：分块 ID
- tenant_id (INTEGER)：所属租户 ID
- knowledge_base_id (VARCHAR)：所属知识库 ID
- knowledge_id (VARCHAR)：所属文档 ID
- content (TEXT)：分块内容
- chunk_index (INTEGER)：文档内索引
- is_enabled (BOOLEAN)：启用状态
- chunk_type (VARCHAR)：类型（text/image/table）
- created_at, updated_at, deleted_at (TIMESTAMP)

## 用法示例

查询知识库信息：
{
  "sql": "SELECT id, name, description FROM knowledge_bases ORDER BY created_at DESC LIMIT 10"
}

按状态统计文档：
{
  "sql": "SELECT parse_status, COUNT(*) as count FROM knowledges GROUP BY parse_status"
}

获取存储用量：
{
  "sql": "SELECT SUM(storage_size) as total_storage FROM knowledges"
}

关联知识库与文档：
{
  "sql": "SELECT kb.name as kb_name, COUNT(k.id) as doc_count FROM knowledge_bases kb LEFT JOIN knowledges k ON kb.id = k.knowledge_base_id GROUP BY kb.id, kb.name"
}

## 重要说明
- 不要在 WHERE 中写 tenant_id——系统会自动添加
- 一般不要手动过滤 deleted_at——默认已强制 deleted_at IS NULL
- 仅允许 SELECT
- 使用 LIMIT 控制结果量以提升性能
- 跨表查询时使用合适的 JOIN
- 所有时间戳为带时区的 UTC`,
	schema: utils.GenerateSchema[DatabaseQueryInput](),
}

type DatabaseQueryInput struct {
	SQL string `json:"sql" jsonschema:"The SELECT SQL query to execute. DO NOT include tenant_id condition - it will be automatically added for security."`
}

// DatabaseQueryTool allows AI to query the database with auto-injected tenant_id for security
type DatabaseQueryTool struct {
	BaseTool
	db            *gorm.DB
	searchTargets types.SearchTargets
}

// NewDatabaseQueryTool creates a new database query tool
func NewDatabaseQueryTool(db *gorm.DB, searchTargets types.SearchTargets) *DatabaseQueryTool {
	return &DatabaseQueryTool{
		BaseTool:      databaseQueryTool,
		db:            db,
		searchTargets: searchTargets,
	}
}

// Execute executes the database query tool
func (t *DatabaseQueryTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	logger.Infof(ctx, "[Tool][DatabaseQuery] Execute started")

	tenantID := uint64(0)
	if tid, ok := ctx.Value(types.TenantIDContextKey).(uint64); ok {
		tenantID = tid
	}

	// Parse args from json.RawMessage
	var input DatabaseQueryInput
	if err := json.Unmarshal(args, &input); err != nil {
		logger.Errorf(ctx, "[Tool][DatabaseQuery] Failed to parse args: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	// Extract SQL from input
	if input.SQL == "" {
		logger.Errorf(ctx, "[Tool][DatabaseQuery] Missing or invalid SQL parameter")
		return &types.ToolResult{
			Success: false,
			Error:   "Missing or invalid 'sql' parameter",
		}, fmt.Errorf("missing sql parameter")
	}

	logger.Infof(ctx, "[Tool][DatabaseQuery] Original SQL query:\n%s", input.SQL)
	logger.Infof(ctx, "[Tool][DatabaseQuery] Tenant ID: %d", tenantID)

	// Validate and secure the SQL query
	logger.Debugf(ctx, "[Tool][DatabaseQuery] Validating and securing SQL...")
	securedSQL, err := t.validateAndSecureSQL(input.SQL, tenantID)
	if err != nil {
		logger.Errorf(ctx, "[Tool][DatabaseQuery] SQL validation failed: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("SQL validation failed: %v", err),
		}, err
	}

	logger.Infof(ctx, "[Tool][DatabaseQuery] Secured SQL query:\n%s", securedSQL)
	logger.Infof(ctx, "Executing secured SQL query - original: %s, secured: %s, tenant_id: %d",
		input.SQL, securedSQL, tenantID)

	// Execute the query
	logger.Infof(ctx, "[Tool][DatabaseQuery] Executing query against database...")
	rows, err := t.db.WithContext(ctx).Raw(securedSQL).Rows()
	if err != nil {
		logger.Errorf(ctx, "[Tool][DatabaseQuery] Query execution failed: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Query execution failed: %v", err),
		}, err
	}
	defer rows.Close()

	logger.Debugf(ctx, "[Tool][DatabaseQuery] Query executed successfully, processing rows...")

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to get columns: %v", err),
		}, err
	}

	// Process results
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		// Create a slice of interface{} to hold each column value
		columnValues := make([]interface{}, len(columns))
		columnPointers := make([]interface{}, len(columns))
		for i := range columnValues {
			columnPointers[i] = &columnValues[i]
		}

		// Scan the row
		if err := rows.Scan(columnPointers...); err != nil {
			return &types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to scan row: %v", err),
			}, err
		}

		// Create a map for this row
		rowMap := make(map[string]interface{})
		for i, colName := range columns {
			val := columnValues[i]
			// Convert []byte to string for better readability
			if b, ok := val.([]byte); ok {
				rowMap[colName] = string(b)
			} else {
				rowMap[colName] = val
			}
		}
		results = append(results, rowMap)
	}

	if err := rows.Err(); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Error iterating rows: %v", err),
		}, err
	}

	logger.Infof(ctx, "[Tool][DatabaseQuery] Retrieved %d rows with %d columns", len(results), len(columns))
	logger.Debugf(ctx, "[Tool][DatabaseQuery] Columns: %v", columns)

	// Log first few rows for debugging
	if len(results) > 0 {
		logger.Debugf(ctx, "[Tool][DatabaseQuery] First row sample:")
		for key, value := range results[0] {
			logger.Debugf(ctx, "[Tool][DatabaseQuery]   %s: %v", key, value)
		}
	}

	// Format output
	logger.Debugf(ctx, "[Tool][DatabaseQuery] Formatting query results...")
	output := t.formatQueryResults(columns, results)

	logger.Infof(ctx, "[Tool][DatabaseQuery] Execute completed successfully: %d rows returned", len(results))
	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"columns":      columns,
			"rows":         results,
			"row_count":    len(results),
			"display_type": "database_query",
		},
	}, nil
}

// validateAndSecureSQL validates the SQL query and injects tenant_id conditions
func (t *DatabaseQueryTool) validateAndSecureSQL(sqlQuery string, tenantID uint64) (string, error) {
	securedSQL, validationResult, err := utils.ValidateAndSecureSQL(
		sqlQuery,
		utils.WithSecurityDefaults(tenantID),
		utils.WithSoftDeleteFilter("knowledge_bases", "knowledges", "chunks"),
		utils.WithHiddenKBFilter(),
		utils.WithInjectionRiskCheck(),
		utils.WithSearchScopes(searchScopesFromTargets(t.searchTargets)),
	)
	if err != nil {
		return "", err
	}

	if !validationResult.Valid {
		var errMsgs []string
		for _, valErr := range validationResult.Errors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", valErr.Type, valErr.Message))
		}
		return "", fmt.Errorf("validation failed: %s", strings.Join(errMsgs, "; "))
	}

	return securedSQL, nil
}

func searchScopesFromTargets(searchTargets types.SearchTargets) []utils.SearchScope {
	scopes := make([]utils.SearchScope, 0, len(searchTargets))
	for _, target := range searchTargets {
		if target == nil || target.KnowledgeBaseID == "" {
			continue
		}
		scopes = append(scopes, utils.SearchScope{
			KnowledgeBaseID: target.KnowledgeBaseID,
			KnowledgeIDs:    append([]string(nil), target.KnowledgeIDs...),
			TagIDs:          append([]string(nil), target.TagIDs...),
		})
	}
	return scopes
}

// formatQueryResults formats query results into readable text
func (t *DatabaseQueryTool) formatQueryResults(
	columns []string,
	results []map[string]interface{},
) string {
	output := "=== Query Results ===\n\n"
	output += fmt.Sprintf("Returned %d rows\n\n", len(results))

	if len(results) == 0 {
		output += "No matching records found.\n"
		return output
	}

	output += "=== Data Details ===\n\n"

	// Format each row
	for i, row := range results {
		output += fmt.Sprintf("--- Record #%d ---\n", i+1)
		for _, col := range columns {
			value := row[col]
			// Format the value
			var formattedValue string
			if value == nil {
				formattedValue = "<NULL>"
			} else if jsonData, err := json.Marshal(value); err == nil {
				// Check if it's a complex type
				switch v := value.(type) {
				case string:
					formattedValue = v
				case []byte:
					formattedValue = string(v)
				default:
					formattedValue = string(jsonData)
				}
			} else {
				formattedValue = fmt.Sprintf("%v", value)
			}

			output += fmt.Sprintf("  %s: %s\n", col, formattedValue)
		}
		output += "\n"
	}

	// Add summary statistics if applicable
	if len(results) > 10 {
		output += fmt.Sprintf("Note: Showing %d records out of %d total. Consider using a LIMIT clause to restrict the result count.\n", len(results), len(results))
	}

	return output
}
