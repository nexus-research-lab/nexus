# Shared Mention

本目录拥有跨 Feature 的 Mention 文本匹配、插入和目标选择视图，不解释 Agent、Room 或消息协议。

- `mention-target-model.ts` 保存触发符、匹配、插入、筛选、键盘命令和浮层定位纯规则。
- `mention-target-popover.tsx` 只渲染消费者提供的标签、说明和标记。
- 全局键盘捕获只在真实锚点和可见候选均存在时注册；隐藏/关闭后立即释放，不能截获其他菜单或编辑器的方向键、Enter、Tab 与 Escape。共置 `mention-target-popover.test.tsx` 覆盖隐藏、打开、选值和关闭后的完整生命周期。

目标分类与标记由消费者投影。共享视图不得根据业务类型决定图标、字符或筛选范围。
