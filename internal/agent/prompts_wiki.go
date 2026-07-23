package agent

// WikiTaxonomyPlanPrompt assigns a directory path (category) to every entity /
// concept page produced by ONE ingest batch in a single call, so the whole set
// lands on one coherent tree that reuses existing folders — instead of each page
// inventing its own folders in parallel (which diverges worst on the founding
// batch, when the KB still has no folders to anchor on). The result is applied
// in reduce only to pages that don't already have a category, so user edits and
// previously-filed pages are never churned.
const WikiTaxonomyPlanPrompt = `你正在将一个 wiki 知识库整理成导航目录。请为下面的每个条目分配一个目录路径（分类），使整批条目落在一棵连贯的树上。

<existing_folders>
{{.ExistingTaxonomy}}
</existing_folders>

<items>
{{.Items}}
</items>

<instructions>
为每个条目输出一个分类路径：一个由宽到窄的文件夹标签数组（最多 2 级）。分类描述的是该条目本质上"是什么"（它在图书馆书架上长期固定的位置），而不是它在某一份文档中扮演的角色。

如何为每个条目选择路径：
1. 如果 <existing_folders> 中已有合适的文件夹，请复用其**完全一致**的标签（逐字符一致）。不要创造同义的新文件夹（例如，已经有"春节 / 传统习俗"时，不要再创建"春节习俗"）。
2. 如果没有合适的现有文件夹，请为该条目创建一个新的、宽泛的、长期稳定的文件夹（例如：一个组织 → "组织"，一个法律概念 → "法律概念"，一个地点 → "地点"）。目录不必保持很小——大多数条目确实有一个自然的归属，所以应创造一个合理的顶层文件夹，而不是让它们无处安放。将同类条目归入同一个新文件夹，以保持树结构的连贯性。
3. 只有当一个条目确实不属于任何长期稳定的主题时，才给出空路径 []。这种情况必须**罕见**。找不到匹配的现有文件夹不是使用 [] 的理由——应该创建一个新文件夹。

其他规则：
- 将同类条目归入同一深度的同一文件夹。不要把某个等价条目归入比同类条目更深一级的位置（例如避免出现"地点 / 地址 / 地址1"与"地点 / 地址2"并存——对等价条目应选择一致的深度）。
- 优先使用单一的宽泛顶层文件夹；只有当多个条目确实共享一个持久的子领域时，才增加第二级。
- 不要将条目类型（"entity"/"concept"）用作文件夹名。不要在单个标签内部使用斜杠。
- <items> 中的每个条目 slug 必须在输出中恰好出现一次。
- 所有文件夹标签均使用 {{.Language}} 书写。

### JSON 格式规则
- 只输出合法的 JSON，不要有任何前言。
- 不要在 JSON 字符串值中使用字面换行符。
</instructions>

输出格式：
{
  "assignments": [
    {"slug": "entity/zhang-san", "path": ["人物"]},
    {"slug": "concept/spring-festival", "path": ["节日", "传统节日"]}
  ]
}`

// Wiki ingest prompt templates for LLM-powered wiki page generation.
// These prompts are used by the wiki ingest pipeline to extract structured
// knowledge from raw documents and build/update wiki pages.

// WikiSummaryPrompt generates a summary page for a newly ingested document.
//
// Filename and title are intentionally NOT passed to the LLM: documents
// uploaded to WeKnora often carry filenames that say nothing about the
// content (e.g. scanned PDFs named after the scanner model "MX5280.pdf"),
// and feeding such filenames to the model invites hallucinated summaries
// when the actual extracted content is thin. The model must rely solely on
// the document content provided below.
const WikiSummaryPrompt = `你是一名 wiki 编辑。根据以下文档内容，创建一个结构化的 wiki 摘要页面，使用 Markdown 格式。

<document>
<content>
{{.Content}}
</content>
</document>

<available_wiki_pages>
{{.ExtractedSlugs}}
</available_wiki_pages>

<instructions>
1. 输出的**第一行**必须是：SUMMARY: {一句话，15-40 个词，描述该文档的主题——用于 wiki 索引列表展示}
2. 在 SUMMARY 行之后，用 Markdown 格式撰写该文档的完整摘要。
3. 包含关键事实、论点和结论。
4. 使用恰当的标题层级（## 表示章节，### 表示小节）。
5. **Wiki 链接规则**：上方的 available_wiki_pages 列表将 slug 映射到显示名称及其别名（格式为："[[slug]] = 显示名称 (Aliases: a, b)"）。当你提到的名称或别名与列表中的某个条目匹配时，必须将其写成 [[slug|显示名称]]（例如 [[entity/zhong-guo|中国]]），而不是写成粗体（**名称**）或裸露的 [[slug]]。必须使用提供的**准确** slug——不要自行编造新的 slug。
6. **图片规则**：如果文档中包含 <images> 标签下的 <image> 元素，你**应该**在摘要中使用 Markdown 语法 ![caption](url) 引用相关图片。请将图片放在与文本上下文相关的位置。![caption](url) 中的 URL 是一个不透明的令牌；必须**逐字逐句**原样复制，不得修改、缩短或做任何规范化处理。
7. 结尾处加入一个 "## 要点总结" 章节，用项目符号列出要点。
8. 使用 {{.Language}} 书写。
9. 摘要应简明但充分（根据文档长度，控制在 500-1500 字左右）。
10. **空内容规则**：如果上方的 <content> 块为空、只包含没有提取出文字的图片引用，或者不包含任何实质性信息，请准确输出："SUMMARY: 本文档未能提取出任何可总结的文本内容。"，随后附上一句简要说明，指出该文档无法被总结。不要凭空猜测主题，不要根据其他线索进行推测。
</instructions>

请先输出 SUMMARY 行，然后输出 Markdown 内容。不要包含任何其他前言。`

// WikiKnowledgeExtractPrompt extracts both entities and concepts in a single LLM call.
// Returns a JSON object with "entities" and "concepts" arrays.
// This replaces the former separate WikiEntityExtractPrompt and WikiConceptExtractPrompt.
const WikiKnowledgeExtractPrompt = `你是一个知识抽取系统。请分析以下文档，抽取所有重要的实体（entity）和关键概念（concept）。

<document>
<content>
{{.Content}}
</content>
</document>

<previous_slugs>
{{.PreviousSlugs}}
</previous_slugs>

<instructions>
返回一个包含两个数组的 JSON 对象："entities" 和 "concepts"。
**重要：所有名称、描述和细节都必须使用 {{.Language}} 书写**。

如果上方的 <content> 块为空、只包含没有提取出文字的图片引用，或者不包含任何实质性信息，请返回 {"entities": [], "concepts": []}。不要从任何其他来源凭空编造实体或概念。

### Slug 连续性规则
如果上方提供了 previous_slugs，你必须遵循以下规则：
- 如果上一次抽取的某个实体或概念在当前文档中仍然存在，必须**复用其在上一次列表中的原始 slug**。不要为同一事物生成新的 slug。
- 如果某个实体或概念在文档中不再出现，**不要**将其包含在输出中。
- 只为真正全新的（不在上一次列表中的）实体/概念生成新 slug。
- 这样可以确保文档更新前后 slug 保持稳定。

### Entities（人物、组织、产品、地点、技术、事件等）
每个实体应包含：
- "name"：实体名称，使用 {{.Language}}（人类可读）
- "slug"：URL 友好的 slug，格式为 "entity/<小写连字符名称>"（非拉丁字符名称使用罗马化/拼音形式）。**如果该实体此前已被抽取过，必须复用其原有 slug。**
- "aliases"：一个字符串数组，表示指向**完全相同**实体的其他名称。仅包含：官方缩写（如 "IBM" 对应 "International Business Machines"）、全称/简称变体（如 "腾讯" 对应 "腾讯控股有限公司"）、译名（如 "苹果公司" 对应 "Apple"）、以及广为人知的其他名称（如 "谷歌母公司" 对应 "Alphabet"）。不要包含上级分类、相关产品、泛化术语或更宽泛的概念。若没有别名则给出 []。
- "description"：**索引列表摘要**——一句话，15-40 个词，使用 {{.Language}}。描述该实体**是什么**及其在文档中的作用。必须自足（不看全文也能理解）。该内容会展示在 wiki 索引中。
- "details"：使用 {{.Language}} 撰写的 2-5 句关键事实摘要，取自文档内容。**图片规则**：如果文档中在 <images> 标签下包含相关的 <image> 元素，请使用 Markdown 语法 ![caption](url) 将其纳入 details 中。![caption](url) 中的 URL 是一个不透明的令牌；必须**逐字逐句**原样复制，不得修改、缩短或做任何规范化处理。

只包含被实质性讨论的实体（至少被提及两次，或有详细描述）。不要包含泛化术语。

### Concepts（主题、议题、方法论、理论等）
每个概念应包含：
- "name"：概念名称，使用 {{.Language}}（人类可读）
- "slug"：URL 友好的 slug，格式为 "concept/<小写连字符名称>"（非拉丁字符名称使用罗马化/拼音形式）。**如果该概念此前已被抽取过，必须复用其原有 slug。**
- "aliases"：一个字符串数组，表示指向**完全相同**概念的其他名称。仅包含：官方缩写（如 "RAG" 对应 "Retrieval-Augmented Generation"）、全称/简称变体、以及该领域中通用的知名同义词。不要包含子主题、相关技术、更宽泛的分类或实现细节。若没有别名则给出 []。
- "description"：**索引列表摘要**——一句话，15-40 个词，使用 {{.Language}}。定义该概念**是什么**。必须自足（不看全文也能理解）。该内容会展示在 wiki 索引中。
- "details"：使用 {{.Language}} 撰写的 2-5 句说明，取自文档中的相关论述。**图片规则**：如果文档中在 <images> 标签下包含相关的 <image> 元素，请使用 Markdown 语法 ![caption](url) 将其纳入 details 中。![caption](url) 中的 URL 是一个不透明的令牌；必须**逐字逐句**原样复制，不得修改、缩短或做任何规范化处理。

只包含被实质性讨论的概念。跳过琐碎或过于泛化的概念。

### 去重规则
- 如果某事物是一个具体命名的实物（人物、公司、产品、地点），只将其放入 "entities"。
- 如果某事物是一个抽象的想法、方法论或理论，只将其放入 "concepts"。
- 不要在两个数组中重复出现同一条目。

### JSON 格式规则
- **重要**：不要在 JSON 字符串值中使用字面换行符。如果字符串中需要换行，必须使用转义序列 \n。
</instructions>

只输出合法的 JSON。示例：
{
  "entities": [
    {
      "name": "Acme Corp",
      "slug": "entity/acme-corp",
      "aliases": ["Acme", "Acme Corporation"],
      "description": "一家专注于人工智能解决方案的科技公司。",
      "details": "Acme Corp 成立于 2020 年，目前已发展到 500 名员工。他们专注于企业级人工智能产品，并最近推出了其旗舰级 RAG 平台。"
    }
  ],
  "concepts": [
    {
      "name": "Retrieval-Augmented Generation",
      "slug": "concept/retrieval-augmented-generation",
      "aliases": ["RAG"],
      "description": "一种将信息检索与语言模型生成相结合的技术。",
      "details": "RAG 的工作方式是首先使用向量相似度搜索从知识库中检索相关文档，然后将这些文档作为上下文提供给大语言模型以生成答案。"
    }
  ]
}`

// WikiCandidateSlugPrompt (Pass 0 of the chunk-cited pipeline) asks the LLM to
// scan a document and output the SKELETON of all entities/concepts it contains:
// name, slug, aliases, a short description, and a short details tiebreaker.
// The heavy lifting — linking each slug to concrete supporting chunks — is
// done in a second pass (see WikiChunkCitationPrompt). Because this prompt no
// longer has to carry full facts per item, it stays cheap even for long docs.
const WikiCandidateSlugPrompt = `你是一个知识抽取系统。请分析以下文档，列出其中所有重要的实体（entity）和关键概念（concept），作为一个轻量级的候选集合。稍后的另一个处理阶段会为每个条目关联具体的支持性文本块（chunk），因此这里**不需要**为每个条目写出详尽的事实信息。

<document>
<content>
{{.Content}}
</content>
</document>

<previous_slugs>
{{.PreviousSlugs}}
</previous_slugs>

<instructions>
返回一个包含两个数组的 JSON 对象："entities" 和 "concepts"。
**重要：所有名称、描述和细节都必须使用 {{.Language}} 书写**。

如果上方的 <content> 块为空、只包含没有提取出文字的图片引用，或者不包含任何实质性信息，请返回 {"entities": [], "concepts": []}。不要从任何其他来源凭空编造实体或概念。

### 抽取范围（粒度：{{.Granularity}}）
{{.GranularityGuidance}}

### Slug 连续性规则
如果上方提供了 previous_slugs，你必须遵循以下规则：
- 如果上一次抽取的某个实体或概念在当前文档中仍然存在，必须**复用其在上一次列表中的原始 slug**。不要为同一事物生成新的 slug。
- 如果某个实体或概念在文档中不再出现，**不要**将其包含在输出中。
- 只为真正全新的（不在上一次列表中的）实体/概念生成新 slug。
- 这样可以确保文档更新前后 slug 保持稳定。

### Entities（人物、组织、产品、地点、技术、事件等）
每个实体应包含：
- "name"：实体名称，使用 {{.Language}}（人类可读）。
- "slug"：URL 友好的 slug，格式为 "entity/<小写连字符名称>"（非拉丁字符名称使用罗马化/拼音形式）。**如果该实体此前已被抽取过，必须复用其原有 slug。**
- "aliases"：一个字符串数组，表示指向**完全相同**实体的其他名称。仅包含：官方缩写（如 "IBM" 对应 "International Business Machines"）、全称/简称变体（如 "腾讯" 对应 "腾讯控股有限公司"）、译名、以及广为人知的其他名称。不要包含上级分类、相关产品、泛化术语或更宽泛的概念。若没有别名则给出 []。
- "description"：**索引列表摘要**——一句话，15-40 个词，使用 {{.Language}}。描述该实体**是什么**及其在文档中的作用。必须自足。该内容会展示在 wiki 索引中。
- "details"：使用 {{.Language}} 撰写的简短 1-3 句兜底摘要。该字段仅在下游按块引用（chunk-level citation）失败时使用，因此**不需要**详尽，控制在 300 字以内。

请遵循上方的抽取范围规则。切勿将只是被顺带提及的名称提升为实体。

### Concepts（主题、议题、方法论、理论等）
每个概念应包含：
- "name"：概念名称，使用 {{.Language}}（人类可读）。
- "slug"：URL 友好的 slug，格式为 "concept/<小写连字符名称>"（非拉丁字符名称使用罗马化/拼音形式）。**如果该概念此前已被抽取过，必须复用其原有 slug。**
- "aliases"：一个字符串数组，表示指向**完全相同**概念的其他名称。仅包含：官方缩写（如 "RAG" 对应 "Retrieval-Augmented Generation"）、全称/简称变体、以及该领域中通用的知名同义词。不要包含子主题、相关技术、更宽泛的分类或实现细节。若没有别名则给出 []。
- "description"：**索引列表摘要**——一句话，15-40 个词，使用 {{.Language}}。定义该概念**是什么**。必须自足。
- "details"：使用 {{.Language}} 撰写的简短 1-3 句兜底摘要，控制在 300 字以内。

请遵循上方的抽取范围规则。跳过仅被点名提及而没有实质讨论的概念。

### 去重规则
- 如果某事物是一个具体命名的实物（人物、公司、产品、地点），只将其放入 "entities"。
- 如果某事物是一个抽象的想法、方法论或理论，只将其放入 "concepts"。
- 不要在两个数组中重复出现同一条目。

### JSON 格式规则
- **重要**：不要在 JSON 字符串值中使用字面换行符。如果字符串中需要换行，必须使用转义序列 \n。
</instructions>

只输出合法的 JSON。示例：
{
  "entities": [
    {
      "name": "Acme Corp",
      "slug": "entity/acme-corp",
      "aliases": ["Acme", "Acme Corporation"],
      "description": "一家专注于人工智能解决方案的科技公司。",
      "details": "成立于 2020 年，专注于企业级人工智能产品。"
    }
  ],
  "concepts": [
    {
      "name": "Retrieval-Augmented Generation",
      "slug": "concept/retrieval-augmented-generation",
      "aliases": ["RAG"],
      "description": "一种将信息检索与语言模型生成相结合的技术。",
      "details": "先检索相关文档，再将其作为上下文提供给大语言模型。"
    }
  ]
}`

// WikiChunkCitationPrompt (Pass 1..N of the chunk-cited pipeline) asks the LLM
// to read a batch of chunks and, for each candidate entity/concept, list the
// chunk IDs that substantively discuss it. This keeps per-slug "facts" in
// their verbatim form (the chunk text) instead of asking the LLM to paraphrase.
// Block order matters for provider prefix caching: the static rules,
// output schema and the per-document-stable <candidate_slugs> are placed
// BEFORE the per-batch <chunks> block. Within one document only ChunksXML
// changes between batches, so every batch after the first shares the long
// [rules | candidate_slugs] prefix and avoids re-billing the static rules.
const WikiChunkCitationPrompt = `你是一个精确的引用系统。你的任务是扫描一批文档文本块（chunk），并针对下方的每个候选实体/概念，判断哪些文本块对其进行了实质性讨论。

<instructions>
**重要：所有名称、描述和细节都必须使用 {{.Language}} 书写**。

### 主要任务
针对下方 <candidate_slugs> 中列出的每个候选 slug，从下方 <chunks> 块中选出**实质性讨论**该实体/概念的文本块 ID。"实质性"意味着该文本块陈述了至少一个关于该候选对象的具体事实、属性、步骤、日期、数字、关系或其他有用信息——而不仅仅是一笔带过。

- 只能引用出现在下方 <chunks> 块中的文本块。
- 使用每个 <c> 元素的 "id" 属性，原样引用（例如 "c003"）。
- 如果某个候选对象在这批文本块中没有被任何一个文本块实质性讨论，则在输出中省略它（不要输出空数组）。
- 如果一个文本块确实同时实质性讨论了多个候选对象，可以被多个候选对象引用。
- 如果某个文本块内容过长或混杂了多个不相关主题，只要它讨论了某个候选对象，仍应为该候选对象引用它。

### 次要任务：新增 slug
如果这批文本块揭示了一个重要的实体/概念，且它**不在** <candidate_slugs> 中，你可以将其加入 "new_slugs"，以便纳入结果。只添加真正全新且被实质性讨论的条目。不要重新发现已经在 <candidate_slugs> 中列出的条目——如果它们已经是候选对象，请复用其 slug。

每个新 slug 必须包含：
- "type"："entity" 或 "concept"
- "name"、"slug"、"aliases"、"description"、"details"（语义与候选列表相同）
- "source_chunks"：当前批次中讨论该条目的文本块 ID 列表

### JSON 格式规则
- **重要**：不要在 JSON 字符串值中使用字面换行符。如需换行，请使用 \n。
- 只输出合法的 JSON，不要有任何前言。
</instructions>

输出格式：
{
  "citations": {
    "entity/xxx": ["c001", "c003"],
    "concept/yyy": ["c002"]
  },
  "new_slugs": [
    {
      "type": "entity",
      "name": "Example",
      "slug": "entity/example",
      "aliases": [],
      "description": "...",
      "details": "...",
      "source_chunks": ["c005"]
    }
  ]
}

如果这批文本块中没有任何值得引用的内容，返回：{"citations": {}, "new_slugs": []}

<candidate_slugs>
{{.CandidateSlugs}}
</candidate_slugs>

<chunks>
{{.ChunksXML}}
</chunks>

现在请将上述规则应用到这批文本块，并只输出 JSON。`

// WikiPageModifySystemPrompt contains only rules shared by every page update.
// Keeping page identity and source data out of this message gives providers a
// long byte-stable prefix to cache across a reduce batch.
const WikiPageModifySystemPrompt = `You are a wiki editor tasked with updating an existing wiki page. You must process NEW information to add and/or deleted documents whose exclusive contributions must be removed.

### SOURCE GROUNDING & MERGE RULES (CRITICAL):
1. **No Inline Chunk IDs:** Chunk aliases such as [c003] are internal processing metadata. NEVER output them in the page body or summary, and remove any legacy inline chunk aliases from existing content while editing. Source associations are stored separately by the system.
2. **Mandatory Grounding:** Every newly added factual claim, entity, or numerical value MUST be directly supported by the provided new source chunks, but the final prose must remain clean Markdown without inline chunk IDs.
3. **No Hallucination:** Do not invent, synthesize, or infer any information that is not explicitly present in the provided source chunks. If the new chunks clearly and directly supersede or contradict existing content, update the main text to reflect the newer supported information AND add a brief "Contradictions / Updates" section summarizing the change. If the conflict is ambiguous, unresolved, or not directly supported by the provided chunks, do not overwrite the existing content; instead, add only a "Contradictions / Updates" section describing the conflict.
4. The shared source-context block describes what each source document is about and what kind of document it is. Use it only to calibrate scope, attribution, and tone. Never copy source-context wording into the page as factual evidence.
5. Stable system-owned output, grounding, safety, and factuality rules override any business instructions.

### EDITING AND OUTPUT RULES:
1. You are a COMPILER, not a creative writer. Stay close to the verbatim source wording. You may lightly reorder, deduplicate, and join related sentences, but must not rephrase for style, expand short statements, or invent transitions.
2. Do not over-structure. Introduce a section heading only if the source or existing page uses it. Prefer a single top-level heading, short paragraphs, and flat factual lists over an invented hierarchy.
3. Do not add rhetorical filler such as "aims to provide", "designed to", "旨在帮助", "致力于", or "具有重要意义" unless it appears verbatim in an evidentiary source chunk.
4. Keep self-reported claims scoped and attributed. Do not elevate a resume, product page, announcement, or first-person statement into an industry-wide fact.
5. Preserve existing information that remains valid and on-topic. Maintain the existing page's structure and formatting style where possible.
6. Keep a [[slug|name]] link only when its slug is present in the supplied valid-link list. Never invent a slug and never link a page to itself.
7. Images may be included only from supplied new information. Treat each Markdown image URL as an opaque token and reproduce it exactly without altering, shortening, or normalizing it.
8. The first output line must be "SUMMARY: {one sentence, 15-40 words}", followed immediately by clean Markdown page content.

Output the SUMMARY line first, followed by the updated Markdown content, with no other preamble.`

// WikiPageModifyUserPrompt contains the per-batch and per-page data. The document-
// level source context deliberately comes first: all pages generated from one
// source then share the longest possible prefix before page metadata diverges.
const WikiPageModifyUserPrompt = `{{if .HasAdditions}}<shared_source_contexts>
{{.SharedSourceContexts}}</shared_source_contexts>
{{end}}

<page_metadata>
  <slug>{{.PageSlug}}</slug>
  <title>{{.PageTitle}}</title>
  <type>{{.PageType}}</type>{{if .PageAliases}}
  <aliases>{{.PageAliases}}</aliases>{{end}}
</page_metadata>

This wiki page is specifically about **{{.PageTitle}}** (a {{.PageType}}). Every statement on the page MUST be directly about this exact {{.PageType}} — not about related, adjacent, or similarly-named things.

<existing_page_content>
{{.ExistingContent}}
</existing_page_content>

{{if .HasAdditions}}
<new_information>
{{.NewContent}}
</new_information>

The <new_information> block above is assembled from VERBATIM source chunks already cited as directly supporting this page. The preceding <shared_source_contexts> block is framing only, not evidence.
{{end}}

{{if .HasRetractions}}
<deleted_documents>
{{.DeletedContent}}
</deleted_documents>

<remaining_source_documents>
{{.RemainingSourcesContent}}
</remaining_source_documents>
{{end}}

<valid_wiki_links>
{{.AvailableSlugs}}
</valid_wiki_links>

<instructions>
1. The FIRST line of your output MUST be: SUMMARY: {one sentence, 15-40 words, describing what this page is about after the update — for wiki index listing}
{{if .HasRetractions}}
2. REMOVE facts/claims that were ONLY sourced from the <deleted_documents> and are NOT present in any <remaining_source_documents> or <new_information>.
{{end}}
{{if .HasAdditions}}
3. ADD and MERGE the facts from <new_information> into the page. You are a COMPILER, not a writer:
   - **CRITICAL CONFLICT CHECK**: First verify that the <new_information> is actually about **{{.PageTitle}}** (as declared in <page_metadata>). If a piece of new info clearly belongs to a DIFFERENT but related thing (e.g., this page is about "Hunyuan Model" but the new info is about "Qwen3"; or this page is about "居民身份证" but the new info is about "工作居住证"), you MUST REJECT that part of the new information and DO NOT add it.
   - If it is genuinely about {{.PageTitle}} and contradicts old content, prefer the newer information.
{{end}}
4. Preserve existing information that is still valid and still about {{.PageTitle}}.
5. Keep [[slug|name]] wiki-link references ONLY if the slug appears in the <valid_wiki_links> list above. Remove any [[slug|name]] whose slug is NOT in that list. Do NOT invent new wiki-link slugs. The page's own slug ({{.PageSlug}}) MUST NOT appear as a [[...]] link inside its own content.
6. Maintain the existing page structure and formatting style. Use "# {{.PageTitle}}" as the top-level heading if the page does not already have one. Do NOT introduce new heading levels beyond what the source or existing page justifies.
{{if .HasRetractions}}
7. If after removing deleted content the page becomes nearly empty and there is no new information to add, output just: "SUMMARY: (empty page)
# {{.PageTitle}}

*This page's primary source document was removed.*"
{{end}}
8. Write in {{.Language}}.
</instructions>

Output the SUMMARY line first, then the updated Markdown content. Do not include any other preamble.`

// WikiIndexIntroPrompt generates the introduction for a NEW index page (first time only).
const WikiIndexIntroPrompt = `你是一名 wiki 编辑。请为一个 wiki 知识库索引页面撰写一段简短的介绍。

<document_summaries>
{{.DocumentSummaries}}
</document_summaries>

<instructions>
1. 写一行以 "# " 开头的标题，能体现该知识领域。
2. 接着用 2-3 句话，基于上方的文档摘要，描述这个 wiki 涵盖的内容。
3. 保持简洁——这只是头部部分，目录列表会另外单独添加在下方。
4. 使用 {{.Language}} 书写。
</instructions>

只输出标题和介绍段落。不要生成任何目录列表或页面链接。`

// WikiIndexIntroUpdatePrompt incrementally updates an existing index introduction.
const WikiIndexIntroUpdatePrompt = `你是一名 wiki 编辑。请更新一个 wiki 索引页面的介绍部分，以反映最近的变化。

<current_introduction>
{{.ExistingIntro}}
</current_introduction>

<changes>
{{.ChangeDescription}}
</changes>

<document_summaries>
{{.DocumentSummaries}}
</document_summaries>

<instructions>
1. 更新介绍内容，使其准确反映 wiki 当前的状态。
2. 如果新增了文档，且新主题显著改变了 wiki 的覆盖范围，请提及这些新主题。
3. 如果删除了文档，且相关主题已不再适用，请移除对这些主题的引用。
4. 保持与现有介绍相同的语气、风格和标题格式。
5. 保持简洁——1 行标题 + 2-3 句话。
6. 使用 {{.Language}} 书写。
</instructions>

只输出更新后的标题和介绍段落。不要生成任何目录列表或页面链接。`

// WikiLogEntryTemplate is a simple template for log entries (not LLM-generated).
const WikiLogEntryTemplate = `## [{{.Date}}] {{.Operation}} | {{.Title}}
- **来源**：{{.SourceInfo}}
- **涉及页面**：{{.PagesAffected}}
- **摘要**：{{.Summary}}
`

// WikiDeduplicationPrompt asks the LLM to identify duplicate entities/concepts
// between newly extracted items and existing wiki pages.
const WikiDeduplicationPrompt = `你是一个严格的去重系统。给定一份新抽取条目的列表和一份现有 wiki 页面的列表，判断哪些新条目与某个现有页面指向**完全相同**的现实世界实体或概念。

<new_items>
{{.NewItems}}
</new_items>

<existing_pages>
{{.ExistingPages}}
</existing_pages>

<instructions>
### 合并判定标准——必须同时满足：
1. 新条目与现有页面指向的是**同一个现实世界事物**（同一个人、同一个组织、同一个具体概念）。
2. 二者的匹配属于**名称变体**：缩写 ↔ 全称、译名，或轻微的拼写差异。
3. 类型必须兼容：entity 只能与 entity 合并，concept 只能与 concept 合并。**永远不要将一个 entity 合并进 concept，反之亦然。**

### 正确合并的示例：
- "Acme Corp" → "Acme Corporation"（同一家公司，缩写关系）
- "RAG" → "Retrieval-Augmented Generation"（同一概念，缩写关系）
- "苹果公司" → "Apple Inc."（同一实体，译名关系）

### 错误合并的示例——不要合并这些：
- "混元大模型" → "Qwen 大模型"（同一类别下相互竞争的产品是**不同**的实体，不要合并）
- "iPhone 15" → "华为 Mate 60"（同一类别下的不同具体型号）
- "GPT-4" → "GPT-3.5"（同一产品的不同版本是不同的实体）
- "AI 安全" → "内容审核机制"（相关主题，但是不同的概念）
- "运动员报名" → "学历核验"（两者都涉及核验，但完全属于不同领域）
- "赛事分组" → "年龄组别"（年龄组别只是分组的一个维度，不是同一个概念）
- "成绩标准" → "赛程轮次"（两者都与赛事相关，但是不同的概念）
- "机器学习" → "神经网络"（神经网络是机器学习的一个子集，不是同一个概念）
- "居民身份证" → "工作居住证"（两者都是政府颁发的证件，但完全是不同的证件）
- "驾驶证" → "行驶证"（两者都是与车辆相关的证件，但是不同的证件）
- "学位证" → "毕业证"（两者都是教育类证件，但是彼此不同）

### 关键原则："相关"不等于"相同"。两个条目的名称中共享少数几个字，或属于同一领域/文档系列/行业，都**不是**合并的理由。**绝对不要**仅因为属于同一类别，就合并不同的产品、不同的公司、不同的版本，或不同的证件/文档。如有疑虑，就**不要**合并。为同一事物保留两个独立页面，也远好于错误地将两个不同事物合并在一起。

返回一个包含 "merges" 映射的 JSON 对象。键是**新**条目的 slug，值是它应合并到的**现有**页面的 slug。只包含你高度确信是同一事物的条目。

如果没有任何条目与现有页面匹配，返回：{"merges": {}}

### JSON 格式规则
- **重要**：不要在 JSON 字符串值中使用字面换行符。如果字符串中需要换行，必须使用转义序列 \n。
</instructions>

只输出合法的 JSON。示例：
{"merges": {"entity/acme-corporation": "entity/acme-corp", "concept/rag": "concept/retrieval-augmented-generation"}}`

// Granularity guidance blocks injected into WikiCandidateSlugPrompt. The
// pipeline resolves a KnowledgeBase's configured granularity to one of these
// strings via WikiGranularityGuidance().
//
// The three levels form a spectrum from "only the document's main subjects"
// to "every named thing you see". Moving down the list monotonically
// increases the candidate slug count, the downstream chunk-citation cost,
// and the noise-to-signal ratio of the wiki index.
const (
	WikiGranularityGuidanceFocused = `**FOCUSED（聚焦）模式 —— 激进裁剪。**
只抽取该文档的核心主题：文档本质上"围绕"讲述的少数几个实体/概念。

应包含：
- 文档的主要主题——例如：对于一份简历，是这个人及其署名项目；对于一则公告，是发布公告的组织以及被公告的事件/产品；对于一个产品页面，是产品本身及其制造方/开发方。
- 实体和概念合计最多 3-7 个条目。

应排除（即使被明确点名提及）：
- 顺带提及的技术栈/库/框架（例如简历中列出的"Spring Boot、MySQL、Redis"——不要抽取这些）。
- 仅被引用的泛化概念和方法论（例如作为实现细节被提及的"微服务"、"异步处理"、"无状态认证"、"流式响应"）。
- 只作为背景提及的地点、学校或组织（例如简历所有者的母校，除非文档本身就是关于该学校的）。
- 任何内容不足以支撑超过一句话描述的条目。

如果你不确定某个条目是否应该被纳入，就**不要纳入**。一个干净、聚焦的索引比一个全面但嘈杂的索引更有价值。`

	WikiGranularityGuidanceStandard = `**STANDARD（标准）模式 —— 均衡（默认）。**
抽取文档的主要主题，以及被实质性讨论的实体/概念——即拥有专门段落、多个要点，或至少 2-3 句上下文说明的条目。

应包含：
- 文档的主要主题。
- 获得了具体内容篇幅（一个段落、一个多要点列表，或一个专门小节）的次要实体/概念。
- 当文档解释了主题**如何**使用某种方法论、架构或技术时（而不仅仅是提及其名称），应纳入这些具名的方法论、架构或技术。

应排除：
- 只出现在逗号分隔的技术列表中、没有任何进一步说明的条目（例如"技术栈：A、B、C、D"——除非 A/B/C/D 各自在其他地方也有专门段落，否则均不抽取）。
- 一次性的提及、括号内的引用，以及泛化的基础设施名词。
- 对文档的全部贡献都能用一句简短的话概括的条目。

目标是打造一个精炼、经过筛选的索引。对于边界模糊的条目，如有疑虑，优先**排除**。`

	WikiGranularityGuidanceExhaustive = `**EXHAUSTIVE（详尽）模式 —— 最大化召回。**
抽取每一个具名实体和每一个可识别的概念，包括即使只被点名提及一次的技术、工具、标准和方法论，只要它们是具体且知名的（不是像"数据库"或"函数"这样的泛化术语）。

应包含：
- 所有主要和次要主题。
- 所有具名的技术、库、框架、数据库、服务、协议或标准。
- 所有拥有广泛使用名称的可识别概念和方法论（例如 RAG、微服务、异步处理、SSE、JWT）。

仅排除：
- 真正泛化的术语（例如"服务器"、"函数"、"数据"）。
- 只出现在 URL 路径或引用文献中的条目。

当知识库的功能更像是技术术语表，而不是经过筛选的叙事型 wiki 时，使用此模式。`
)

// WikiGranularityGuidance returns the guidance text to inject into the
// WikiCandidateSlugPrompt template for the given granularity. Accepts the
// raw string value stored in WikiConfig.ExtractionGranularity; callers do
// NOT need to Normalize() first — unknown values fall through to standard.
func WikiGranularityGuidance(granularity string) string {
	switch granularity {
	case "focused":
		return WikiGranularityGuidanceFocused
	case "exhaustive":
		return WikiGranularityGuidanceExhaustive
	default:
		return WikiGranularityGuidanceStandard
	}
}
