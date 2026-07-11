# Shared Mention

本目录拥有跨 Feature 的 Mention 文本匹配、插入和目标选择视图，不解释 Agent、Room 或消息协议。

- `mention-target-model.ts` 保存触发符、匹配、插入、筛选、键盘命令和浮层定位纯规则。
- `mention-target-popover.tsx` 只渲染消费者提供的标签、说明和标记。

目标分类与标记由消费者投影。共享视图不得根据业务类型决定图标、字符或筛选范围。
