# Composer 人工介入

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-interaction-model.ts`：按 request 首次出现顺序收敛 DM/Room pending 队列，并区分权限、计划确认和结构化问答。
- `composer-interaction-surface.tsx`：pending 期间原位替换输入壳内容，一次只处理一个请求，并为结构化问答补充当前 Agent 身份。
- `composer-permission-model.ts`：把 runtime 权限建议翻译为 Nexus 规则动作、匹配内容与生效范围提示，不推断或扩张权限规则。
- `composer-permission-surface.tsx`：以 Agent、工具、人话摘要、必要参数和单一决策行展示权限/计划；普通拒绝动作复用 `UiButton`，必要密钥复用 `UiInput`，runtime 提供的持久权限建议只通过共享 `UiSplitButton` 进入“允许本次”旁的次级菜单。

人工介入与普通输入是同一 Composer 位置的互斥状态，禁止叠成输入壳上方浮层，也禁止在 DM/Room 消息正文或 Thread 保留第二个操作入口。请求切换使用 `request_id` 重置本地表单和提交保护；发送失败必须保留当前请求以便重试，发送成功后由 runtime pending 真相推进下一项。Room 并行请求继续按首次到达顺序排队，每项显示自己的 Agent 头像与名称，并使用 `request_id` 路由回原执行。
结构化问答与权限确认共用 Agent/工具元信息节奏和底部决策行；Composer 壳是唯一边界，问题与选项不得再叠加 rail、footer card 或逐项阴影。
权限确认默认不得复用完整 ToolBlock 的状态徽标、时间、提示词分区和权限范围平铺；首屏只呈现决策所需信息，低频范围选择收进明确的次级菜单。主动作与菜单触发器的组合几何由 `UiSplitButton` 唯一持有，业务不得再写颜色、边框、圆角、focus 或移动端高度配方；动作名、匹配规则和保存范围必须投影 Nexus runtime 的真实权限体系，禁止复制其它产品的权限文案。
持久权限规则必须原样来自 runtime 的 `suggestions`：有建议时显示相邻下拉并随允许响应回传；没有建议时只能允许本次，禁止 Composer 猜测工具匹配规则或伪造 `localSettings` 更新。
