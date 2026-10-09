# Nexus Skill 模型与运行时规范

## 1. 核心定义

Skill 有三层状态，不能混成一个“安装状态”：

1. **来源（source）**：Skill 文件由谁提供、存在哪个受管目录。
2. **Agent 绑定（binding）**：当前 Agent 是否允许使用一个全局 Skill，或是否明确停用一个本地 Skill。
3. **运行时投影（runtime projection）**：启动 nxs 或 Claude Code 时，将来源目录和绑定状态转换成 SDK 参数。

- 全局技能库：当前用户拥有、可管理的 Skill 目录（“用户有哪些 Skill”），不等于所有 Agent 都启用。
- Agent 设置页：启停 Skill 的唯一入口（“这个 Agent 用哪些 Skill”）；每个 Agent 可以有不同的启用集合。

## 2. 来源与归属

| 来源 | 文件真相源 | 全局技能库 | Agent 设置页 | 默认状态 | 允许的管理动作 |
| --- | --- | --- | --- | --- | --- |
| 系统内置 | Nexus 产品 `skills/<name>/` | 可见 | 可见但锁定 | 由平台默认配置决定 | 只读 |
| Nexus 平台 Skill | 产品 `skills/<name>/`，同步到 `<config>/platform-skills` | 可见 | 可按 Agent 启停 | 未绑定即停用 | 启用、停用 |
| 宿主全局 Skill | 桌面用户 `~/.agents/skills/<name>/`，同步到 `<config>/host-skills` | 可见 | 可按 Agent 启停 | 未绑定即停用 | 启用、停用 |
| 用户导入 / 第三方市场 | `<workspace>/<owner>/workspace/.agents/skills/<name>/` | 可见 | 可按 Agent 启停 | 未绑定即停用 | 导入、更新、删除、启用、停用 |
| Agent 工作区 Skill | `<agent workspace>/.agents/skills` 或 `.claude/skills` | 不可见 | 仅所属 Agent 可见 | 文件存在即启用 | 启用、停用、删除 |

- 同名投影优先级固定为：Agent 工作区 > 系统/平台 > 桌面宿主 > owner 导入。后一层不能用 frontmatter `name` 绕过前一层；需要并存时必须在来源目录阶段使用不同的 canonical name。
- Room 不是文件来源，而是 `scope: room` 使用范围。Room Skill 仍来自平台、宿主或用户导入源并出现在全局技能库中，但不能绑定到单个 Agent，只能在 Room 配置中选择，不写入 Agent 的 Skill 字段。
- 系统内置 Skill 由平台托管，不能手动删除或切换。
- internal Skill 不得混进公开第三方市场。

`source_type` 描述来源类别：

- `system`：系统内置；
- `builtin`：平台或宿主提供的全局 Skill；
- `external`：用户导入或第三方市场导入；
- `workspace`：当前 Agent 工作区内的私有 Skill。

`source_kind`、`storage_scope`、`origin_kind` 补充来源、存储范围和创建归属，不能替代 Agent 的启用状态。

## 3. 目录边界

全局宿主目录只接受两类：产品随包的 `skills/` 和桌面用户的 `~/.agents/skills/`。

- **不扫描** `~/.codex/skills`、宿主 `~/.claude/skills`、`~/.cc-switch/skills` 或其他未声明的外部 Skill 目录。
- `.claude/skills` 只是受管兼容根中为 Claude Code 提供的、与 `.agents/skills` 相同内容的发现入口，不是第二个来源。

发现规则：

- 每个发现根只接受一层 `skill-name/SKILL.md`；canonical name 始终是 `skill-name` 目录名。
- frontmatter 或导入 manifest 中的 `name` 只作展示元数据，不改写绑定键。
- 目录名不得包含首尾空白，不得使用持久化引用保留前缀 `external:`；绑定键按大小写不敏感去重。
- 宿主根允许顶层目录链接表示一个 Skill：目标必须位于当前用户 home 内，目标根直接包含真实 `SKILL.md`，Skill 内部不允许二次链接或特殊文件。
- 嵌套的 bundle/collection 不是发现单元，不得递归展开为 Skill。未来如引入 connector 层，应在该层导入时展开并命名，不改变 canonical 根的直接子目录契约。

Agent workspace 是独立的动态层：读取 nxs/Codex 原生的 `.agents/skills/<name>` 和 Claude Code 原生的 `.claude/skills/<name>`。这不等于扫描宿主 `~/.claude/skills`，也不支持无 `skills` 层级的 `.agents/<name>` 伪根。

平台源和宿主源分别同步到：

```text
<config>/platform-skills/.agents/skills
<config>/platform-skills/.claude/skills
<config>/host-skills/.agents/skills
<config>/host-skills/.claude/skills
```

- 平台源使用内容指纹和整库分阶段发布。
- 宿主源在服务启动时先创建稳定的 `<config>/host-skills/.agents/skills` 目录，再由后台 watcher 有界校验并按 Skill 分阶段替换。单个 Skill 更新失败时保留该项 last-known-good，不阻断同级 Skill 刷新；源中明确删除的 Skill 从投影删除。
- Catalog 与 runtime 都只读取这一受管投影，不各自重新扫描宿主 home。
- 全局 Skill 不复制到 Agent workspace 作为绑定真相；workspace 中出现的文件一律按该 Agent 的本地来源处理。

用户导入的 Skill 按 owner 隔离：

```text
<workspace>/<owner>/workspace/.agents/skills/<skill_name>
<workspace>/<owner>/workspace/.claude/skills -> ../.agents/skills
```

Agent 的私有目录为：

```text
<workspace>/<owner>/workspace/<agent_workspace>/.agents/skills/<skill_name>
<workspace>/<owner>/workspace/<agent_workspace>/.claude/skills/<skill_name>
```

## 4. 持久化真相源

### 4.1 全局 Skill

- 文件真相源：平台、宿主或 owner 全局目录中的 Skill 文件；
- Agent 绑定真相源：`Agent.options.skill_ids`；
- 平台/宿主 Skill 使用 canonical name，例如 `ima-skill`；
- 用户导入 Skill 使用 `external:<canonical-name>`；
- `skill_ids` 不保存路径，也不因运行时同步产生 workspace 副本。

全局 Skill 没有“安装副本”：启用只把引用写入当前 Agent 的 `skill_ids`，停用只移除该引用，源文件仍留在全局技能库中。

### 4.2 Agent 工作区 Skill

- 文件真相源：所属 Agent workspace 中的 Skill 文件；
- Agent 状态真相源：`Agent.options.disabled_skill_ids`；
- 文件被发现时默认启用；
- 停用只写入该 Agent 的 canonical name，不删除文件；
- 重新启用只移除该 Agent 的停用项；
- 不进入全局技能库、全局使用统计或其他 Agent 的设置页。

`disabled_skill_ids` 只表达本地动态 Skill 的显式停用。全局 Skill 不在 `skill_ids` 中即表示停用，切换全局 Skill 不得写入 `disabled_skill_ids`。

### 4.3 目录与界面投影

catalog、Agent 列表和详情页都是上述真相源的投影：

- `enabled_for_agent` 是当前 Agent 视角下的计算字段；
- `enabled_agent_count` 只统计全局 `skill_ids` 引用；
- Agent 本地 Skill 只在所属 Agent 的列表中追加；
- 同名本地 Skill 在 Agent 列表中覆盖全局目录展示，但不会删除或改写全局绑定。

## 5. Agent 级交互

### 5.1 全局技能库


- 浏览来源、版本、信任级别和 Agent 使用数；
- 导入本地压缩包、Git 或第三方市场 Skill；
- 更新或删除用户导入的源文件；
- 在详情页查看所有 Agent 的启用矩阵。

全局页面不显示 Agent workspace Skill，也不提供“把 Skill 安装到某个
workspace”的概念。

### 5.2 Agent 设置页


- **已启用**：当前 Agent 绑定的全局 Skill，加上默认启用且未被停用的本地 Skill；
- **可启用**：当前 Agent 可见但尚未启用的全局 Skill；
- 本地 Skill 显示“Agent 本地”标记，只能在所属 Agent 设置页操作；
- 每次切换只更新目标 Agent，不携带整个 Agent 草稿，避免旧快照覆盖其他设置。

Composer 的 `/skills` 选择器同时展示上述两组。可启用但未绑定的 Skill 标记为“单次使用”：用户明确选择后只对当前轮加载，不自动打开 Agent 设置页，也不修改长期启用状态（Slash 执行见 [slash-command-spec.md](slash-command-spec.md)）。

### 5.3 同名 Skill

全局 Skill 与 Agent 本地 Skill 同名时：

1. Agent 设置页展示本地 Skill，因为本地来源优先；
2. 全局详情页的 Agent 矩阵始终操作全局来源；
3. PATCH 必须传 `target_scope`，不能仅靠名称修改同名来源；
4. `global_library` 与 `agent_workspace` 的开关互不改写对方的持久化字段；
5. 运行时按 canonical name 合并来源；本地 Skill 的显式停用会阻止该名称的
   有效调用，但不会移除全局 `skill_ids` 绑定。

## 6. API 约定

Agent 列表返回 `enabled_for_agent`，不再使用“installed”作为产品语义。

```http
GET /agents/{agent_id}/skills
```

返回该 Agent 可见的全局 Skill 和私有 workspace Skill。

```http
PATCH /agents/{agent_id}/skills/{skill_name}
Content-Type: application/json

{
  "enabled": true,
  "target_scope": "global_library"
}
```

`target_scope` 取值：

- `global_library`：读写 `options.skill_ids`；
- `agent_workspace`：读写 `options.disabled_skill_ids`。

- 详情页使用 `GET /skills/{skill_name}/agents` 读取全局 Skill 的 Agent 启用矩阵，并始终传 `target_scope=global_library`。
- 旧的 POST/DELETE Agent Skill 路由仅作为兼容入口保留，新 UI 使用 PATCH；DELETE 对本地 Skill 仍表示删除 workspace 文件。

## 7. 运行时装配

启动 runtime 前，宿主完成以下步骤：

1. 使用已发布的平台根、稳定宿主快照和当前 owner 全局根；
2. 从 `skill_ids` 将 `external:<name>` 还原为 canonical name；
3. 从 Agent workspace 动态发现本地 Skill；
4. 将全局绑定名称与未停用的本地名称传给 nxs；
5. 将全局兼容根和 owner 根作为 additional directories；
6. 为 Claude Code 使用全量发现加 deny 规则，拒绝未绑定的全局 Skill 和显式
   停用的本地 Skill。

- nxs 的显式白名单用于全局绑定，workspace Skill 按 CC 语义动态发现。
- 运行时计算出的拒绝列表可以包含未绑定的全局名称，但这只是本次投影，不回写 `disabled_skill_ids`。
- 用户显式选择 Skill 时，Composer 只把原始 `/skill-name args` 作为普通 user message 发送。平台源和 owner 源已通过 additional directories 暴露给 runtime；nxs 和 Claude Code 在 runtime 内展开 `$ARGUMENTS`、位置参数和 Skill 目录变量。解析路径、inline 正文与 `context: fork` 语义见 [slash-command-spec.md](slash-command-spec.md)。
- 显式调用不改变后续轮次的发现、拒绝集合或 Agent 持久设置。

### 7.1 内置控制 Skill 的分层契约

依赖 Nexus 控制面的内置 Skill 不共享一个宽泛的自动触发入口；信任边界和业务决策保持为独立顶层 Skill：

| 领域 | 控制入口 |
| --- | --- |
| Goal、Execution、Automation | 共享 round-scoped `nexus.command` |
| Subagent 委派 | 复用 Execution Skill 的按需 `references/subagents.md` 和 command 的 subagent domain；不增加独立常驻工具或顶层 Skill |
| Configuration | round-scoped `nexuscfg` |
| owner 平台资源管理 | 仅主智能体可见的 `nexusctl` |

- 一个领域被选中时，runtime 只需加载该领域的根 `SKILL.md`。
- 根 `SKILL.md` 只保留领域触发条件、bootstrap、不可由 schema 表达的不变量和 reference 路由；项目门禁将上述五个入口限制在 5 KiB。
- 按操作阶段拆分的 `references/` 只在当前动作需要时读取，不维护每次全量加载的 operation 大表。
- 精确 tool schema、request identity 与结果 envelope 由 `nexus.command` 自描述；operation 字段、枚举、required、集合上限和 parser contract 由 runtime command registry/schema 返回。
- Skill reference 只补充模型必须做出的跨字段选择、状态转换、恢复和权限边界，不复制代码已提供的完整 wire schema。
- authority、identity、revision 和服务端状态门槛必须在 adapter/service fail closed；Skill 规则不是授权来源。

### 7.2 默认产品说明 Skill

`nexus-product-guide` 是所有新建 Agent 默认绑定的系统内置 Skill；迁移为既有 Agent 补齐同一绑定。

- 职责：把用户目标映射到当前产品功能、可见界面入口、操作步骤、结果和当前限制；用户要求实际变更时，路由到 Goal、Execution、Automation、Configuration 或主智能体资源管理等专用 Skill。
- 覆盖：会话与固定入口、Agent 与 Room、Goal 与命名 WorkGraph、Echo、Automation、Browser、Skill、Connector、外部消息和设置。分主题 reference 按需加载；维护来源表把每项说明追溯到当前 UI 与现行产品规范。
- 它不是产品协议、权限或运行状态的真相源，不得根据手册宣称能力已经启用、连接或执行成功；具体状态以当前 UI、服务端读取结果和对应领域合同为准。
- 内容必须先确认当前实现，再追求覆盖完整；不得把未来方案或未挂载入口写成已交付能力。
- 对用户优先使用用途、入口和普通操作语言；不先暴露 Session、distillation、CDP、wire schema 等内部术语，也不暴露内部 URL 或 ID。
- 窄屏、角色、权限和前置状态造成入口差异时，以条件式导航解释，不虚构固定布局。

## 8. 生命周期语义

| 动作 | 全局 Skill | Agent workspace Skill |
| --- | --- | --- |
| 导入 | 写入 owner 全局源和 manifest | 不适用 |
| 启用 | 加入当前 Agent `skill_ids` | 清除当前 Agent 的停用项 |
| 停用 | 从当前 Agent `skill_ids` 移除 | 写入当前 Agent `disabled_skill_ids`，保留文件 |
| 显式单次使用 | runtime 展开当前轮，不改写 `skill_ids` | runtime 展开当前轮，不改写 `disabled_skill_ids` |
| 更新 | 原子替换 owner 源，所有已绑定 Agent 自然读取新版本 | 修改所属 Agent 文件 |
| 删除 | 仅用户导入源可删除；同时清理全局绑定 | 删除所属 Agent workspace 文件 |

## 9. 自定义私有来源

### 9.1 目标与边界

企业通过自建 HTTPS 服务向 Nexus 分发不适合放入公开 GitHub 或社区市场的 Skill。第一版只解决以下闭环：

1. 用户在技能来源面板添加一个私有服务器；
2. Nexus 使用用户保存的访问凭据搜索该服务器；
3. 用户预览并将某个 Skill 导入 owner 全局技能库；
4. Nexus 按远端内容摘要检查更新，经用户确认后原子替换本地版本；
5. 已绑定该 Skill 的 Agent 保留原有绑定，下一轮自然读取新版本。

- 这是 Nexus 来源适配协议，不改变 Skill 包内容规范；下载的 zip 仍必须是包含 `SKILL.md` 的 Agent Skills 目录。
- 第一版不包含自动静默更新、服务端推送、依赖解析、跨来源联邦、mTLS、自定义 CA、跨域下载或 Claude Plugin Marketplace 兼容；这些只能作为独立扩展，不进入最小协议。
- 协议采用一个服务端搜索接口（同 `claude-plugins.dev`、`skills.sh`、`clawhub.ai` 的 `q` 搜索模式）加一个 zip 下载接口，而非 Hermes Skills Index、`browse.sh` 那样由 Nexus 本地过滤的全量 JSON 索引。

### 9.2 私有服务器协议

用户配置的 `url` 是私有来源 base URL。Nexus 去掉末尾 `/` 后追加 `/api/skills`：

```text
来源 URL: https://skills.example.internal/registry
搜索 URL: https://skills.example.internal/registry/api/skills
```

#### 搜索和精确查询

```http
GET /registry/api/skills?q=知识&limit=20        # 搜索
GET /registry/api/skills?id=internal-knowledge  # 检查已安装 Skill 的当前版本
Authorization: Bearer <token>
Accept: application/json
```

约定：

- `q` 和 `id` 互斥；
- `q` 是 UTF-8 查询，服务端按 `name`、`title`、`description` 和 `tags`
  匹配；
- `id` 是大小写敏感的精确查询，必须返回零或一条；
- 两者都不传时返回默认列表；
- `limit` 默认为 20，最大为 100；
- 搜索结果按相关度降序，相关度相同时按 `name` 升序；
- 第一版不做 cursor/offset 分页，响应最多返回 `limit` 条；
- `total` 是应用 `limit` 前的匹配总数。

响应：

```json
{
  "skills": [
    {
      "id": "internal-knowledge",
      "name": "internal-knowledge",
      "title": "内部知识助手",
      "description": "组织内部制度、产品资料和常见问题检索指南。",
      "version": "1.3.0",
      "tags": ["knowledge", "internal", "assistant"],
      "download_url": "api/skills/internal-knowledge/download",
      "sha256": "8c0f7cf04741b6be4e3b5c0b0c6bdf6a5ae8231aa3d4ab8e9fb4d9f5a3c2e117",
      "size": 182340,
      "readme_markdown": ""
    }
  ],
  "total": 1
}
```

| 字段 | 必填 | 约定 |
| --- | --- | --- |
| `id` | 是 | 来源内稳定身份，发布后不得复用给另一个 Skill |
| `name` | 是 | canonical name，必须通过 Nexus Skill 名称校验；同一 `id` 发布后不得变更 |
| `title` | 是 | 用户可见标题 |
| `description` | 是 | 搜索摘要 |
| `version` | 是 | 非空展示版本；更新判断不依赖 SemVer 比较 |
| `tags` | 否 | 字符串数组，最多 32 项 |
| `download_url` | 是 | zip 下载地址，必须与来源 base URL 同源，且不得在 URL 中嵌入凭据或查询参数 |
| `sha256` | 是 | zip 原始字节的 SHA-256，64 位小写十六进制 |
| `size` | 否 | zip 字节数，用于进度和提前拒绝超限包 |
| `readme_markdown` | 否 | 已授权用户可见的预览内容，最大 2 MiB |

`title`、`description`、`tags` 和 `readme_markdown` 只是搜索与预览投影；导入后 `SKILL.md` 是内容真相源。

#### 下载

`download_url` 可以是绝对 HTTPS URL 或相对 URL；Nexus 始终相对归一化后的 `<base_url>/` 解析。上例解析为：

```http
GET /registry/api/skills/internal-knowledge/download
Authorization: Bearer <token>
Accept: application/zip
```

成功响应必须为 `200 OK`，内容是单个 zip Skill 包。Nexus 不依赖 `ETag` 或 `Content-Length` 判断完整性，而是对实际读取的字节重新计算 SHA-256。

包结构：

```text
internal-knowledge/
├── SKILL.md
├── scripts/
├── references/
└── assets/
```

包内必须只有一个可发现 Skill；顶层目录名、`SKILL.md` frontmatter `name` 与响应 `name` 必须一致。

### 9.3 Nexus 来源管理 API

私有来源与默认社区来源共用 `skill_sources`，但必须区分系统配置和用户创建的记录（`managed_by`，见 9.6）。

创建（`GET /skills/sources` 列表）：

```http
POST /skills/sources
Content-Type: application/json

{
  "name": "内部技能库",
  "url": "https://skills.example.internal/registry",
  "auth_type": "bearer",
  "token": "secret-token"
}
```

创建时 Nexus 先请求 `/api/skills?limit=1`；只有远端返回成功状态且响应通过结构校验后才持久化来源。

修改展示名、开关或轮换凭据：

```http
PATCH /skills/sources/{source_id}
Content-Type: application/json

{
  "name": "内部技能库",
  "enabled": true,
  "auth_type": "bearer",
  "token": "new-secret-token"
}
```

- `token` 缺省表示保留已存凭据。`auth_type` 更新为 `none` 时必须同时删除已存凭据。
- 来源 `url` 创建后不可修改；更换地址必须删除后重新创建，避免已信任的来源身份被无感指向另一个服务器。

删除用户来源：`DELETE /skills/sources/{source_id}`。

- 只删除配置与凭据，不删除已导入 Skill，也不修改 Agent 绑定；这些 Skill 仍可离线使用，检查更新时明确报告来源不存在。
- 系统和部署配置来源不可通过该端点删除。

`GET /skills/sources` 对私有来源的投影：

```json
{
  "source_id": "skill_src_...",
  "name": "内部技能库",
  "kind": "private_registry",
  "url": "https://skills.example.internal/registry",
  "trust": "private",
  "managed_by": "user",
  "auth_type": "bearer",
  "credential_configured": true,
  "enabled": true,
  "deletable": true,
  "last_checked_at": null,
  "last_error": ""
}
```


### 9.4 搜索边界与隐私

外部搜索接口支持可选的服务端作用域：

```http
GET /skills/search/external?q=知识&source_id={source_id}
```

- 传入 `source_id` 时只能请求当前 owner 的该来源；
- 来源已停用、不存在或不属于当前 owner 时拒绝请求；
- 不传 `source_id` 时才执行多来源并发搜索；单个私有来源失败不阻断其他已启用来源返回结果；
- 前端来源筛选必须下推到该参数，不能先搜索全部来源再在浏览器过滤；选中私有来源后只向后端传递该 `source_id`，视图不自行持有或修正认证状态；
- 用户选择私有来源时，搜索词不得发送给任何公共社区来源；
- UI 的“全部来源”是用户明确选择的联合检索，不是私有筛选的展示别名。

私有来源搜索结果投影为 `ExternalSkillSearchItem`，`source_kind` 固定为 `private_registry`，`source_trust` 固定为 `private`。前端复用现有卡片、来源筛选和预览弹窗，不建立私有市场页。

### 9.5 导入与更新

信任边界不能是“前端回传完整下载 URL”。前端只提交稳定身份：

```http
POST /skills/import/source
Content-Type: application/json

{
  "source_id": "skill_src_...",
  "skill_id": "internal-knowledge"
}
```

- 后端必须用当前 owner 的来源凭据重新请求 `/api/skills?id=internal-knowledge`，取得当前 `download_url` 和 `sha256` 后才下载。
- 客户端传入的标题、信任级别、下载地址或内容摘要都不是授权依据。
- 同名冲突沿用现有封闭状态，不覆盖系统、平台或其他用户导入 Skill。
- 导入记录额外持久化 `source_skill_id` 与 `artifact_sha256`（见 9.6）。

检查更新时，Nexus 按 `source_id + source_skill_id` 重新查询远端：

- 远端 `sha256` 与 `artifact_sha256` 相同：已是最新；
- 两者不同：标记有更新；
- 远端 Skill 不存在或来源已删除：保留本地内容并记录明确失败原因；
- 来源请求失败：只影响该 Skill，错误按来源或 Skill 精确展示，不把批量结果投影为“暂无更新”；
- `version` 只用于展示，不实现 SemVer 排序、自动降级或静默更新。

更新沿用用户全局 Skill 分阶段原子替换。替换成功后只刷新 owner 技能库和导入记录，不重写 Agent `skill_ids`；已绑定 Agent 下一轮自然读取新版本。

### 9.6 持久化

`skill_sources` 增加：

| 字段 | 说明 |
| --- | --- |
| `managed_by` | `system` 或 `user`；只有 `user` 来源可删除 |
| `auth_type` | `none` 或 `bearer` |
| `credentials_encrypted` | 服务端保存的 Bearer Token 明文（沿用旧字段名），空值表示无认证 |

`imported_skills` 增加：

| 字段 | 说明 |
| --- | --- |
| `source_skill_id` | 私有来源内稳定 Skill ID |
| `artifact_sha256` | 已安装 zip 的完整内容摘要 |

- 用户来源的 `source_id` 由归一化后的 `kind + url` 确定，所有读写都带 `owner_user_id`；两个 owner 可以对同一 URL 配置不同 token，搜索结果与凭据互不可见。
- 用户删除后重新添加同一 URL 时恢复同一 `source_id`，既有导入记录恢复更新能力。
- 环境变量配置的来源为 `managed_by=system`。来源列表和实际搜索不以“必须存在于进程配置”过滤数据库中的用户来源；系统来源与用户来源在存储层归一后，再按 `managed_by` 投影管理能力。
- SQLite 与 Postgres 使用同一领域语义，新增字段和 owner 约束在两类迁移中保持一致。

### 9.7 凭据和出站安全

- 当前只支持 `none` 和 `bearer`；
- bearer token 沿用 Provider 存储模型，由服务端明文保存，不依赖
  `CONNECTOR_CREDENTIALS_KEY`；该边界必须在 UI 和部署文档中明确；
- 前端永远不接收 token，搜索、预览、导入和更新都由 Go 服务端添加
  `Authorization` 头；
- 来源 base URL、搜索 URL 与 `download_url` 必须保持同源；Nexus 不跟随远端重定向；
- 生产来源只允许 HTTPS，不提供“忽略 TLS 校验”开关；
- 桌面本地模式允许当前用户显式配置的 HTTPS 内网地址；
- 认证的 Web 部署只能访问管理员配置的私有来源 host 白名单，防止普通用户将
  Nexus 变成内网请求代理；
- token、`Authorization` 头和含凭据的错误不得进入 API 响应、应用日志、WebSocket 事件、来源 `last_error`、浏览器状态或前端反馈；
- 记录远端失败时只保留状态码与脱敏原因。

- 私有 zip 下载受 32 MiB 压缩内容上限保护，并限制解压后总字节数、单文件字节数与条目数。
- 绝对路径、`..`、符号链接、硬链接或特殊文件必须在写入 owner 技能库之前被拒绝。
- 远端超时、非法 JSON、摘要不符、非法 zip，以及下载、校验或解压失败时，保留 last-known-good 目录。

### 9.8 前端交互

“管理技能来源”弹窗提供“添加自定义来源”主动作，表单只包含：

```text
来源名称
服务器地址
认证方式：无认证 / Bearer Token
Token（仅 Bearer Token 时显示）
```

- 提交按钮文案为“验证并添加”。
- 私有来源行显示“私有”、“已配置凭据”或“无认证”标记，并提供开关、更换凭据和删除动作；系统来源只提供开关。
