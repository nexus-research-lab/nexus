# Composer 人工介入

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-interaction-model.ts`：按 request 首次出现顺序收敛 pending 队列，并区分权限、计划确认和结构化问答。
- `composer-interaction-surface.tsx`：pending 期间原位替换 DM 输入壳内容，一次只处理一个请求。

人工介入与普通输入是同一 Composer 位置的互斥状态，禁止叠成输入壳上方浮层，也禁止在 DM 消息正文保留第二个操作入口。请求切换使用 `request_id` 重置本地表单和提交保护；发送失败必须保留当前请求以便重试，发送成功后由 runtime pending 真相推进下一项。Room 的并行请求仍归各 Agent 消息轨道，不消费本组件。
