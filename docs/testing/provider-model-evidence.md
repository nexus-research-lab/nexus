# Provider 模型官方资料核对记录

核对日期：2026-09-17；`qwen-vl-plus`、`glm-5.3-flashx` 补充核对：2026-09-23。此文件是研究证据与维护记录，不替代
[当前合同](../specs/provider-model-guidance-spec.md)。可执行的精确 ID、能力和推荐策略
以 `internal/service/provider/model_advice_catalog.go` 为准。

## 服务与套餐矩阵

| Nexus 来源 | 已核对的官方接口/范围 | 推荐与能力依据 | 限制 |
| --- | --- | --- | --- |
| OpenAI | `api.openai.com/v1`；Chat/Responses 与 Images 分开 | [模型目录](https://platform.openai.com/docs/models)、[Astra](https://platform.openai.com/docs/models/gpt-6-astra)：Astra 复杂任务优先，Terra 平衡，Luna 轻量；均文本/图片输入、文本输出 | 能调用生图工具不等于模型 ID 可直接调用 Images |
| Anthropic | Claude Messages API | [官方完整模型表](https://docs.anthropic.com/en/docs/about-claude/models/overview.md)：Opus 5 默认建议，Sonnet 5 平衡；Fable 5.1 更高要求任务；当前模型支持文字/图片输入和文字输出 | 不继承 Bedrock/Vertex/Foundry 的 ID 或部署权限 |
| DeepSeek | `api.deepseek.com`，Anthropic 兼容入口 `/anthropic` | [模型与价格表](https://api-docs.deepseek.com/quick_start/pricing)：`deepseek-flash` 对应 V4.1-Flash，支持视觉；`deepseek-v4-pro` 明确不支持视觉 | 不再推荐旧 `deepseek-chat`；不能把官方 Flash 新别名能力外推到托管平台的 V4-Flash |
| GLM Coding Plan | `/api/anthropic`；`/api/coding/paas/v4` | [套餐](https://docs.bigmodel.cn/cn/coding-plan/overview)、[切换指南](https://docs.bigmodel.cn/cn/coding-plan/latest-model)、[GLM-5.3](https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3)：5.3 旗舰纯文本，5.3-Flash 原生图片输入 | 5.1/5.2 路由到 5.3，旧 ID 不作首选；视觉 MCP 不等于原生视觉。套餐工具范围及额度仍受官方约束 |
| Qwen Token Plan | 北京 **团队版** `token-plan.cn-beijing.maas.aliyuncs.com`，分别 `/apps/anthropic`、`/compatible-mode/v1` | [快速开始](https://help.aliyun.com/zh/model-studio/token-plan-team-quickstart)、[完整模型能力表](https://help.aliyun.com/zh/model-studio/token-plan-team-overview)：Qwen3.8-Max 高能力、Qwen3.8-Flash 轻量；3.7-Plus/3.6-Plus/3.6-Flash、DeepSeek-V4.1-Flash、Kimi-K2.5/2.6/2.7-Code 支持视觉；GLM5/5.1/5.2/5.3、MiniMax-M2.5、DeepSeek V4-Pro/V4-Flash 在表中为文本模型 | 与旧 [Coding Plan](https://help.aliyun.com/zh/model-studio/coding-plan) 的 `coding.dashscope.aliyuncs.com` 完全隔离，不共用 Key/清单。套餐含生图不代表当前聊天端点可生图；不自动转到按量付费 |
| MiniMax Token Plan | 官方新指南给出 `api.minimax.cn/v1`、`/anthropic`；Nexus 已有预设仍为 `api.minimaxi.com`，本次不改已保存地址 | [接入指南](https://platform.minimaxi.com/docs/token-plan/other-tools)、[模型概览](https://platform.minimaxi.com/docs/guides/text-generation)、[Anthropic 字段支持](https://platform.minimaxi.com/docs/api-reference/text-anthropic-api)、[OpenAI 多模态输入](https://platform.minimaxi.com/docs/api-reference/text-openai-api)：M3 原生图片/视频输入；M2/M2.1/M2.5/M2.7 明确不支持图片、视频 | [Token Plan](https://platform.minimaxi.com/docs/coding-plan/intro) 含多种模型，不能将套餐内独立生图/语音权益算到每个 M 模型。旧域名实际可用性未做凭据调用验证 |
| Kimi Code | `api.kimi.com/coding/v1`、`api.kimi.com/coding/` | [精确 ID/版本/模态/会员表](https://www.kimi.com/code/docs/kimi-code/models.html)：`kimi-for-coding` 现在为 K2.8 Preview；`k3`/`k3-256k` 是 K3；全部支持图片，256k 版不支持视频 | 标准模型所有会员可用；K3 需 Moderato+，1M 需 Allegretto+；highspeed 需 Allegretto+。不套用 Moonshot 按量 API ID |
| Volcengine Coding Plan | `ark.cn-beijing.volces.com/api/coding`、`/api/coding/v3` | [接入 ID](https://docs.volcengine.com/docs/82379/1928261?lang=zh)、[套餐模型表](https://docs.volcengine.com/docs/82379/1925114?lang=zh)：GLM-5.3-Flash、MiniMax-M3 原生视觉，适合日常；GLM-5.3 重难点；Seed2.1-Turbo/2.0-Lite、Kimi-K2.7-Code/K3 支持视觉 | K3 官方仅建议 Pro；GLM5.3/DeepSeek Pro 消耗高。`ark-code-latest` 是控制台可变路由，不固定标模态。此处 DeepSeek V4-Flash 未明确视觉，不继承原厂新 Flash 能力 |
| Doubao 按量 API | `ark.cn-beijing.volces.com/api/v3`，Chat/Responses/Images | [模型清单](https://docs.volcengine.com/docs/82379/1330310?lang=zh)：`doubao-seed-2-1-pro-260915` 当前推荐 Pro；`doubao-seed-2-1-turbo-260628` 平衡；Evolving 迭代别名；均列入视觉能力表 | 与 Coding Plan 的点号别名、额度和地址分开；Endpoint ID 不能由名称推断对应模型 |
| DashScope 按量 API | 百炼按量 Key 与 endpoint | [模型目录](https://help.aliyun.com/zh/model-studio/getting-started/models)、[文本选型](https://help.aliyun.com/zh/model-studio/text-generation-model)、[视觉 API](https://help.aliyun.com/zh/model-studio/vision)：Qwen3.8-Max 推理优先，Qwen3.7-Plus 日常平衡且支持图片输入；[qwen-vl-plus](https://help.aliyun.com/zh/model-studio/qwen-vl-plus) 支持图片输入、文本输出，不支持 Function Calling | 地域/业务空间影响可用性；目录内第三方命名空间不能删除后按原厂 ID 匹配 |
| ModelScope | `api-inference.modelscope.cn/v1`；AIGC 独立接口 | [API 推理介绍](https://modelscope.cn/docs/model-service/API-Inference/intro)：精确示例 `Qwen/Qwen3.5-35B-A3B` 支持图片输入；`Qwen/Qwen-Image` 图片生成 | 官方明确示例模型可上下线，因此只补能力，不据示例评选“最新/最强”；命名空间必须完整。当前 Nexus adapter 未实现编辑 |
| Azure | 资源专属 endpoint、部署 ID | [官方模型与地域](https://learn.microsoft.com/en-us/azure/ai-foundry/openai/concepts/models)、[部署名规则](https://learn.microsoft.com/en-us/azure/ai-foundry/openai/how-to/switching-endpoints) | `model` 是任意部署名；精确同名可补能力默认值，但不是底层版本证明，不继承 OpenAI 推荐/套餐提示；以部署返回的能力或用户覆盖为准 |
| Custom | 用户指定地址 | 精确模型 ID 的无冲突目录能力默认值、新拉取模型卡的显式能力、用户覆盖 | 名称相同不证明实际模型相同；冲突的目录能力保持未知，不按模型族或命名空间猜测，也不继承 Provider 专属推荐/套餐提示 |

## FlashX 模型卡补充

`glm-5.3-flashx` 按[官方模型文档](https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash)
补充视觉输入、文本输出、推理与工具调用能力；上下文按现有 GLM 目录的 1,000,000 token 基线，
最大输出按 [API 参数上限](https://docs.z.ai/api-reference/llm/chat-completion)设为 131,072。
官方明确 FlashX 暂未开放 Coding Plan，因此该条目仅用于精确名称能力默认值，不关联套餐推荐。
思考模式不可关闭；模型卡的 reasoning 标记只表示支持推理，不表示可以关闭思考。

## 图片输出与调用协议

- [OpenAI Images 指南](https://platform.openai.com/docs/guides/image-generation)：
  `gpt-image-2.5-flare` 面向日常生成，`gpt-image-2.5-sunburst` 面向精细编辑；
  两者直接用于 Images。Astra 的 image generation **tool** 不能替代这个模型 ID。
- [百炼图片选型](https://help.aliyun.com/zh/model-studio/image-model)：
  `wan2.7-image-pro` 支持生成与多图参考编辑；`qwen-image-3.0-pro` 是官方当前通用推荐。
  本次保留已有 Wan transport 对应推荐；Qwen 3.0 的默认尺寸等请求参数未完成 adapter 核对，
  不以官方选型页面单独宣称 Nexus 已完成该接口适配。
- 豆包官方模型表推荐 `doubao-seedream-5-0-pro-260628`，支持文生图和参考图生图；
  `doubao-seedream-5-0-260128` 与 `doubao-seedream-5-0-lite-260128` 是精确别名。
  模型具备编辑能力，但 Nexus 通用 OpenAI 编辑分支使用 `/images/edits` multipart；
  不将其等同于 Seedream 的图片输入请求，故本次编辑资格关闭，保留真实能力标注。
- Token Plan 团队版的[多模态接口](https://help.aliyun.com/zh/model-studio/token-plan-multimodal-gen)
  与文字入口分开。本次不新增该 transport；列出的图片模型不会进入聊天或生图可选项。

## 证据与验证边界

官方正文由直接 HTTP 文档读取；火山与魔搭使用独立后台浏览器读取 JS 渲染正文。
搜索摘要只用于定位入口，不作为能力结论。以上是 2026-09-17 的资料快照，非账号实际授权证明，
也不代表所有模型都完成了真实付费调用。未验证模型保持未知、不打推荐，仍可手动配置。

维护时同时核对：精确 preset/endpoint、精确 model ID、同名条目能力冲突、输入/输出模态、独立工具、套餐档位、
生命周期与 alias、Nexus 已实现 transport。修改数据必须更新来源、核对日期和 catalog version，
并运行作用域、用户覆盖、旧记录来源、未知版本、图片协议资格与默认选择回归。
