# Agent Options 技能域

- `use-agent-skills-resource.ts` 负责 Agent 作用域列表、请求取消和可见时刷新；前台与后台刷新使用显式模式。
- `use-agent-skills-controller.ts` 负责安装/移除互斥命令、Agent 作用域失效和确认状态；命令回调只创建命令，执行与收尾按阶段处理。
- 资源加载态合并、请求过期判断与命令展示态均由纯函数投影，Hook 只编排生命周期。
- `agent-skills-model.ts` 只处理安装分组与搜索投影。
- `agent-options-skills-view.tsx` 只组合页头、错误提示、内容与确认弹窗，`agent-options-skills-content.tsx` 分别渲染状态、已安装列表和可添加列表，`agent-skill-card.tsx` 只渲染单项。

列表与命令结果必须绑定 Agent；旧请求、旧命令不得写入新作用域，页面卸载后不得继续刷新视图状态。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
