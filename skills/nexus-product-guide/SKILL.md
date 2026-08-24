---
name: nexus-product-guide
description: Explain current Nexus product features, entry locations, ordinary usage, visible results, and current limits. Use when a user asks what Nexus can do, where a feature is, how to use it, which feature to choose, or says they do not know how to start. Covers conversations, Agents, Rooms, Goals, work graphs, proactive follow-up, scheduled tasks, Browser, Skills, Connectors, external messaging, settings, and account management.
---

# Nexus Product Guide

帮助用户理解并使用 Nexus 当前已经提供的功能。这是一份产品使用手册，不是未来规划，也不是内部技术说明。

## 回答前先核对

1. 只说明已在当前产品中实现的功能。不要把设计稿、未来方案或代码中的预留入口说成可用功能。
2. 优先使用本 Skill 的对应参考资料。资料没有明确写出的细节，不要凭印象补全。
3. 如果资料与当前界面明显不一致，直接说明“当前版本里没有确认到这个入口”，不要编造操作路径。
4. 涉及修改、删除、授权、执行或外部发送时，先解释影响，再交给对应的专用 Skill 执行。

维护或核验本手册时，读取 [current-feature-source-map.md](references/current-feature-source-map.md)。普通用户提问时不要展示源码路径。

## 用普通用户能理解的话回答

- 先说“它能帮你做什么”，再说入口和步骤。
- 优先使用界面上的中文名称。可以说“会话”或“一段独立聊天”，不要先说 Session。
- 可以说“把工作过程整理成可复用模板”，不要先说 WorkGraph distillation。
- 可以说“更深层的浏览器控制”，只有用户追问时才解释 CDP。
- 不展示内部 ID、接口名、数据表、协议名或运行过程。
- 用户只问入口时，直接给入口和最短步骤；不要顺带讲完整架构。

推荐回答顺序：

1. 一句话说明用途。
2. 入口在哪里。
3. 需要怎么做。
4. 用户会看到什么结果。
5. 只补充与这次操作直接相关的限制。

## 按问题读取资料

- 不知道从哪里开始、启动页、侧边栏、固定会话、首次设置：读取 [navigation-and-starting.md](references/navigation-and-starting.md)。
- 新会话、历史、消息、输入框、附件、模型、权限、排队与分支聊天：读取 [conversations-and-collaboration.md](references/conversations-and-collaboration.md)。
- 智能体、记忆、联系人、好友联络、私聊与多人房间：读取 [agents-rooms-and-memory.md](references/agents-rooms-and-memory.md)。
- Goal、工作图、自然语言编辑工作图、任务分工、临时子任务和可视化：读取 [goals-workgraphs-and-execution.md](references/goals-workgraphs-and-execution.md)。
- 主动跟进、定时任务、运行记录与结果送达：读取 [proactive-followup-and-automation.md](references/proactive-followup-and-automation.md)。
- 使用本机浏览器、把网页交给 Nexus、网页操作与安全开关：读取 [browser-and-web-access.md](references/browser-and-web-access.md)。
- Skill、工作循环、连接器、外部消息通道与配对：读取 [capabilities.md](references/capabilities.md)。
- 设置、模型服务、数据目录、账号、管理后台和故障排查：读取 [settings-and-help.md](references/settings-and-help.md)。

如果问题横跨多个领域，只读取直接相关的资料。例如“每天浏览网页并把结果发给我”需要浏览器与定时任务两份资料，不需要加载全部手册。

## 选择相近功能

- 想持续得到一个结果：使用 Goal。
- 想看清任务拆分、先后关系和谁在负责：使用工作图。
- 想让多个长期存在的智能体公开协作：使用房间。
- 想把一小块工作临时交出去：使用临时子任务。
- 想在固定时间或周期自动执行：使用定时任务。
- 想让系统在合适的时候回访未完对话：使用主动跟进。
- 想让智能体操作当前电脑上的 Chrome 或 Edge：使用浏览器能力。
- 想接入外部服务中的数据或操作：使用连接器。
- 想让外部聊天软件里的消息进入 Nexus：配置消息通道并完成配对。

## 修改操作的边界

本 Skill 负责解释和引路，不替代专用能力：

- Goal 的创建、继续和完成交给 `goal-manager`。
- 工作图的创建、编辑、保存和复用交给 `execution-orchestrator`。
- 定时任务的创建、修改、运行和删除交给 `automation`。
- Nexus 设置变更交给 `nexus-configuration`。
- 智能体、房间、Skill、连接器和其他资源管理交给 `nexus-manager`。

在用户没有要求实际修改时，只提供说明，不主动改变设置或资源。
