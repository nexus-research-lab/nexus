# Skill 统一规范

## 1. 文档目的

本文档定义 Nexus 中 `skill` 的统一设计、路径协议、暴露边界、真相源规则，以及 `skill` 能力彻底去 DB 化后的目标形态。

`skill` 不是数据库对象，也不是单纯的 UI 卡片。它同时横跨以下几层：

- 源定义目录
- 外部导入注册表
- Workspace 内部部署目录
- Claude 运行时发现入口
- Catalog 聚合与管理逻辑

后续所有 `skill` 相关迭代，都必须以本文档为准推进。若本文档与当前代码实现不一致，以本文档定义的目标态为准做收敛。

## 2. 几类概念先分开

### 2.1 skill 源目录

- 表示一个 skill 的原始内容目录
- 至少包含一个 `SKILL.md`
- 可能来自仓库内置目录，也可能来自外部导入注册表

### 2.2 registry skill

- 表示一个已经被导入到本地受控注册表的外部 skill
- 实际路径位于 `cache/skills/registry/<skill_name>/`
- 它是外部 skill 的稳定本地源，不是运行时直接消费目录

### 2.3 deployed skill

- 表示已经被部署到某个 Agent workspace 内部的 skill 副本
- 实际路径位于 `<workspace>/.agents/skills/<skill_name>/`
- 这是运行时真实可执行的 skill 实体

### 2.4 runtime entry

- 表示 Claude 运行时用来发现 skill 的入口
- 当前通过 `<workspace>/.claude/skills/<skill_name>` 相对软链实现
- 它应始终指向 `.agents/skills/<skill_name>`

### 2.5 public catalog skill

- 表示允许出现在 Skill Marketplace 和 Agent 技能配置页里的 skill
- 包括：
  - 对外可见系统 skill
  - 内置 public skill
  - 外部导入 public skill

### 2.6 internal skill

- 表示仅供平台内部编排使用的 skill
- 可以部署到特定 workspace
- 但不能暴露到公开 catalog、Agent skill 列表或普通安装接口

### 2.7 控制面元数据

- 表示围绕 skill 的管理信息
- 当前目标只允许来自文件系统和静态清单
- 例如：
  - 内置 catalog 清单中的分类与推荐语
  - 外部导入 manifest 中的来源、版本与导入方式
  - 某个 Agent workspace 当前实际部署了哪些 public skill
- 它不是数据库行，也不能脱离文件系统单独宣称“skill 已安装”

## 3. 设计目标

统一规范必须长期满足以下目标：

1. skill 的执行真相源必须是文件系统，而不是数据库状态位
2. internal skill 和 public skill 必须显式区分，不能混用
3. workspace 初始化必须幂等，随时可补齐缺失 skill
4. 公开 catalog 的 `installed` 和 `locked` 语义必须与实际运行态一致
5. 外部导入 skill 必须有稳定的本地注册表，不直接依赖临时目录
6. skill 专用能力必须彻底去 DB 化，不再维护 `pool_skills` / `agent_skills` 这类专用表

## 4. 架构分层总览

目标态下，skill 体系分四层：

### 4.1 源定义层

- 仓库内置 skill：`skills/<skill_name>/`
- 外部导入注册表：`cache/skills/registry/<skill_name>/`
- 内置 catalog 清单：`agent/service/capability/skills/data/curated_skill_catalog.json`

### 4.2 Catalog 聚合层

- 负责把系统、内置、外部 skill 聚合成统一的 public catalog
- 供 API / 前端查询
- internal skill 不进入这一层的公开输出

### 4.3 Workspace 部署层

- 负责把 skill 源目录同步到 `<workspace>/.agents/skills/<skill_name>/`
- 并在 `<workspace>/.claude/skills/<skill_name>` 创建相对软链
- 这是运行时真实使用的 skill 形态

### 4.4 管理层

- 负责 catalog 查询、导入、安装、卸载、更新、删除
- 该层只能从源目录、注册表、workspace 部署状态推导结论
- 不再依赖 skill 专用数据库表保存资源池、启用开关或 Agent 挂载关系

## 5. 路径协议

当前路径协议如下：

| 层级 | 路径 | 语义 |
| --- | --- | --- |
| 仓库内置源目录 | `skills/<skill_name>/` | 仓库自带 skill 定义 |
| 内置 catalog 清单 | `agent/service/capability/skills/data/curated_skill_catalog.json` | 内置 public skill 分类与推荐语 |
| 外部注册表目录 | `cache/skills/registry/<skill_name>/` | 导入后的受控 skill 副本 |
| 外部清单文件 | `cache/skills/registry/<skill_name>/.nexus-skill.json` | 外部导入元数据 |
| Workspace 内部部署目录 | `<workspace>/.agents/skills/<skill_name>/` | 运行态真实 skill 副本 |
| Claude 发现入口 | `<workspace>/.claude/skills/<skill_name>` | 指向 `.agents/skills/<skill_name>` 的相对软链 |

补充规则：

- skill 运行时一律从 workspace 内部部署目录消费，不直接引用仓库根目录或 registry 目录
- `.claude/skills/` 不保存真实文件，只保存入口软链
- `.agents/skills/` 和 `.claude/skills/` 都属于内部运行时目录，业务文件接口不得直接暴露给用户改写
- 外部导入 skill 的所有控制面元数据，必须与 `.nexus-skill.json` 同目录存放

## 6. skill 分类与暴露规则

当前 skill 按“是否公开”和“来源类型”分成四类：

| 类别 | 示例 | 是否公开 | 是否自动部署 | 是否允许手动装卸 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 对外可见系统 skill | `memory-manager` | 是 | 是，所有 Agent | 否 | `locked=true`，平台托管 |
| 内部编排 skill | `nexus-manager` | 否 | 是，仅主智能体 | 否 | 只能内部使用 |
| 内置 public skill | curated catalog 项 | 是 | 否 | 是 | 被 catalog 收录后即可直接部署到 Agent |
| 外部 public skill | Git / skills.sh / 本地导入 | 是 | 否 | 是 | 导入 registry 后即可直接部署到 Agent |

补充说明：

- `scope=main` 表示“公开但只适用于当前主智能体”
- 当前主智能体的唯一真相源是后端 `settings.DEFAULT_AGENT_ID`
- 前端只通过 `/agent/v1/runtime/options` 读取主智能体标识，不再自带第二份默认值
- `internal skill` 和 `scope=main` 不是同一概念
- `internal skill` 不对外暴露；`scope=main` 的 public skill 仍可出现在公开 catalog，只是普通 Agent 不可用

## 7. 生命周期规范

### 7.1 系统托管 skill 生命周期

系统托管 skill 分两类：

- 对外可见系统 skill
- internal skill

当前规则：

- `memory-manager`
  - 对外可见
  - 所有 Agent workspace 初始化时自动部署
- `nexus-manager`
  - 仅主智能体内部使用
  - 只在主智能体 workspace 初始化时自动部署

系统托管 skill 生命周期由 workspace 初始化器负责，不走普通安装接口。

### 7.2 内置 public skill 生命周期

流程如下：

1. skill 源目录来自仓库或内置搜索根目录
2. `curated_skill_catalog.json` 收录该 skill 的公开分类信息
3. catalog 发现该 skill
4. 用户为某个 Agent 安装该 skill
5. deploy 到该 Agent workspace
6. 卸载时移除 workspace 部署副本和运行时入口

### 7.3 外部 public skill 生命周期

流程如下：

1. 从本地目录、zip、Git 或 skills.sh 导入
2. 解析 `SKILL.md` 和 frontmatter
3. 写入 `cache/skills/registry/<skill_name>/`
4. 生成 `.nexus-skill.json`
5. catalog 自动发现该 skill
6. 用户为某个 Agent 安装该 skill
7. deploy 到目标 workspace
8. 更新时先刷新 registry，再同步到已实际部署该 skill 的 Agent workspace

### 7.4 删除与更新规则

- system skill 不允许 uninstall / delete
- internal skill 不允许公开 install / uninstall / delete
- external skill 更新以 registry 为中间层，不直接改 workspace 原文件
- 删除 external public skill 时，必须先清理所有已部署 workspace 副本，再删 registry 目录
- 删除 builtin public skill 只意味着停止对 Agent 部署，不允许删除仓库源目录

## 8. Workspace 初始化契约

workspace 初始化必须同时完成三件事：

1. 补齐目录结构和模板文件
2. 注入 skill 渲染上下文
3. 补齐系统托管 skill 部署

当前 `SKILL.md` 支持的上下文字段包括：

- `agent_id`
- `agent_name`
- `created_at`
- `project_root`
- `workspace`

规则如下：

- 只有在部署到 workspace 时才做 `SKILL.md` 上下文渲染
- 不覆盖用户已有业务文件
- 可以重复执行，用于补齐存量 workspace 缺失 skill
- 对 public skill 的安装状态判断，必须以当前 workspace 中的实际部署结果为准

## 9. 真相源规则

### 9.1 执行真相源

以下状态一律以文件系统为真相源：

- 某个 Agent 当前是否真的能执行某个 skill
- 某个 skill 的运行时正文内容
- Claude 运行时能否发现某个 skill
- 某个 workspace 是否已经具备系统托管 skill
- 某个 Agent 是否已经安装某个 public skill

具体以这些路径为准：

- `<workspace>/.agents/skills/<skill_name>/`
- `<workspace>/.claude/skills/<skill_name>`

### 9.2 源内容真相源

以下内容以源目录或注册表为真相源：

- 仓库内置 skill 正文
- 内置 public skill 的分类与推荐信息
- 外部导入 skill 正文
- 外部导入 manifest

### 9.3 控制面真相源

以下管理信息允许存在，但必须由文件系统或静态清单承载：

- 某个内置 skill 是否属于公开 catalog
- 某个 external skill 的来源、版本、导入方式
- 某个 Agent 当前实际部署了哪些 public skill

必须满足：

- 控制面只描述“如何管理”
- 控制面不能脱离文件系统单独决定“运行时是否已存在”
- 一旦与文件系统冲突，先修复 registry 或 workspace，再重新聚合 catalog

## 10. 去 DB 规范

### 10.1 适用边界

本节只约束 `skill` 专用能力，不影响其他业务数据：

- 保留 Agent、Room、Session 等既有数据库能力
- 移除 `skill` 专用资源池与挂载关系表
- 不再为 `skill` 单独维护“已安装”“全局启用”“进入资源池”这类数据库状态

### 10.2 真相源矩阵

| 语义 | 真相源 | 读取方式 |
| --- | --- | --- |
| 对外可见系统 skill 是否存在 | 仓库内置目录 | `skills/<skill_name>/SKILL.md` |
| 内置 public skill 是否属于 catalog | 内置 catalog 清单 + 源目录 | `curated_skill_catalog.json` + `skills/<skill_name>/SKILL.md` |
| external public skill 是否存在 | registry 目录 + `.nexus-skill.json` | `cache/skills/registry/<skill_name>/` |
| internal skill 是否允许暴露 | 后端内置白名单 | `skill_catalog.py` / deployer 常量 |
| 某个 Agent 是否已安装某个 skill | workspace 部署目录 | `<workspace>/.agents/skills/<skill_name>/` |
| Claude 是否可发现某个 skill | runtime entry 软链 | `<workspace>/.claude/skills/<skill_name>` |

### 10.3 禁止语义

以下语义不得继续存在：

- 资源池 `pool`
- `pool_skills`
- `agent_skills`
- `global_enabled`
- “先入池再挂载 Agent”
- “数据库记录存在即视为 skill 已安装”

对应原则：

- public skill 的可见性来自清单或注册表，不来自数据库
- public skill 的安装状态来自 workspace，不来自数据库
- external skill 的可删除性来自来源类型，不来自数据库开关

### 10.4 Service 协议

服务层必须满足以下条款：

1. `SkillCatalog` 的公开输出只允许由仓库源目录、内置 catalog 清单和 external registry 聚合产生。
2. `installed` 字段只允许由目标 Agent workspace 的实际部署状态推导。
3. skill 安装的协议结果只允许是目标 workspace 中出现部署副本与 runtime entry，不允许生成独立数据库安装记录。
4. skill 卸载的协议结果只允许是目标 workspace 中对应部署副本与 runtime entry 被移除，不允许依赖数据库状态完成卸载。
5. external skill 更新的协议顺序固定为：先刷新 registry，再覆盖所有已实际部署该 skill 的 workspace 副本。
6. Agent skill 列表的公开输出只允许包含对该 Agent 可见的 public skill，不允许包含 internal skill。

### 10.5 API 规范

API 必须满足以下约束：

- 保留 `/skills`
- 保留 `/skills/{skill_name}`
- 保留 `/agents/{agent_id}/skills`
- 保留 `/agents/{agent_id}/skills` 的 install / uninstall / batch install / update-installed
- 保留 external import 与 update API
- 移除 `/skills/{skill_name}/install`
- 移除 `/skills/{skill_name}/global-enabled`
- `/skills/{skill_name}` 的 delete 只允许删除 external public skill 的 registry 项

### 10.6 前端协议

前端必须满足以下条款：

1. 全局 skills 页面只表达 catalog 收录、external 导入、external 更新、external 删除，不表达资源池语义。
2. 前端不得出现全局启用、全局停用、入池、出池等控制项与状态文案。
3. 前端不得引入“已安装到池里但未挂到 Agent”的中间态。
4. Agent 维度的安装与卸载入口只能出现在 Agent skill 配置视图中。
5. 前端对 skill 状态的表达只允许基于 catalog 可见、当前 Agent 已安装、当前 Agent 不可用三类语义。

## 11. API 语义规范

### 11.1 `/skills`

- 表示公开 Skill Marketplace catalog
- 只返回 public skill
- 不返回 internal skill
- 带 `agent_id` 查询时，`installed` 表示该 Agent 视角下该 skill 当前是否已实际部署

### 11.2 `/skills/{skill_name}`

- 表示某个 public skill 的详情
- internal skill 不提供公开 detail 查询

### 11.3 `/agents/{agent_id}/skills`

- 表示某个 Agent 当前可配置的 public skill 列表
- 包括：
  - 对外可见系统 skill
  - scope 匹配的内置 public skill
  - scope 匹配的 external public skill
- 不包括 internal skill
- `installed` 一律来自该 Agent workspace 的实际部署状态

### 11.4 `locked`

- `locked=true` 只表示系统托管，不能由用户手动 uninstall
- 它不代表“内部 skill”
- internal skill 应通过“完全不暴露”表达，而不是依赖 `locked=true` 暗示

## 12. 导入与命名冲突规则

外部导入必须遵守以下规则：

1. 导入内容中只能解析出一个有效 `SKILL.md`
2. 不能覆盖仓库内置或系统保留名称
3. 同名 external skill 不允许绕过保护根目录直接覆盖

保护根目录包括：

- `skills/`
- `~/.codex/skills/`
- `~/.agents/skills/`
- `~/.cc-switch/skills/`

结论：

- external 导入完成后即进入公开 catalog，不再存在“导入成功但还没入池”的第二阶段
- public external skill 只能扩展 catalog，不能覆盖平台内置协议

## 13. 稳定性规则

以下内容一旦冻结，不允许轻易变更：

1. workspace 内 `.agents/skills/` 作为运行态真实目录
2. workspace 内 `.claude/skills/` 作为 Claude 发现入口
3. system skill 与 internal skill 的公开边界
4. “文件系统是执行与安装真相源，skill 不引入专用数据库表”的总原则
5. public skill 与 internal skill 不混用的分类方法

可以扩展，但不能直接推翻现有含义：

- 新增 public skill 类别
- 新增 external 导入来源
- 调整 public catalog 分类字段
- 扩展 `.nexus-skill.json` 的非运行态元数据

## 14. 变更治理流程

如果未来必须升级 skill 机制，流程必须是：

1. 先更新本文档，明确新增能力属于哪一层
2. 明确该能力的真相源是源目录、registry 还是 workspace
3. 明确它是 public 还是 internal
4. 再修改 service / API / UI / workspace 初始化逻辑
5. 提供存量 workspace 的补齐策略
6. 如果涉及用户可见行为变化，同步更新 `CHANGELOG.md`

任何跳过本文档直接改语义的做法，都视为不合规改动。

## 15. 反模式清单

以下做法明确禁止：

1. 把 `nexus-manager` 重新暴露到 `/skills` 或 Agent 配置页
2. 把 `installed` 重新做成“只看数据库，不看文件系统”
3. 重新引入 `pool_skills` / `agent_skills` 一类 skill 专用状态表
4. 重新引入“导入 registry 之后还要先入池”的双阶段模型
5. 直接往 `.claude/skills/` 写真实 skill 文件而绕过 `.agents/skills/`
6. 允许 external 导入覆盖仓库内置 skill 名称
7. 把 `SKILL.md` 正文塞进数据库长期存储
8. 在业务代码里手工维护 skill 软链而绕过统一 deployer
9. 假设 workspace 一定已经拥有系统 skill，而不走初始化或补齐流程
10. 用 `locked=true` 代替 internal/public 的暴露边界设计
11. 在前端或文档里再维护一份主智能体默认值，绕过后端 `settings.DEFAULT_AGENT_ID`

## 16. 当前代码落点

当前 skill 相关代码应以这些文件为准：

- catalog 聚合：`agent/service/capability/skills/skill_catalog.py`
- skill service：`agent/service/capability/skills/skill_service.py`
- 外部导入：`agent/service/capability/skills/skill_import_service.py`
- 外部注册表：`agent/service/capability/skills/skill_registry_store.py`
- 内置 catalog 清单：`agent/service/capability/skills/data/curated_skill_catalog.json`
- API：`agent/api/capability/api_skill.py`
- workspace 初始化：`agent/service/workspace/workspace_template_initializer.py`
- workspace skill 部署：`agent/service/workspace/workspace_skill_deployer.py`
- Agent 启动补齐：`agent/service/agent/agent_repository.py`
- Skill 模型：`agent/schema/model_skill.py`
- workspace 部署读写：`agent/service/capability/skills/skill_workspace_store.py`

## 17. 最终结论

后续要长期坚持这条原则：

- skill 的执行真相源是文件系统
- skill 的安装真相源也是文件系统
- public skill 和 internal skill 必须显式分层
- `memory-manager` 是对外可见系统 skill
- `nexus-manager` 是主智能体内部 skill，不对外暴露
- 主智能体身份只由后端配置决定，skill 文档和 UI 都只能引用这份配置
- `skill` 专用能力的目标态是不依赖数据库，不再维护资源池和挂载关系表

如果未来 skill 体系继续演进，应当在本文档上增量扩展，而不是重新发明第二套语义。
