# Nexus Skill 模型与运行时规范

## 1. 核心定义

Skill 有三层状态，不能混成一个“安装状态”：

1. **来源（source）**：Skill 文件由谁提供、存在哪个受管目录。
2. **Agent 绑定（binding）**：当前 Agent 是否允许使用一个全局 Skill，或是否明确停用一个本地 Skill。
3. **运行时投影（runtime projection）**：启动 nxs 或 Claude Code 时，将来源目录和绑定状态转换成 SDK 参数。

“全局技能库”是当前用户可以管理的 Skill 目录，不等于所有 Agent 都启用。
Agent 的设置页才是启停 Skill 的入口；每个 Agent 可以有不同的启用集合。

## 2. 来源与归属

| 来源 | 文件真相源 | 全局技能库 | Agent 设置页 | 默认状态 | 允许的管理动作 |
| --- | --- | --- | --- | --- | --- |
| 系统内置 | Nexus 产品 `skills/<name>/` | 可见 | 可见但锁定 | 由平台默认配置决定 | 只读 |
| Nexus 平台 Skill | 产品 `skills/<name>/`，同步到 `<config>/platform-skills` | 可见 | 可按 Agent 启停 | 未绑定即停用 | 启用、停用 |
| 宿主全局 Skill | 桌面用户 `~/.agents/skills/<name>/`，同步到 `<config>/host-skills` | 可见 | 可按 Agent 启停 | 未绑定即停用 | 启用、停用 |
| 用户导入 / 第三方市场 | `<workspace>/<owner>/workspace/.agents/skills/<name>/` | 可见 | 可按 Agent 启停 | 未绑定即停用 | 导入、更新、删除、启用、停用 |
| Agent 工作区 Skill | `<agent workspace>/.agents/skills` 或 `.claude/skills` | 不可见 | 仅所属 Agent 可见 | 文件存在即启用 | 启用、停用、删除 |

Room 不是一种文件来源，而是 `scope: room` 使用范围。Room Skill 仍来自平台、
宿主或用户导入源，继续出现在全局技能库中，但不能绑定到单个 Agent，只能在
Room 配置中选择。

`source_type` 描述来源类别：

- `system`：系统内置；
- `builtin`：平台或宿主提供的全局 Skill；
- `external`：用户导入或第三方市场导入；
- `workspace`：当前 Agent 工作区内的私有 Skill。

`source_kind`、`storage_scope`、`origin_kind` 用于补充来源、存储范围和创建归属，
不能替代 Agent 的启用状态。

## 3. 目录边界

Nexus 只接受以下两类全局宿主目录：

- 产品随包的 `skills/`；
- 桌面用户的 `~/.agents/skills/`。

Nexus **不会扫描** `~/.codex/skills`、`~/.claude/skills`、`~/.cc-switch/skills`
或其他外部 Skill 目录。`.claude/skills` 只是在受管兼容根中为 Claude Code 提供
与 `.agents/skills` 相同内容的发现入口，不是第二个来源。

平台源和宿主源会分别同步到：

```text
<config>/platform-skills/.agents/skills
<config>/platform-skills/.claude/skills
<config>/host-skills/.agents/skills
<config>/host-skills/.claude/skills
```

同步使用源内容指纹、临时目录和原子替换。Nexus 不把全局 Skill 复制到每个
Agent workspace；workspace 中出现的文件一律按该 Agent 的本地来源处理。

用户导入的 Skill 按 owner 隔离，目录为：

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

全局 Skill 没有“安装副本”。启用只是把引用写入当前 Agent 的
`skill_ids`；停用只是移除该引用，源文件仍留在全局技能库中。

### 4.2 Agent 工作区 Skill

- 文件真相源：所属 Agent workspace 中的 Skill 文件；
- Agent 状态真相源：`Agent.options.disabled_skill_ids`；
- 文件被发现时默认启用；
- 停用只写入该 Agent 的 canonical name，不删除文件；
- 重新启用只移除该 Agent 的停用项；
- 不进入全局技能库、全局使用统计或其他 Agent 的设置页。

`disabled_skill_ids` 只表达本地动态 Skill 的显式停用。全局 Skill 不出现在
`skill_ids` 即表示停用，切换全局 Skill 不应写入 `disabled_skill_ids`。

### 4.3 目录与界面投影

catalog、Agent 列表和详情页都是上述真相源的投影：

- `enabled_for_agent` 是当前 Agent 视角下的计算字段；
- `enabled_agent_count` 只统计全局 `skill_ids` 引用；
- Agent 本地 Skill 只在所属 Agent 的列表中追加；
- 同名本地 Skill 在 Agent 列表中覆盖全局目录展示，但不会删除或改写全局绑定。

## 5. Agent 级交互

### 5.1 全局技能库

全局技能库负责管理“用户拥有的 Skill”：

- 浏览来源、版本、信任级别和 Agent 使用数；
- 导入本地压缩包、Git 或第三方市场 Skill；
- 更新或删除用户导入的源文件；
- 在详情页查看所有 Agent 的启用矩阵。

全局页面不显示 Agent workspace Skill，也不提供“把 Skill 安装到某个
workspace”的概念。

### 5.2 Agent 设置页

Agent 设置页负责管理“这个 Agent 使用什么”：

- **已启用**：当前 Agent 绑定的全局 Skill，加上默认启用且未被停用的本地 Skill；
- **可启用**：当前 Agent 可见但尚未启用的全局 Skill；
- 本地 Skill 显示“Agent 本地”标记，只能在所属 Agent 设置页操作；
- 每次切换只更新目标 Agent，不携带整个 Agent 草稿，避免旧快照覆盖其他设置。

### 5.3 同名 Skill

全局 Skill 与 Agent 本地 Skill 同名时：

1. Agent 设置页展示本地 Skill，因为本地来源优先；
2. 全局详情页的 Agent 矩阵始终操作全局来源；
3. PATCH 必须传 `target_scope`，不能仅靠名称猜测目标；
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

详情页使用 `GET /skills/{skill_name}/agents` 读取全局 Skill 的 Agent 启用矩阵，
并始终传 `target_scope=global_library`。旧的 POST/DELETE Agent Skill 路由仅作为
兼容入口保留，新 UI 使用 PATCH；DELETE 对本地 Skill 仍表示删除 workspace 文件。

## 7. 运行时装配

启动 runtime 前，宿主完成以下步骤：

1. 同步平台、宿主和 owner 全局兼容根；
2. 从 `skill_ids` 将 `external:<name>` 还原为 canonical name；
3. 从 Agent workspace 动态发现本地 Skill；
4. 将全局绑定名称与未停用的本地名称传给 nxs；
5. 将全局兼容根和 owner 根作为 additional directories；
6. 为 Claude Code 使用全量发现加 deny 规则，拒绝未绑定的全局 Skill 和显式
   停用的本地 Skill。

nxs 的显式白名单用于全局绑定，workspace Skill 按 CC 语义动态发现。运行时计算
出的拒绝列表可以包含未绑定的全局名称，但这只是本次运行时投影，不回写
`disabled_skill_ids`。

## 8. 生命周期语义

| 动作 | 全局 Skill | Agent workspace Skill |
| --- | --- | --- |
| 导入 | 写入 owner 全局源和 manifest | 不适用 |
| 启用 | 加入当前 Agent `skill_ids` | 清除当前 Agent 的停用项 |
| 停用 | 从当前 Agent `skill_ids` 移除 | 写入当前 Agent `disabled_skill_ids`，保留文件 |
| 更新 | 原子替换 owner 源，所有已绑定 Agent 自然读取新版本 | 修改所属 Agent 文件 |
| 删除 | 仅用户导入源可删除；同时清理全局绑定 | 删除所属 Agent workspace 文件 |

系统内置 Skill 由平台托管，不能手动删除或切换。Room Skill 由 Room 配置管理，
不写入 Agent 的 Skill 字段。

## 9. 禁止项

- 把全局 Skill 复制到每个 Agent workspace 作为绑定真相；
- 用 `disabled_skill_ids` 表示全局 Skill 的停用；
- 用 Skill 名称而不带 `target_scope` 修改同名来源；
- 把路径写入 `skill_ids`；
- 扫描 `~/.codex`、宿主 `~/.claude`、`.cc-switch` 或未声明的外部目录；
- 把 Agent workspace Skill 放进全局技能库或其他 Agent 的设置页；
- 把 internal Skill 混进公开第三方市场。

## 10. 一句话总结

全局技能库解决“用户有哪些 Skill”，Agent 设置解决“这个 Agent 用哪些 Skill”；
全局绑定写 `skill_ids`，Agent 本地停用写 `disabled_skill_ids`，workspace-local
Skill 默认只对自己的 Agent 可见，运行时再把三者投影为 nxs/Claude 的发现与权限。
