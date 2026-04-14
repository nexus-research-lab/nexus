# Nexus 主智能体统一规范

## 1. 文档目的

本文档定义 Nexus 主智能体的统一身份、真相源、运行边界、编排职责和前后端接入规则。

主智能体不是普通 Agent 的一个默认名字，也不是首页上的一块 UI。它同时横跨以下几层：

- 后端配置
- Agent 初始化与持久化
- Workspace 模板与系统提示词
- Skill 与 CLI 编排入口
- Launcher / 首页交互
- 多通道默认路由

后续所有与主智能体相关的改动，都必须以本文档为准推进。若代码实现与本文档不一致，以“唯一真相源”和“职责边界”优先做收敛。

## 2. 几类概念先分开

### 2.1 主智能体标识

- 表示系统内唯一保留的主智能体 `agent_id`
- 当前唯一真相源是后端 `settings.DEFAULT_AGENT_ID`
- 它是业务身份，不是展示文案，不是前端默认值

### 2.2 主智能体 Profile

- 表示围绕主智能体的固定运行规则
- 当前由 `MainAgentProfile` 承载
- 它不是独立真相源，只是后端配置的行为适配层

### 2.3 主智能体 Workspace

- 表示主智能体专用工作区
- 路径固定落在 `workspace/<default_agent_id>/`
- 初始化时使用主智能体专属模板，而不是普通成员模板

### 2.4 主智能体编排能力

- 表示主智能体可调用的系统级管理动作
- 当前由 `nexus-manager` skill、Typer CLI 和 `MainAgentOrchestrationService` 共同组成
- 当前覆盖 agent / room / workspace / skill / automation orchestration，而不是只做成员与房间管理
- 它服务于组织协作，不是普通房间成员能力

### 2.5 主智能体入口会话

- 表示首页 Launcher 右侧的 Nexus 对话入口，以及其他默认落到主智能体的外部通道入口
- 这些入口共享同一个主智能体身份
- 但不代表它们都属于同一条业务会话历史

### 2.6 普通成员 Agent

- 表示用户创建的普通协作成员
- 可加入 Room、被删除、被重命名、被配置 skill
- 与主智能体是不同角色，不允许混用

## 3. 设计目标

主智能体体系必须长期满足以下目标：

1. 主智能体身份只能有一个真相源
2. 前端不能再维护第二份主智能体默认值
3. 主智能体和普通成员的职责必须显式区分
4. 主智能体的系统提示词、Workspace 模板和托管 skill 必须自动补齐
5. 主智能体不能被当作普通 Room 成员使用
6. 所有“默认走主智能体”的入口都必须显式依赖同一份配置

## 4. 架构总览

主智能体相关逻辑当前分成五层：

### 4.1 配置层

- 后端 `settings.DEFAULT_AGENT_ID`
- Settings 页里的默认 Provider 配置
- 后端 `settings.MAIN_AGENT_SYSTEM_PROMPT`

### 4.2 身份与初始化层

- `MainAgentProfile`
- `AgentRepository._ensure_main_agent()`
- `FileStorageBootstrap`
- `WorkspaceTemplateInitializer`

### 4.3 运行时行为层

- `AgentPromptBuilder`
- `WorkspaceSkillDeployer`
- `SkillService`
- `RoomService`
- `AgentManager`

### 4.4 编排能力层

- `nexus-manager` skill
- `MainAgentOrchestrationService`
- `agent/cli.py`

### 4.5 入口与路由层

- `LauncherPage`
- `Launcher API`
- `ChatService`
- Discord / Telegram channel
- 前端 `runtime/options` 启动配置

## 5. 唯一真相源规则

### 5.1 身份真相源

主智能体身份的唯一真相源是：

```text
agent/config/config.py -> settings.DEFAULT_AGENT_ID
```

规则如下：

- 任何主智能体身份判断都必须最终回到这项配置
- `MainAgentProfile.AGENT_ID` 只是配置映射，不是第二真相源
- 前端运行时只能读取后端下发的 `default_agent_id`
- 不允许再通过 `VITE_DEFAULT_AGENT_ID`、硬编码 `"nexus"` 或 `"main"` 维持另一套默认值

### 5.2 行为真相源

主智能体行为规则的统一入口是：

```text
agent/service/agent/main_agent_profile.py
```

它负责：

- 判断某个 `agent_id` 是否为主智能体
- 提供主智能体展示标签
- 提供主智能体默认运行参数

规则如下：

- 业务判断使用 `MainAgentProfile.is_main_agent()`
- 用户可见提示使用 `MainAgentProfile.display_label()`
- 不允许在业务代码里再手写 `"main agent"` 或 `"nexus"` 来表示主智能体身份

### 5.3 前端真相源

前端当前只允许通过以下链路获取主智能体标识：

```text
/agent/v1/runtime/options -> web/src/config/options.ts -> DEFAULT_AGENT_ID
```

规则如下：

- `DEFAULT_AGENT_ID` 现在是前端运行时缓存，不是独立配置
- React 启动前必须先执行 `hydrateRuntimeOptions()`
- 页面和 hook 只能消费这份运行时值，不允许再自行推导

## 6. 生命周期规范

### 6.1 启动初始化

启动时，`AgentRepository._ensure_ready()` 必须保证：

1. 文件系统基础目录已就绪
2. 主智能体记录存在于 SQLite
3. 主智能体 Workspace 已初始化
4. 其他活跃成员的 Workspace 也完成补齐

### 6.2 主智能体建档

`_ensure_main_agent()` 的职责包括：

- 如果主智能体不存在，则自动创建
- 如果已存在但状态不正确，则强制修正
- 保证主智能体 `name`、`display_name`、`workspace_path` 和默认运行参数一致

这意味着：

- 主智能体不是用户手工创建对象
- 它是系统保留对象
- 它必须始终存在

### 6.3 前端启动注入

前端应用启动时：

1. 调用 `/agent/v1/runtime/options`
2. 读取 `default_agent_id`
3. 写入前端运行时 `DEFAULT_AGENT_ID`
4. 再执行 React 渲染

这样做的目的不是增加动态性，而是消灭前后端双真相源。

## 7. Workspace 与 Prompt 规范

### 7.1 模板分流

主智能体和普通成员必须使用不同模板：

- 普通成员：`WORKSPACE_TEMPLATES`
- 主智能体：`MAIN_AGENT_WORKSPACE_TEMPLATES`

主智能体模板强调：

- 它是系统级组织代理
- 它负责整理协作结构
- 它不长期承载执行型协作
- 它优先把用户带入具体 Room 或 Contacts

### 7.2 System Prompt 分流

`AgentPromptBuilder` 构建提示词时：

- 主智能体优先加载 `MAIN_AGENT_SYSTEM_PROMPT`
- 普通成员加载 `BASE_SYSTEM_PROMPT`
- 之后再拼接 Workspace 文件内容

这意味着：

- 主智能体不是“普通 Agent + 特殊名字”
- 它在模型提示层就已经被定义成系统级协调者

## 8. Skill 与编排能力规范

### 8.1 系统托管 skill

当前系统托管 skill 分两类：

- 所有 Agent 都有：`memory-manager`
- 只有主智能体有：`nexus-manager`

### 8.2 部署规则

`WorkspaceSkillDeployer` 负责：

- 基础 skill 总是部署
- 当且仅当目标 Agent 是主智能体时，额外部署 `nexus-manager`

### 8.3 暴露规则

`SkillCatalog` 里：

- `memory-manager` 是公开系统 skill
- `nexus-manager` 是 internal skill

规则如下：

- `nexus-manager` 不进入公开 marketplace
- 普通成员看不到也不能安装它
- 主智能体通过 Workspace 自动获得它

### 8.4 编排接口

主智能体通过以下能力执行系统操作：

- `MainAgentOrchestrationService`
- `agent/cli.py`
- `agent/cli/command.py`
- `nexus-manager` skill

当前覆盖的动作包括：

- 创建 / 删除成员
- 创建 / 更新 / 删除 Room
- 追加 / 移除 Room 成员
- 读写成员 Workspace
- 安装 / 卸载 Skill
- 列出 / 创建 / 启停 / 立即运行 / 删除定时任务
- 读取定时任务运行记录

定时任务创建能力当前已经不是“只能绑定已有会话”的模型，而是统一使用结构化会话目标：

- `main`：投递到目标 Agent 的主会话
- `bound`：绑定到一个已有 `session_key`
- `named`：投递到一个稳定的命名会话 key
- `isolated`：每次执行使用隔离自动化会话

这意味着主智能体编排入口在创建定时任务时，必须按当前实现理解会话目标，而不能再把 scheduled task 默认描述成 bound-only。

当前还有两个容易误解的点，需要在主智能体技能和交互里明确说明：

- 现有 CLI `create_scheduled_task` 只暴露 `session_target` 与 `schedule`，创建时固定使用 `delivery=none`；也就是说，CLI 侧没有“结果回传到当前会话 / 指定会话”的参数。
- Web 控制台比 CLI 多一层“结果回传”配置。当前前端默认规则是：
  - `existing + execution`：结果回到当前执行会话
  - `temporary + execution`：Agent 模式下不额外回传；结果保留在临时会话
  - `dedicated + execution`：结果回到专用长期会话
  - `selected`：结果回到用户指定的会话
  - `none`：不额外回传

因此，“创建定时任务后默认回当前 session”并不是全局成立的规则，只在 Web 控制台的 `existing + execution` 组合下成立。

Heartbeat 相关能力当前通过后端 automation API 暴露：

- `GET /agent/v1/automation/heartbeat/{agent_id}`：读取状态
- `PUT /agent/v1/automation/heartbeat/{agent_id}`：更新持久化配置
- `POST /agent/v1/automation/heartbeat/{agent_id}/wake`：手动触发一次 wake

这里要注意：

- 这组 heartbeat 能力当前是后端 API 语义，不是 main-agent CLI 子命令
- 文档和交互说明不能把尚不存在的 heartbeat CLI 包装成已支持能力

## 9. 会话与入口规范

### 9.1 首页 Launcher

首页右侧 Nexus 面板是主智能体入口。

规则如下：

- 入口 Agent 必须使用当前运行时 `DEFAULT_AGENT_ID`
- 会话种子键只能基于当前主智能体生成
- 不能写死 `main`、`nexus` 或其他固定字面量

### 9.2 默认 Agent 回退

以下入口在未显式指定 Agent 时，会默认路由到主智能体：

- Web 普通 ChatService 回退
- Discord 通道
- Telegram 通道
- 部分文件会话/成本汇总的默认 agent 补位

规则如下：

- 这些回退都必须基于后端 `settings.DEFAULT_AGENT_ID`
- 不能在各模块各自维护一个“默认 agent 名称”

### 9.3 Launcher 推荐边界

Launcher 推荐列表明确不推荐主智能体：

- 推荐成员列表会过滤掉 `settings.DEFAULT_AGENT_ID`

原因很简单：

- 主智能体是系统入口，不是普通成员推荐项
- 用户选择成员时，应看到真正可加入协作的普通 Agent

## 10. Room 与成员边界

主智能体不能参与 Room 成员关系。

当前明确限制包括：

- 不可加入 Room
- 不可作为 Room 成员保留
- 创建 Room 时会自动跳过主智能体
- 如果传入成员列表只剩主智能体，直接报错

同时：

- 主智能体不能删除
- 主智能体不能重命名

这些限制保证主智能体始终保持“系统级协调者”身份，而不是退化成普通协作成员。

## 11. 对前端的影响

前端当前需要遵守这些规则：

1. `DEFAULT_AGENT_ID` 只读运行时配置
2. Room 邀请候选列表要过滤主智能体
3. 首页 Nexus 对话只走主智能体，不与普通成员入口混用
4. 会话默认归属、路由解析和 session_key builder 不能再手写主智能体名字

## 12. 当前代码落点

当前主智能体相关代码应以这些文件为准：

- 后端配置：`agent/config/config.py`
- 运行时下发：`agent/api/common/api_runtime.py`
- 主智能体行为入口：`agent/service/agent/main_agent_profile.py`
- 初始化与保底建档：`agent/service/agent/agent_repository.py`
- Agent 提示词构建：`agent/service/agent/agent_prompt_builder.py`
- Workspace 模板：`agent/service/workspace/workspace_templates.py`
- 系统 skill 部署：`agent/service/workspace/workspace_skill_deployer.py`
- Skill catalog / 安装规则：`agent/service/capability/skills/skill_catalog.py`
- Skill service：`agent/service/capability/skills/skill_service.py`
- 编排服务：`agent/service/agent/main_agent_orchestration_service.py`
- CLI：`agent/cli.py`、`agent/cli/command.py`
- 定时任务 capability API：`agent/api/capability/api_scheduled_task.py`
- Heartbeat automation API：`agent/api/automation/api_heartbeat.py`
- Room 边界：`agent/service/room/room_service.py`
- 首页入口：`web/src/pages/launcher/launcher-page.tsx`
- 前端运行时配置：`web/src/config/options.ts`
- 默认聊天回退：`agent/service/chat/chat_service.py`
- 多通道入口：`agent/service/channels/im/discord_channel.py`、`agent/service/channels/im/telegram_channel.py`

## 13. 反模式清单

以下做法明确禁止：

1. 在前端环境变量里再维护一份主智能体默认值
2. 在 UI 或 service 里手写 `"main"`、`"nexus"` 来判断主智能体身份
3. 把主智能体当作普通 Room 成员加入协作
4. 让主智能体出现在普通成员推荐列表里
5. 允许用户删除或重命名主智能体
6. 把 `nexus-manager` 暴露到公开 skill 列表
7. 用普通成员模板初始化主智能体 Workspace
8. 让首页 Nexus 面板和普通成员 DM 共用一套心智
9. 在文档里把“主智能体名字”写成固定字面量，而不强调它来自后端配置

## 14. 最终结论

后续要长期坚持这条原则：

- 主智能体身份的唯一真相源是后端 `settings.DEFAULT_AGENT_ID`
- `MainAgentProfile` 是行为适配层，不是第二真相源
- 主智能体是系统级组织代理，不是普通成员 Agent
- 它负责编排协作、组织结构、引导入口
- 具体执行协作应回到普通成员和具体 Room

如果未来主智能体继续演进，应当在本文档上增量扩展，而不是重新发明第二套身份体系。
