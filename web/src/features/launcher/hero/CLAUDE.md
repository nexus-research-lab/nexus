# Launcher Hero

- `launcher-hero-stage.tsx` 只渲染首屏、输入框和最近入口。
- `launcher-recent-entry-model.ts` 统一投影 DM/Room 标签、颜色、标记和动画延迟。
- `launcher-recent-entries.tsx` 只渲染最近入口与主 Agent 交接动作。
- `use-launcher-query-input.ts` 拥有受控输入、IME、Mention 和提交交互。
- `pile/` 独立拥有 Agent Pile 的描述表、Matter 生命周期和 Token 视图，不回流 Console。
- Surface Theme 只投影 CSS 变量。

Hero 不直接调用 Launcher、Room 或 Agent API。输入匹配和插入复用 `shared/ui/mention/`，本目录只决定触发符对应的目标分类。
