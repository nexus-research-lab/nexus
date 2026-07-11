# Agent Options 技能域

- `use-agent-skills-resource.ts` 负责 Agent 作用域列表、请求取消和可见时刷新。
- `use-agent-skills-controller.ts` 负责安装/移除互斥命令和确认状态。
- `agent-skills-model.ts` 只处理安装分组与搜索投影。
- `agent-options-skills-view.tsx`、`agent-skill-card.tsx` 只渲染窄接口。

列表与命令结果必须绑定 Agent；旧请求、旧命令不得写入新作用域，页面卸载后不得继续刷新视图状态。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
