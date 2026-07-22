package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

type graphConfigSummary struct {
	Nodes     []string
	Relations []string
}

var queryKnowledgeGraphTool = BaseTool{
	name: ToolQueryKnowledgeGraph,
	description: `查询知识图谱，探索实体关系与知识网络。

## 核心功能
在已配置图谱抽取的知识库中探索实体之间的关系。

## 何时使用
✅ **适用于**：
- 理解实体间关系（如「Docker 与 Kubernetes 的关系」）
- 探索知识网络与概念关联
- 查找特定实体的相关信息
- 理解技术架构与系统关系

❌ **不要用于**：
- 一般文本搜索 → 使用 knowledge_search
- 未配置图谱抽取的知识库
- 需要精确文档内容 → 使用 knowledge_search

## 参数
- **knowledge_base_ids**（必需）：知识库短 ID bN 数组（1-10）。仅已配置图谱抽取的知识库会生效。
- **query**（必需）：查询内容——可以是实体名、关系查询或概念搜索。

## 图谱配置
知识图谱须在知识库中预先配置：
- **实体类型**（Nodes）：如 "Technology"、"Tool"、"Concept"
- **关系类型**（Relations）：如 "depends_on"、"uses"、"contains"

若知识库未配置图谱，工具将返回常规搜索结果。

## 工作流
1. **关系探索**：query_knowledge_graph → list_knowledge_chunks（获取详细内容）
2. **网络分析**：query_knowledge_graph → knowledge_search（获得全面理解）
3. **主题研究**：knowledge_search → query_knowledge_graph（深入实体关系）

## 注意
- 结果会标明图谱配置状态
- 跨知识库结果会自动去重
- 结果按相关性排序`,
	schema: utils.GenerateSchema[QueryKnowledgeGraphInput](),
}

// QueryKnowledgeGraphInput defines the input parameters for query knowledge graph tool
type QueryKnowledgeGraphInput struct {
	KnowledgeBaseIDs []string `json:"knowledge_base_ids" jsonschema:"Array of short bN knowledge base IDs to query"`
	Query            string   `json:"query" jsonschema:"Query content (entity name or query text)"`
}

// QueryKnowledgeGraphTool queries the knowledge graph for entities and relationships
type QueryKnowledgeGraphTool struct {
	BaseTool
	knowledgeService interfaces.KnowledgeBaseService
}

// NewQueryKnowledgeGraphTool creates a new query knowledge graph tool
func NewQueryKnowledgeGraphTool(knowledgeService interfaces.KnowledgeBaseService) *QueryKnowledgeGraphTool {
	return &QueryKnowledgeGraphTool{
		BaseTool:         queryKnowledgeGraphTool,
		knowledgeService: knowledgeService,
	}
}

// Execute performs the knowledge graph query with concurrent KB processing
func (t *QueryKnowledgeGraphTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	// Parse args from json.RawMessage
	var input QueryKnowledgeGraphInput
	if err := json.Unmarshal(args, &input); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	// Extract knowledge_base_ids array
	if len(input.KnowledgeBaseIDs) == 0 {
		return &types.ToolResult{
			Success: false,
			Error:   "knowledge_base_ids is required and must be a non-empty array",
		}, fmt.Errorf("knowledge_base_ids is required")
	}

	// Validate max 10 KBs
	if len(input.KnowledgeBaseIDs) > 10 {
		return &types.ToolResult{
			Success: false,
			Error:   "knowledge_base_ids must contain at most 10 KB IDs",
		}, fmt.Errorf("too many KB IDs")
	}

	query := input.Query
	if query == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "query is required",
		}, fmt.Errorf("invalid query")
	}

	// Concurrently query all knowledge bases
	type graphQueryResult struct {
		kbID    string
		kb      *types.KnowledgeBase
		results []*types.SearchResult
		err     error
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	kbResults := make(map[string]*graphQueryResult)

	searchParams := types.SearchParams{
		QueryText:  query,
		MatchCount: 10,
	}

	for _, kbID := range input.KnowledgeBaseIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			// Get knowledge base to check graph configuration
			kb, err := t.knowledgeService.GetKnowledgeBaseByID(ctx, id)
			if err != nil {
				mu.Lock()
				kbResults[id] = &graphQueryResult{kbID: id, err: fmt.Errorf("failed to get knowledge base: %v", err)}
				mu.Unlock()
				return
			}

			// Check if graph extraction is enabled
			if kb.ExtractConfig == nil || (len(kb.ExtractConfig.Nodes) == 0 && len(kb.ExtractConfig.Relations) == 0) {
				mu.Lock()
				kbResults[id] = &graphQueryResult{kbID: id, err: fmt.Errorf("graph extraction not configured")}
				mu.Unlock()
				return
			}

			// Query graph
			results, err := t.knowledgeService.HybridSearch(ctx, id, searchParams)
			if err != nil {
				mu.Lock()
				kbResults[id] = &graphQueryResult{kbID: id, kb: kb, err: fmt.Errorf("query failed: %v", err)}
				mu.Unlock()
				return
			}

			mu.Lock()
			kbResults[id] = &graphQueryResult{kbID: id, kb: kb, results: results}
			mu.Unlock()
		}(kbID)
	}

	wg.Wait()

	// Collect and deduplicate results
	seenChunks := make(map[string]*types.SearchResult)
	var errors []string
	graphConfigs := make(map[string]graphConfigSummary)
	kbCounts := make(map[string]int)

	for _, kbID := range input.KnowledgeBaseIDs {
		result := kbResults[kbID]
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("KB %s: %v", kbID, result.err))
			continue
		}

		if result.kb != nil && result.kb.ExtractConfig != nil {
			graphConfigs[kbID] = summarizeGraphConfig(result.kb.ExtractConfig)
		}

		kbCounts[kbID] = len(result.results)
		for _, r := range result.results {
			if _, seen := seenChunks[r.ID]; !seen {
				seenChunks[r.ID] = r
			}
		}
	}

	// Convert map to slice and sort by score
	allResults := make([]*types.SearchResult, 0, len(seenChunks))
	for _, result := range seenChunks {
		allResults = append(allResults, result)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	if len(allResults) == 0 {
		return &types.ToolResult{
			Success: true,
			Output:  "No relevant graph information found.",
			Data: map[string]interface{}{
				"knowledge_base_ids": input.KnowledgeBaseIDs,
				"query":              query,
				"results":            []interface{}{},
				"graph_configs":      graphConfigsToData(graphConfigs),
				"graph_config":       aggregateGraphConfig(graphConfigs),
				"errors":             errors,
			},
		}, nil
	}

	// Format output with enhanced graph information
	output := "=== Knowledge Graph Query ===\n\n"
	output += fmt.Sprintf("📊 Query: %s\n", query)
	output += fmt.Sprintf("🎯 Target Knowledge Bases: %v\n", input.KnowledgeBaseIDs)
	output += fmt.Sprintf("✓ Found %d relevant results (deduplicated)\n\n", len(allResults))

	if len(errors) > 0 {
		output += "=== ⚠️ Partial Failures ===\n"
		for _, errMsg := range errors {
			output += fmt.Sprintf("  - %s\n", errMsg)
		}
		output += "\n"
	}

	// Display graph configuration status
	hasGraphConfig := false
	output += "=== 📈 Graph Configuration Status ===\n\n"
	for kbID, config := range graphConfigs {
		hasGraphConfig = true
		output += fmt.Sprintf("Knowledge Base [%s]:\n", kbID)

		if len(config.Nodes) > 0 {
			output += fmt.Sprintf("  ✓ Entity Types (%d): %v\n", len(config.Nodes), config.Nodes)
		} else {
			output += "  ⚠️ No entity types configured\n"
		}

		if len(config.Relations) > 0 {
			output += fmt.Sprintf("  ✓ Relationship Types (%d): %v\n", len(config.Relations), config.Relations)
		} else {
			output += "  ⚠️ No relationship types configured\n"
		}
		output += "\n"
	}

	if !hasGraphConfig {
		output += "⚠️ None of the queried knowledge bases have graph extraction configured\n"
		output += "💡 Hint: Configure entity and relationship types in knowledge base settings\n\n"
	}

	// Display result counts by KB
	if len(kbCounts) > 0 {
		output += "=== 📚 Knowledge Base Coverage ===\n"
		for kbID, count := range kbCounts {
			output += fmt.Sprintf("  - %s: %d results\n", kbID, count)
		}
		output += "\n"
	}

	// Display search results
	output += "=== 🔍 Query Results ===\n\n"
	if !hasGraphConfig {
		output += "💡 Returning relevant document chunks (knowledge base has no graph configuration)\n\n"
	} else {
		output += "💡 Content retrieval based on graph configuration\n\n"
	}

	formattedResults := make([]map[string]interface{}, 0, len(allResults))
	currentKB := ""

	for i, result := range allResults {
		// Group by knowledge base
		if result.KnowledgeID != currentKB {
			currentKB = result.KnowledgeID
			if i > 0 {
				output += "\n"
			}
			output += fmt.Sprintf("[Source Document: %s]\n\n", result.KnowledgeTitle)
		}

		relevanceLevel := GetRelevanceLevel(result.Score)

		output += fmt.Sprintf("Result #%d:\n", i+1)
		output += fmt.Sprintf("  📍 Relevance: %.2f (%s)\n", result.Score, relevanceLevel)
		output += fmt.Sprintf("  🔗 Match Type: %s\n", FormatMatchType(result.MatchType))
		output += fmt.Sprintf("  📄 Content: %s\n", result.Content)
		output += fmt.Sprintf("  🆔 chunk_id: %s\n\n", result.ID)

		formattedResults = append(formattedResults, map[string]interface{}{
			"result_index":      i + 1,
			"chunk_id":          result.ID,
			"chunk_index":       result.ChunkIndex,
			"chunk_type":        result.ChunkType,
			"content":           result.Content,
			"score":             result.Score,
			"relevance_level":   relevanceLevel,
			"knowledge_id":      result.KnowledgeID,
			"knowledge_base_id": result.KnowledgeBaseID,
			"knowledge_title":   result.KnowledgeTitle,
			"match_type":        FormatMatchType(result.MatchType),
		})
	}

	output += "=== 💡 Tips ===\n"
	output += "- ✓ Results are deduplicated across knowledge bases and sorted by relevance\n"
	output += "- ✓ Use get_chunk_detail to get full content\n"
	output += "- ✓ Use list_knowledge_chunks to explore context\n"
	if !hasGraphConfig {
		output += "- ⚠️ Configure graph extraction for more precise entity-relationship results\n"
	}
	output += "- ⏳ Full graph query language (Cypher) support is under development\n"

	// Build structured graph data for frontend visualization
	graphData := buildGraphVisualizationData(allResults)

	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"knowledge_base_ids": input.KnowledgeBaseIDs,
			"query":              query,
			"results":            formattedResults,
			"count":              len(allResults),
			"kb_counts":          kbCounts,
			"graph_configs":      graphConfigsToData(graphConfigs),
			"graph_config":       aggregateGraphConfig(graphConfigs),
			"graph_data":         graphData,
			"has_graph_config":   hasGraphConfig,
			"errors":             errors,
			"display_type":       "graph_query_results",
		},
	}, nil
}

func summarizeGraphConfig(config *types.ExtractConfig) graphConfigSummary {
	if config == nil {
		return graphConfigSummary{}
	}

	return graphConfigSummary{
		Nodes:     uniqueSortedNodeNames(config.Nodes),
		Relations: uniqueSortedRelationNames(config.Relations),
	}
}

func uniqueSortedNodeNames(nodes []*types.GraphNode) []string {
	seen := make(map[string]struct{}, len(nodes))
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == "" {
			continue
		}
		if _, exists := seen[node.Name]; exists {
			continue
		}
		seen[node.Name] = struct{}{}
		names = append(names, node.Name)
	}
	sort.Strings(names)
	return names
}

func uniqueSortedRelationNames(relations []*types.GraphRelation) []string {
	seen := make(map[string]struct{}, len(relations))
	names := make([]string, 0, len(relations))
	for _, relation := range relations {
		if relation == nil || relation.Type == "" {
			continue
		}
		if _, exists := seen[relation.Type]; exists {
			continue
		}
		seen[relation.Type] = struct{}{}
		names = append(names, relation.Type)
	}
	sort.Strings(names)
	return names
}

func graphConfigsToData(graphConfigs map[string]graphConfigSummary) map[string]map[string]interface{} {
	if len(graphConfigs) == 0 {
		return nil
	}

	data := make(map[string]map[string]interface{}, len(graphConfigs))
	for kbID, config := range graphConfigs {
		data[kbID] = map[string]interface{}{
			"nodes":     config.Nodes,
			"relations": config.Relations,
		}
	}
	return data
}

func aggregateGraphConfig(graphConfigs map[string]graphConfigSummary) map[string]interface{} {
	if len(graphConfigs) == 0 {
		return nil
	}

	merged := graphConfigSummary{}
	for _, config := range graphConfigs {
		merged.Nodes = append(merged.Nodes, config.Nodes...)
		merged.Relations = append(merged.Relations, config.Relations...)
	}

	return map[string]interface{}{
		"nodes":     uniqueStrings(merged.Nodes),
		"relations": uniqueStrings(merged.Relations),
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// buildGraphVisualizationData builds structured data for graph visualization
func buildGraphVisualizationData(results []*types.SearchResult) map[string]interface{} {
	// Build a simple graph structure for frontend visualization
	nodes := make([]map[string]interface{}, 0)
	edges := make([]map[string]interface{}, 0)

	// Create nodes from results
	seenEntities := make(map[string]bool)
	for i, result := range results {
		if !seenEntities[result.ID] {
			nodes = append(nodes, map[string]interface{}{
				"id":       result.ID,
				"label":    fmt.Sprintf("Chunk %d", i+1),
				"content":  result.Content,
				"kb_id":    result.KnowledgeID,
				"kb_title": result.KnowledgeTitle,
				"score":    result.Score,
				"type":     "chunk",
			})
			seenEntities[result.ID] = true
		}
	}

	return map[string]interface{}{
		"nodes":       nodes,
		"edges":       edges,
		"total_nodes": len(nodes),
		"total_edges": len(edges),
	}
}
