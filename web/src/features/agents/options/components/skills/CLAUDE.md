# Agent Options 技能域

- `use-agent-skills-resource.ts` 负责 Agent 作用域列表、请求取消和可见时刷新；前台与后台刷新使用显式模式。
- `use-agent-skills-controller.ts` 负责 Agent 作用域的启用/停用互斥命令、作用域失效和确认状态；开关走独立原子接口，停用保留 workspace 文件，命令回调只创建命令，执行与收尾按阶段处理。
- 资源加载态合并、请求过期判断与命令展示态均由纯函数投影，Hook 只编排生命周期。
- `agent-skills-model.ts` 只处理已启用/可启用分组与搜索投影。
- 技能选项卡使用可自然增高的紧凑分隔列表，不为每个 Skill 重复绘制卡片外框；手机端开关落到内容下方，标题、徽标和说明不得被固定高度挤压。分组已经表达启用状态，行内不再重复“启用/已启用”文字。
- `agent-skill-card.tsx` 明确标记当前 Agent 的本地 workspace Skill；这类 Skill 不进入全局技能库、不对其它 Agent 可见，存在时默认启用，停用只写当前 Agent 的显式停用状态。
- `agent-options-skills-view.tsx` 只组合错误提示、内容与确认弹窗，不重复渲染技能总数或常驻手动刷新；资源已按页面可见性、窗口焦点和固定间隔自动刷新。`agent-options-skills-content.tsx` 分别渲染状态、已启用列表和可启用列表，`agent-skill-card.tsx` 只渲染单项。

列表与命令结果必须绑定 Agent；旧请求、旧命令不得写入新作用域，页面卸载后不得继续刷新视图状态。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
