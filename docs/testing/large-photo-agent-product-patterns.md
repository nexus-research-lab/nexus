# 万张照片与智能体产品实现模式

核对日期：2026-09-29。本文是 `non-normative` 调研记录，用来回答“产品能不能接收 10,000 张照片，以及通常怎样让智能体从中找到照片”。它把**图库/知识库容量**和**单次模型请求的图片数量**分开；两者不是同一个指标。

## 先给结论

- 公开资料能证明的成熟做法是：把照片作为持久化资产导入图库或知识库，后台异步建立缩略图、元数据、OCR、人脸/对象标签和向量索引；查询时先检索候选，再把少量候选交给视觉模型复核。
- Google Photos、Apple Photos、Immich、PhotoPrism 这类产品可以管理万张级甚至更大的照片库，但官方通常不承诺一个固定的“最多 10,000 张”数字，实际边界由存储配额、数据库、索引任务和设备资源决定。
- NotebookLM、Dify 这类智能体/知识库产品也采用“摄取→索引→检索→生成”，但它们的来源或上传额度通常远低于 10,000 个独立照片，不能把文档来源上限当成图片模型上限。
- 没有找到任何官方资料证明这些产品会把 10,000 张原图拼成一次多模态模型请求。对“找猫猫狗狗照片”的请求，正确的产品语义应是“搜索图库并返回资产”，不是“把 10,000 张照片附加到聊天消息”。

## 产品调研

| 产品/类别 | 能否作为万张级资产库 | 是否把 10,000 张一次发给模型 | 已核实的实现方式与边界 |
| --- | --- | --- | --- |
| Google Photos / Ask Photos | **图库形态支持万张级**；官方帮助明确一个相册最多 20,000 张照片或视频，账号整体仍受存储配额和单文件规则约束 | **未证明，按产品形态不是这样做** | Google Photos 按 people、things、places 等维度搜索；Ask Photos 面向已有照片库提问。原图持续进入 library，查询时从索引召回。来源：[查找照片](https://support.google.com/photos/answer/6193313)、[搜索照片](https://support.google.com/photos/answer/6128843)、[相册数量限制](https://support.google.com/photos/answer/6128849)、[存储限制](https://support.google.com/photos/answer/10100180) |
| Apple Photos / iCloud Photos | **图库形态支持大图库**；容量主要受 iCloud 套餐、设备空间和索引资源约束，未公布固定 10,000 张上限 | **未证明，按产品形态不是这样做** | 支持按标题、描述、关键词、日期、地点、人物/宠物、照片中文字和自然语言描述搜索。来源：[iCloud Photos](https://support.apple.com/en-us/108782)、[搜索照片和视频](https://support.apple.com/guide/photos/search-for-photos-and-videos-pht64de33e5a/mac) |
| Immich | **自托管，能否到 10,000 张取决于磁盘、Postgres 和 CPU/GPU**；官方未给固定照片条数上限 | **否；官方资料指向索引和检索** | Postgres 保存 metadata 与 contextual CLIP search；VectorChord/CLIP 做语义搜索。人脸流程是 preview → 检测 → crop/embedding → 数据库索引，并支持后台增量任务。来源：[Searching](https://docs.immich.app/features/searching/)、[人脸识别](https://docs.immich.app/features/facial-recognition/) |
| PhotoPrism | **自托管，能否到 10,000 张取决于文件库、数据库和索引资源**；官方未给固定照片条数上限 | **否；官方资料指向索引和检索** | 对 originals 做手动、定时或自动 indexing，生成 metadata/缩略图；搜索支持 people、places、labels、folders、date 等；视觉管线生成 face embedding 并聚类。来源：[索引原图](https://docs.photoprism.app/user-guide/library/originals/)、[搜索](https://docs.photoprism.app/user-guide/search/)、[人脸识别](https://docs.photoprism.app/developer-guide/vision/face-recognition/) |
| Google NotebookLM / Gemini Notebook | **不适合作为 10,000 张独立图片图库**；来源数按计划为几十到数百级，个人/Workspace 方案页面最高约 500–600 sources/notebook，单来源有大小/文字限制 | **否** | 多来源提问时先检索最相关来源，再生成回答；支持 PDF、网页、YouTube、音频、Docs、Slides 等，独立照片不是其公开的 10,000 来源方案。来源：[个人方案限额](https://support.google.com/gemininotebook/answer/16213268)、[Workspace 限额](https://support.google.com/gemininotebook/answer/16337734)、[来源限制](https://support.google.com/notebooklm/answer/16269187?hl=en) |
| Claude Projects | **不能据公开资料确认 10,000 个独立照片文件**；它是项目知识库，付费计划会在接近上下文限制时使用 RAG，官方描述容量可扩展到原上下文的约 10 倍 | **否** | 官方 RAG 说明明确说使用 project knowledge search，只取回答所需信息，而不是一次把全部项目内容装入内存；文件上传页支持图片，但没有公开“10,000 张项目图片”承诺。来源：[Projects](https://support.anthropic.com/en/articles/9517075-what-are-projects)、[项目 RAG](https://support.claude.com/en/articles/11473015-retrieval-augmented-generation-rag-for-projects)、[可上传文件类型](https://support.anthropic.com/en/articles/8241126-what-kinds-of-files-can-i-upload-to-claude) |
| Dify Knowledge | **不宜直接视为 10,000 张照片库**；Cloud 计划有存储和单次上传限制，且图片常作为文档 chunk 的附件处理 | **否** | 导入后做解析、Vector/Full-text/Hybrid indexing，可选 rerank，默认检索 Top-K；启用视觉 embedding 时才对图片建立向量。来源：[导入数据](https://docs.dify.ai/en/cloud/use-dify/knowledge/create-knowledge/import-text-data/readme)、[存储限制](https://docs.dify.ai/en/cloud/use-dify/knowledge/knowledge-storage-limit)、[索引方式](https://docs.dify.ai/en/cloud/use-dify/knowledge/create-knowledge/setting-indexing-methods) |
| Amazon Bedrock Knowledge Bases | **企业知识库可做大规模摄取**；官方支持多模态数据，解析/存储/索引能力由数据源和账户资源决定，不等价于单轮 10,000 图片输入 | **否** | Managed Knowledge Base 负责 ingestion、向量索引和 retrieval；图片/音频/视频的多模态文件通过 S3 或 custom data source 支持，新增/修改/删除可增量同步，模型只收到检索结果。来源：[Knowledge Bases](https://docs.aws.amazon.com/bedrock/latest/userguide/knowledge-base.html)、[多模态数据](https://docs.aws.amazon.com/bedrock/latest/userguide/kb-how-data.html)、[高级解析](https://docs.aws.amazon.com/bedrock/latest/userguide/kb-advanced-parsing.html)、[增量同步](https://docs.aws.amazon.com/bedrock/latest/userguide/kb-data-source-sync-ingest.html) |

表中的“能否”描述的是**资产库/索引系统的产品形态**，不是厂商对“10,000 张”这个数字作出的 SLA。没有公开硬上限时，应标为“万张级可行、固定上限未公开”，不要写成“官方保证 10,000 张”。

Google Photos 是本次调研中最接近“图库加智能体”的公开案例。Ask Photos 的官方说明把它描述为 Gemini 驱动的实验功能：简单问题先展示相关结果，复杂问题在后台继续缩小并突出最相关结果；Google Photos 搜索本身按人物/宠物、地点、日期、描述等索引，结果先给最相关和最近的照片，再允许查看全部。另一个官方说明明确，Gemini 连接 Google Photos 时，搜索结果由 Google Photos 提供，点击结果后回到 Photos 打开原图、视频或相册。这些描述说明模型是在图库检索结果上工作，而不是把全库像素拼入一次 prompt。来源：[Ask Photos](https://support.google.com/photos/answer/15318661)、[Google Photos 搜索](https://support.google.com/photos/answer/15235862)、[Gemini 连接 Google Photos](https://support.google.com/photos/answer/17597604)。

## 这些产品通常怎么做

### 1. 原图和派生数据分层

原图进入持久化图库、工作区或对象存储，拥有稳定的 `asset_id` 和内容哈希。缩略图、OCR、标签、embedding、聚类和检索倒排表是可重建的派生数据，放在索引/缓存层，并记录生成它们的模型版本。这样清理缓存不会丢失照片事实，替换或删除原图时可以按哈希精确失效。

### 2. 导入是后台任务，不阻塞聊天

批量上传通常先落盘并返回导入任务，然后按队列处理：校验格式和哈希、生成缩略图、读 EXIF、做 OCR/人脸/对象识别、写 metadata，再批量写入向量索引。任务应可重试、可暂停、可恢复；新照片只增量处理，不重建整个图库。

### 3. 查询使用混合检索

用户输入“找猫猫狗狗照片”时，系统先把自然语言拆成可解释的条件，例如对象、时间、地点、人物、相册和排序偏好，然后组合：

- metadata/倒排过滤：日期、地点、文件名、相册、人物、OCR、标签；
- embedding 相似度：猫、狗、宠物、海边等语义概念；
- 感知哈希/聚类：去重、相似照片和连拍归并。

先召回候选集，再按相关性、质量、去重和权限排序。只有需要视觉确认时，才把 Top-K 缩略图或原图交给视觉模型。常见的工程起点是先取 20–50 张候选；候选多时分页或按相册/日期分组，而不是扩大上下文。

### 4. 精确模式：索引负责召回，逐图核验负责结论

仅靠 embedding 或文字搜图接口，可能漏掉小目标、遮挡目标、复杂关系和否定条件。需要高准确率时，可以把图片搜索接口定位为**第一阶段召回器**，而不是最终裁决器：

1. 原图先进入工作区的持久目录或对象存储，例如 `workspace/photos/`，登记 `asset_id`、哈希、权限和原始元数据。
2. 导入后台为每张图片生成缩略图、OCR、通用标签、对象/人脸结果、caption 和 image embedding。每个结果保存模型版本、置信度和处理状态。
3. 用户查询时，调用文字搜图片或向量搜索接口召回一批候选，例如 100–500 张，再做 metadata 过滤、去重和排序。
4. 对候选逐张调用视觉模型核验，并输出固定结构，例如 `cat=true`、`dog=false`、`contains_both=false`、`confidence`、`evidence`。只有通过核验的图片进入最终结果。
5. 对低置信度、冲突标签或用户纠正的图片进入补标队列；新模型版本只增量重处理受影响的资产。

如果“特别准确”是核心卖点，第四步不应等到用户每次搜索才开始。更好的做法是在首次导入时异步完成逐图标注，搜索时使用“标签/对象结果 + embedding + 视觉复核”的组合。这样 10,000 张照片只需要被逐图处理一次或按版本重处理，查询延迟不会随着图库规模线性增长。

### 5. 模型输出引用，不复制整库

模型返回 `asset_id`、文件名、缩略图链接、所在相册和可选的解释。用户点击预览、下载或导出时，系统再读取原图。这样结果可以复用、审计和撤销，也不会把本机路径直接暴露给远端模型。

## 对云盘型智能体项目的建议

Nexus 的“10,000 张照片”应建模为**图库/工作区资产导入功能**，而不是 Composer 的 10,000 个附件。建议产品链路如下：

```text
导入 workspace/photos 或对象存储
  → 持久化 asset_id、内容哈希、权限和原始元数据
  → durable background job：缩略图、EXIF/OCR、标签、人脸/对象 embedding
  → SQLite/Postgres metadata + vector index
  → 用户自然语言查询
  → metadata + vector hybrid Top-K
  → 视觉模型复核/排序少量候选
  → 返回资产引用、缩略图和可操作链接
```

具体边界：

- **原图放 workspace/photos 等持久目录**，不要只放可清理的缓存目录；缓存目录只保存缩略图、OCR、embedding 等可重建结果。
- **聊天消息只携带搜索意图和结果引用**。只有用户明确要求查看或复核时，才将少量候选转换成 Provider 所需的图片内容块。
- **导入、索引、搜索、模型复核分成独立任务**，每一步有状态、进度、失败原因和重试边界；不能让一次聊天请求同步扫描 10,000 个文件。
- **按权限过滤后再做向量检索**，避免先召回再补权限；删除、撤权、替换原图时同时失效 asset、缩略图和向量索引。
- **结果要支持分页和批量操作**。用户要“所有猫照片”时返回一个可继续浏览的搜索结果集或相册，而不是生成一个包含 10,000 个图片块的模型请求。

云盘产品可以把“上传附件”和“导入图库”做成两个入口：少量图片继续走即时对话附件，超过阈值就切换为图库导入任务，并在对话中返回导入进度和搜索结果引用。Provider 的上下文、图片数量、请求体和费用仍会进一步收紧单轮复核上限；图库可以是万张级，单轮模型复核仍应保持小候选集。

## 证据和表述边界

- “官方未公布固定上限”不等于“无限”；应继续用存储、数据库、索引吞吐和账户配额做容量压测。
- “支持多模态知识库”不等于“支持把一万张独立 JPEG 作为一条 prompt”；要分别核对数据源格式、批量摄取上限、索引规模和单次检索/模型输入上限。
- 第三方路由可能改变同一模型的图片协议和请求限制。Nexus 接入时仍必须按精确 Provider、model ID 和传输方式重新核验。
