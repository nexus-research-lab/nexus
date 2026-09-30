# 多模态模型上下文与图片输入限制

核对日期：2026-09-29。本文是按**精确 Provider + model ID**核对的研究记录（`non-normative`），用于产品设计和容量评估，不替代 Provider 合同、账户配额或 Nexus 运行时协议。不同云厂商、兼容接口和第三方路由即使使用相同模型名称，也可能有不同限制。

## 结论表

| Provider / 精确模型 | 上下文窗口 | 单次图片数量上限 | 图片大小、分辨率或请求体限制 | 官方依据 |
| --- | ---: | --- | --- | --- |
| 智谱 API：GLM-5.3-Flash 系列（例如 `glm-5.3-flashx`） | 1,000,000 tokens（1M） | **50 张** | 每张小于 5 MB；像素不超过 6000×6000；支持 JPG/PNG/JPEG；URL 或 Base64 | [模型页](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash) · [Chat Completion API](https://docs.bigmodel.cn/api-reference/%E6%A8%A1%E5%9E%8B-api/%E5%AF%B9%E8%AF%9D%E8%A1%A5%E5%85%A8.md) |
| Moonshot API：`kimi-k3` | 1,048,576 tokens（1M） | 官方 Vision 文档写明**没有图片数量限制** | 单次请求 Body 不超过 100 MB；图片按动态 token 计费；推荐分辨率不超过 4096×2160；当前 URL 图片不支持，使用 Base64 或文件 ID | [Vision 模型](https://platform.kimi.com/docs/guide/use-kimi-vision-model.md) · [模型定价与上下文窗口](https://platform.kimi.com/docs/pricing/chat.md) · [文件上传](https://platform.kimi.com/docs/api/files-upload.md) |
| Alibaba Cloud Model Studio：`qwen3.8-max` | 1,000,000 tokens；最大输入 991,808，最大输出 131,072（思考模式最大输入 983,616） | URL/本地路径最多 **2048 张**；Base64 最多 **250 张** | Qwen3.8 系列 URL 单图不超过 20 MB、本地路径不超过 10 MB；OpenAI 兼容或 DashScope Base64 原图不超过 20 MB、Data URI 不超过 20 MB；建议分辨率不超过 8K、长宽比不超过 200:1；Anthropic 兼容接口请求体整体不超过 6 MB | [模型页](https://help.aliyun.com/zh/model-studio/qwen3-8-max) · [视觉理解限制](https://help.aliyun.com/zh/model-studio/vision) |
| Alibaba Cloud Model Studio：`qwen3.7-plus` | 1,000,000 tokens；最大输入 991,808，最大输出 131,072（思考模式最大输入 983,616） | URL/本地路径最多 **2048 张**；Base64 最多 **250 张** | Qwen3.7 系列 URL 单图不超过 20 MB、本地路径不超过 10 MB；OpenAI 兼容或 DashScope Base64 原图不超过 20 MB、Data URI 不超过 20 MB；建议分辨率不超过 8K、长宽比不超过 200:1；Anthropic 兼容接口请求体整体不超过 6 MB | [模型页](https://help.aliyun.com/zh/model-studio/qwen3-7-plus) · [视觉理解限制](https://help.aliyun.com/zh/model-studio/vision) |

这里的“单次图片数量上限”只是接口允许的图片块数量。实际可发送数量还会被上下文 token、请求体大小、图片下载时间、RPM/TPM、账户额度和费用共同压低。

## 逐项证据

### GLM-5.3-Flash

智谱模型页把上下文窗口列为 `1M`，并在参数说明中写明支持 1M 上下文。Chat Completion API 的视觉参数原文明确写出：图片 URL 或 Base64 每张图像限制在 5 MB 以下、像素不超过 6000×6000，并且 **GLM-5.3-Flash 系列限制 50 张**。这就是“50 张”的官方来源，不是从其他 GLM 版本推断出来的。

官方原文对应的限制句是：`图像大小上传限制为每张图像 5M 以下，且像素不超过 6000*6000……GLM-5.3-Flash 系列限制 50 张。`

### Kimi K3

Moonshot 的价格页对精确模型 ID `kimi-k3` 列出 `1,048,576 tokens` 上下文窗口。Vision 文档列出 `kimi-k3`，并在“功能支持与限制”中写明 Vision 模型没有图片数量限制，同时要求请求 Body 不超过 100 MB。它还说明图片 token 随分辨率动态计算，推荐图片不超过 4K。这里的“没有图片数量限制”不能理解为可以无条件发送 10,000 张：100 MB Body 和 1M token 仍是硬约束。

官方原文是：`图片数量：Vision 模型没有图片数量限制，但请确保请求的 Body 大小不超过 100M`。

如果要重复引用图片，Moonshot 建议使用文件上传和文件 ID。文件接口当前单文件上限为 100 MiB，文件数量不再单独限制，组织默认总存储量为 10 GiB；这属于文件存储限制，不是单次视觉请求可以容纳的图片数量。

### Qwen `qwen3.8-max` 与 `qwen3.7-plus`

这两个模型页都把输入模态列为 Image、Text、Video，输出模态为 Text，并给出相同的百万级上下文表：上下文长度 1,000,000，最大输入长度 991,808，最大输出长度 131,072；思考模式最大输入长度为 983,616。阿里视觉理解总页明确列出：Qwen3.8-Max、Qwen3.8-Flash、Qwen3.7-Plus 以公网 URL 或本地路径传入时最多 2048 张图片；Base64 传入最多 250 张。

官方原文对应的两条限制是：`Qwen3.8-Max、Qwen3.8-Flash、Qwen3.7-Plus：最多 2048 张`，以及 `以 Base64 编码传入时：最多 250 张`。可直接打开[视觉理解限制页的图像限制段落](https://help.aliyun.com/zh/model-studio/vision#图像限制)查看。

同一页还区分了接口：Qwen3.8/Qwen3.7 系列普通公网 URL 单图不超过 20 MB，本地路径单图不超过 10 MB；OpenAI 兼容或 DashScope 的 Base64 原图不超过 20 MB、编码后的 Data URI 不超过 20 MB；Anthropic 兼容接口的整体请求体不超过 6 MB，多图共享这 6 MB。所有图片和文本的总 token 必须小于模型的最大输入。

视觉理解页还要求原图宽高比不超过 `200:1`，推荐将图片分辨率控制在 8K（7680×4320）以内。这里的 6 MB 只适用于 Anthropic 兼容接口的 Base64 整体请求体，不能当作 DashScope 原生或 OpenAI 兼容接口的通用上限。

## Nexus 当前实现的实际闸门

即使 Provider 允许更多图片，当前 Nexus 对普通对话还有本地限制：

- Composer 最多保留 6 个附件，见 [`composer-local-attachment-model.ts`](../../web/src/features/conversation/shared/composer/attachments/composer-local-attachment-model.ts) 的 `MAX_COMPOSER_ATTACHMENTS` 和追加时的 `slice(0, MAX_COMPOSER_ATTACHMENTS)`。
- Composer 单个附件的浏览器检查上限为 20 MiB，见 [`composer-attachments.ts`](../../web/src/features/conversation/shared/composer/attachments/composer-attachments.ts)。这是前端附件限制，不是 Provider 的视觉 API 限制。
- Runtime 每次提交最多展开 100 个图片 block，每张图片的 Base64 编码后最多 5 MiB，见 [`attachment.go`](../../internal/service/conversation/attachment.go)。Base64 会膨胀数据，5 MiB 编码上限对应的原始图片通常要低于约 3.75 MiB；还要再满足 Provider 的请求体限制。
- 仅把本机绝对路径写入 prompt 不能让远端模型读取图片。Runtime 必须先读取工作区文件，再按 Provider 协议生成图片内容块。

因此，Nexus 当前普通 Composer 不能直接把 10,000 张图片作为一条消息发送；即使绕过前端，Runtime 的 100 个图片块和各 Provider 的请求体/token 上限也会先拦截它。

## 10,000 张照片的推荐产品方案

不要把 10,000 张照片建模成 10,000 个聊天附件。更适合的链路是：

1. **持久化原图**：将原图放在 owner/Agent 工作区的图库目录（例如 `photos/`），把它作为事实来源。不要把原图只放在可清理的缓存目录。
2. **异步建索引**：为每张照片保存稳定 asset ID、内容哈希、尺寸、时间、来源和可选的 OCR/标签；用感知哈希去重，用图像 embedding 建立语义检索索引。
3. **缓存可重建结果**：缩略图、OCR 结果、embedding 和候选标签放缓存或索引目录，原图删除或替换时按哈希失效并重建。
4. **先检索后复核**：用户说“找猫猫狗狗照片”时，先在本地索引中召回 Top-K（建议先取 20～50 张缩略图或候选图），再把这批候选交给视觉模型做确认、排序或去重。
5. **返回资产引用**：模型输出稳定 asset ID、文件名和工作区链接；只有用户要查看或导出时才取原图。Kimi 需要多次复用时，可以把精选结果上传一次并使用 file ID。

Composer 可以提供“导入图库 / 建立索引 / 搜索图库”的入口，而不是让用户在一个对话框里添加 10,000 个附件。这样既避开单次请求上限，也能让搜索结果可解释、可复用和可增量更新。

## 适用范围

本文记录的是上述官方 API 文档在核对日期的限制。OpenRouter、阿里云其他产品、火山引擎、ModelScope 或自建推理服务可能重新定义模型 ID、图片协议、上下文窗口和请求体上限；接入 Nexus 时必须按实际 Provider 配置与精确 model ID 重新核验，不能把本表数值跨路由继承。
