# Agent Options 技能域

- `use-agent-skills-resource.ts` 负责 Agent 作用域列表、请求取消和可见时刷新；前台与后台刷新使用显式模式。
- `use-agent-skills-controller.ts` 负责 Agent 作用域的启用/停用互斥命令、作用域失效和确认状态；开关走独立原子接口，停用保留 workspace 文件，命令回调只创建命令，执行与收尾按阶段处理。
- 资源加载态合并、请求过期判断与命令展示态均由纯函数投影，Hook 只编排生命周期。
- `agent-skills-model.ts` 只处理已启用/可启用分组与搜索投影。
- 技能选项卡使用固定紧凑卡片；`agent-options-skills.css` 按编辑器内容容器的真实宽度切换三列、两列、单列，禁止复用整窗断点挤压右侧简介面板。卡片使用 `skill.name` 生成跨页面稳定的静态数学曲线身份标记，头像、标题/必要来源徽标和开关共用 40px 垂直中心轴；最多两行用途摘要帮助决定是否启用，完整说明仍在全局技能详情展示。分组与开关已经表达启用状态，行内不再重复“启用/已启用”文字；系统内置状态由标题旁锁定徽标表达，右侧不再重复状态。
- 内置 Skill 的双语名称统一由 `lib/skill-description.ts` 做只读展示投影；本域不得修改 API Skill 对象、`SKILL.md` 或同名用户 Skill。
- 可启用 Skill 的搜索属于分组工具，桌面与分组标题同排、手机端换到标题下方；内置 Skill 使用当前语言的展示说明参与搜索。未搜索时不显示无信息增量的 `n/n`，筛选时才显示命中数与可用总数。
- `agent-skill-card.tsx` 明确标记当前 Agent 的本地 workspace Skill；这类 Skill 不进入全局技能库、不对其它 Agent 可见，存在时默认启用，停用只写当前 Agent 的显式停用状态。
- `agent-options-skills-view.tsx` 只组合错误提示、内容与确认弹窗，不重复渲染技能总数或常驻手动刷新；资源在页面进入、窗口重新聚焦或恢复可见时刷新，不做固定间隔轮询。`agent-options-skills-content.tsx` 分别渲染状态、已启用列表和可启用列表，`agent-skill-card.tsx` 只渲染单项。

列表与命令结果必须绑定 Agent；旧请求、旧命令不得写入新作用域，页面卸载后不得继续刷新视图状态。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
