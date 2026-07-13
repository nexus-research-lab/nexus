# Capability API

- 每个能力使用独立协议文件，摘要只归 `summary-api.ts`。
- 定时任务、频道、连接器、技能和 Loop 的转换规则与请求参数留在各自文件。
- Agent workspace、会话和设置接口不得下沉到能力目录。
