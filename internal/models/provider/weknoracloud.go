package provider

import "github.com/Tencent/WeKnora/internal/types"

const (
	ProviderTreeRAGCloud ProviderName = "weknoracloud"

	// TreeRAGCloudBaseURL TreeRAGCloud 服务硬编码 Base URL（统一入口，路径由各实现拼接）
	TreeRAGCloudBaseURL = "https://weknora.weixin.qq.com"
)

type TreeRAGCloudProvider struct{}

func init() {
	Register(&TreeRAGCloudProvider{})
}

func (p *TreeRAGCloudProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        ProviderTreeRAGCloud,
		DisplayName: "TreeRAGCloud",
		Description: "TreeRAG云服务，模型：chat, embedding, rerank, vlm",
		DefaultURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: TreeRAGCloudBaseURL,
			types.ModelTypeEmbedding:   TreeRAGCloudBaseURL,
			types.ModelTypeRerank:      TreeRAGCloudBaseURL,
			types.ModelTypeVLLM:        TreeRAGCloudBaseURL,
		},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeEmbedding,
			types.ModelTypeRerank,
			types.ModelTypeVLLM,
		},
		RequiresAuth: true,
	}
}

func (p *TreeRAGCloudProvider) ValidateConfig(config *Config) error {
	// AppID/AppSecret 通过专用初始化接口写入，此处仅做结构校验。
	// 其中 AppSecret 字段当前实际承载上游 API Key。
	return nil
}
