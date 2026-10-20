import type { ModelConfig } from '@/api/model'

export type ModelSelectorType = ModelConfig['type']

export function filterModelsByType(
  allModels: ModelConfig[],
  modelType: ModelSelectorType,
  onlyBuiltin = false,
): ModelConfig[] {
  const byType = modelType === 'VLLM'
    ? allModels.filter(
      (model) =>
        model.type === 'VLLM' ||
        (model.type === 'KnowledgeQA' &&
          model.parameters?.supports_vision === true),
    )
    : allModels.filter((model) => model.type === modelType)

  if (!onlyBuiltin) {
    return byType
  }
  return byType.filter((model) => model.is_builtin === true)
}
