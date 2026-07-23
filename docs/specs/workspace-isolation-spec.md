# Workspace 隔离与多用户运行时规范

## 1. 文档状态

- 状态：目录布局与迁移第一阶段已实施；OS UID/GID、Hook 和最终系统调用校验仍待后续阶段
- 日期：2026-07-23
- 适用范围：Linux 服务端多用户部署
- 当前结论：以操作系统 UID/GID 为主边界，项目组/ACL 负责显式协作，runtime hook 和最终路径校验负责策略收口；`.nexus` 是统一状态根，`app/` 保存 Nexus 宿主数据，runtime 配置和会话按用户独立存放

本文定义安全边界和运行契约；当前提交已经实现状态根/用户 workspace/runtime 的布局、启动迁移和 nxs/Claude 环境注入，但不宣称已经完成 OS UID/GID、Hook 或最终系统调用级隔离。

## 2. 目标与非目标

### 2.1 目标

1. 用户 A 的 runtime 不能通过文件工具、Shell、nxs、Claude 或普通进程直接读取或修改用户 B 的 workspace。
2. 同一用户的多个 Agent 可以继续共享该用户被授予的资源。
3. 跨用户协作只能通过显式项目成员关系授予，不依赖路径猜测或模型自律。
4. 在允许的 workspace 内保持无额外确认的正常开发体验。
5. nxs 和 Claude 使用同一份用户身份、配置根与 workspace policy。
6. server、runtime、控制面数据和用户数据拥有清晰的权限边界。
7. App 与 Web 使用同一套用户/租户模型；App 只是自动登录的单用户部署。

### 2.2 非目标

- 防御宿主 root、容器运行时、内核或文件系统本身被攻破。
- 用文件权限替代 HTTP、WebSocket、`nexusctl` 和 storage 层的 owner 授权。
- 第一阶段实现每个 Agent 一个操作系统用户。
- 第一阶段隔离网络、CPU、内存和系统调用；这些属于后续 sandbox/cgroup 能力。
- 在 macOS/Windows 桌面端强制创建本地系统用户。
- 第一阶段引入独立的组织/成员层级；本阶段的租户边界就是 `owner_user_id`。

## 3. 威胁模型

### 3.1 纳入的攻击者

- 能控制 Agent 提示词、Skill、project hook 或 Bash 输入的模型/运行时进程。
- 用户 A 试图读取用户 B 的 workspace、transcript、配置或缓存。
- 被误配置的 runtime 通过绝对路径、相对路径、符号链接、Shell 展开或 `nexusctl` scope 访问越界资源。

### 3.2 不纳入的攻击者

- 已获得 `nexus-host` 或宿主 root 权限的攻击者。
- 利用 Docker、Linux 内核、文件系统驱动或硬件漏洞逃逸的攻击者。
- 仅通过业务 API 伪造 owner 身份的攻击者；这类问题必须由控制面授权单独阻断。

### 3.3 安全结论

在“宿主和内核可信、runtime 不可信”的前提下，独立 UID/GID 是跨用户文件隔离的主边界。Hook 只能作为防御纵深和审计入口，不能代替 DAC、ACL 或最终系统调用边界。

## 4. 核心原则

1. **身份先于路径**：workspace 路径只是定位信息，最终权限由 OS 身份和文件系统权限决定。
2. **默认拒绝**：没有明确授予的用户、组、路径和操作均拒绝。
3. **宿主托管**：安全策略、运行身份、环境变量和启动参数由 Nexus 宿主生成；用户配置、Skill 和 project hook 不能降低安全边界。
4. **宿主根与用户根分区**：`.nexus` 是 Nexus 的统一状态命名空间；其中 `app/` 只保存宿主控制面和宿主共享资源，nxs/Claude 的 config、`projects`、`HOME`、cache、tmp 和用户 workspace 只能进入 `users/<owner_user_id>/`。
5. **协作显式化**：共享目录必须有项目成员关系；加入项目组意味着成员之间互相信任并可按项目权限操作。
6. **身份稳定**：产品用户到 OS UID/GID 的映射必须持久化，不能因为重启、恢复或用户删除而意外复用。
7. **平台诚实**：只在具备可靠 POSIX 权限语义的部署上承诺该隔离等级。
8. **端无关**：桌面 App 的本地免登录只是认证适配器，不能形成第二套 owner、workspace 或 runtime 规则。

## 5. 身份模型

### 5.1 产品用户与运行身份

每个产品 `owner_user_id` 对应一个产品用户、一个用户数据根和一个 runtime OS identity；同一用户的多个 Agent 默认复用该身份。

```text
UserScope
  owner_user_id
  surface                 # app | web，仅用于认证/展示，不参与授权
  auth_subject
  user_root
  workspace_root
  runtime_root

RuntimeIdentity
  owner_user_id
  uid
  private_gid
  supplementary_gids
  home_dir
  temp_dir
  status
  generation
```

约束：

- `uid`、`private_gid` 和目录路径由宿主生成，不直接使用用户输入。
- 账号无密码、不可交互登录、无 sudo 权限，默认 shell 为 `nologin`。
- 映射记录持久化在 `app/data` 中。
- 删除用户后，只有在其文件完成清理或迁移后才允许回收 UID/GID。
- `supplementary_gids` 默认为空；仅加入当前 session 明确需要的项目组。
- `owner_user_id` 是所有控制面查询、workspace、Agent、Room、凭据、Skill、automation 和 runtime 目录的统一归属键。
- Web 登录用户直接生成 `UserScope`；App 启动时自动绑定现有 `SystemUserID` 对应的本地用户，仍然生成同一个 `UserScope`。
- `surface` 不能被业务层当作授权条件；不得出现“桌面走 system scope、Web 走 owner scope”的双轨逻辑。
- 如果未来需要 Agent 级隔离，再引入 per-agent UID 或 Landlock policy，不改变本规范的用户级基线。

### 5.2 身份启动

Nexus server 不以 root 身份运行，也不直接把任意 UID/GID 参数交给 runtime。

runtime 必须通过受控 launcher 或 worker 启动。launcher 至少校验：

- 调用方是受信任的 `nexus-host`；
- runtime executable 位于固定 allowlist；
- `CWD` 位于该 identity 被授予的 workspace/project root；
- UID、主 GID、附加 GID 来自已持久化映射；
- 环境变量来自宿主 allowlist；
- 不允许调用方注入任意 `argv`、`LD_PRELOAD`、动态 loader 或额外文件描述符；
- 启动后立即丢弃不必要的 capability。

不允许为了省去 launcher 而让整个 Nexus server 以 root 运行，也不允许挂载 Docker socket 让 runtime 自己创建容器。

### 5.3 App 与 Web 统一租户模型

本阶段不为 App 和 Web 设计两套租户模型。两端都通过同一个 `UserScope` 进入业务层：

```text
UserScope
  owner_user_id
  user_root
  workspace_root
  runtime_root
  principal
```

- Web：登录 Session、Bearer Token 或其他认证适配器解析出 `principal`，再得到 `owner_user_id`。
- App：本地免登录适配器自动绑定现有 `SystemUserID` 对应的本地用户；它不是“无用户”或“全局 system scope”，只是恰好只有一个用户。
- Handler、service、repository、runtime launcher、Hook 和 transcript store 只消费 `UserScope`/`owner_user_id`，不判断 `app` 或 `web` 来决定权限。
- App 与 Web 的差异仅限认证方式、进程部署和 UI；Agent、workspace、Room、DM、provider、connector、Skill、automation、quota 和审计使用同一套归属规则。
- Web 后续如果增加组织/成员关系，再在 `owner_user_id` 之上增加 `tenant_id`；不回头引入 App 专属的第二套 owner 语义。
- 未认证部署也不等于全局管理员：HTTP/Web 请求在明确没有认证主体时只绑定
  `SystemUserID`；只有显式标记的内部维护上下文才允许无 owner 查询。这样可以让
  App 的单用户适配器和 Web 的认证适配器共享同一条 owner 过滤链，避免“缺少
  principal 就枚举所有用户”的越权回退。

## 6. 文件系统布局与权限

### 6.1 `.nexus` 作为统一状态根，`app/` 保存宿主数据

`nexus_state_root` 固定使用 `.nexus`。不再在里面重复创建 `.nexus` 子目录；宿主自己的控制面数据统一放在 `app/`，用户 runtime 放在 `users/<owner_user_id>/`。

`NEXUS_CONFIG_DIR` 和 `CLAUDE_CONFIG_DIR` 会产生大量属于 runtime 用户的文件；这些文件不能写入 `app/`，而应写入当前用户的 `<user_root>`：

```text
.nexus/                               # nexus_state_root
  app/                                # Nexus App 宿主根，nexus-host 私有
    data/                             # app DB、迁移状态
    config/                           # Nexus 配置、密钥
    logs/                             # server 日志
    cache/                            # 宿主共享 cache
    shared/                            # root-owned 只读 Skill、二进制、模板

  users/
    <owner_user_id>/                  # 当前用户数据根，private group
      workspace/
        <agent_id>/                   # 用户工作目录
      runtime/                        # <user_root>；NEXUS_CONFIG_DIR 与 CLAUDE_CONFIG_DIR 相同
        projects/                     # nxs/Claude transcript store
        .claude.json                  # Claude 全局配置（CLAUDE_CONFIG_DIR 直接父目录）
        settings.json                 # Claude 用户级 settings
        .claude/                      # Claude 用户级扩展与兼容文件
        home/                         # HOME；其他用户级工具文件
        cache/
        logs/
        tmp/

  shared-workspaces/
    <shared_workspace_id>/            # 项目 group/ACL 共享目录
```

桌面端默认使用 `~/.nexus`；Docker 可以把宿主目录挂载到 `/home/agent/.nexus`；服务端也可以把整个状态根映射到 `/var/lib/nexus`。`app/` 与 `users/` 必须是不同权限子树，但可以共用同一个 `.nexus` volume。

权限约束：

- `.nexus/app` 及其 `data/config/logs` 只允许 `nexus-host` 访问；runtime 通过宿主注入用户根，不能继承 app 根。
- `users/<owner_user_id>` 由对应 private group 持有；父目录只允许穿越，不允许 runtime 列举全部用户。
- workspace 和 shared workspace 使用 setgid；default ACL 只授予 Nexus 宿主、当前运行组和明确的项目组。
- 普通文件默认不允许 `other` 读写，runtime 使用 `umask 0007`。
- shared workspace 成员关系由 Nexus 控制，Agent 不能自行改组、改 ACL 或扩大 scope。

### 6.2 nxs 与 Claude 的用户级配置根

`projects` 是 runtime transcript store，不是用户协作项目目录。`<user_root>` 指当前用户专属的 runtime 根，nxs 和 Claude 都直接使用它：

```text
NEXUS_CONFIG_DIR=<user_root>
CLAUDE_CONFIG_DIR=<user_root>
HOME=<user_root>/home
```

nxs 和 Claude 都固定使用同一个 `<user_root>/projects`。Claude 不能独立配置 `projects` 子目录，但可以配置整个 `CLAUDE_CONFIG_DIR`；只要父级指向当前用户 runtime 根，就不构成隔离障碍。

bridge 可以继续把 `CLAUDE_CONFIG_DIR` 与 `NEXUS_CONFIG_DIR` 保持同步，但同步源必须是宿主按 `owner_user_id` 计算出的 `<user_root>`，不能继承 server 的 `.nexus/app` host root。

`NEXUS_CONFIG_DIR` 的语义需要拆成两层：

- server 进程的宿主路径由 `appfs.AppDir()` 计算为 `.nexus/app`；`NEXUS_STATE_ROOT`
  是状态根的唯一新配置，`NEXUS_CONFIG_DIR` 只作为旧版本状态根输入兼容。
- runtime 子进程使用的统一 `<user_root>`：每次按 `owner_user_id` 注入，不能继承
  server 的状态根或宿主目录。

宿主读取 transcript 时不能继续只依赖 server 进程全局的 `NEXUS_CONFIG_DIR`。Agent/session 必须携带或可推导自己的 `RuntimeConfigDir`，`AgentHistoryStore` 从该用户级 `<user_root>/projects` 读取。

第一阶段已完成以下调整：

- `nexus/internal/service/agent/workspace.go` / `ready.go`
- `nexus/internal/storage/workspace/transcript_path.go`
- `nexus/internal/runtime/clientopts/agent_client.go`
- `nexus/deploy/docker-compose.yml`
- `nexus/internal/infra/appfs/config_dir.go`
- `nexus/internal/migration/state_layout.go` / `workspace_layout.go`

旧版 `.nexus/config` 中已知的 Claude runtime 条目会先分流到
`users/__system__/runtime`，旧全局 `projects` 会依据数据库中的
`owner_user_id` 搬到对应用户 runtime；无法确定归属的 transcript 保留在
系统用户目录，不会猜测性地授予某个用户。显式配置在用户目录之外的
自定义 workspace 不带有可靠的 owner 路径语义，其历史 transcript 继续保留在
系统 runtime，避免迁移时误授予用户。旧状态根中不属于已知 Nexus 宿主
目录的未分类条目也保守迁入系统 runtime；Claude/nxs 新增用户文件时不会
因为宿主迁移清单滞后而误落到 `app/`。

启动顺序为：状态根文件迁移 → schema migration → workspace/Agent 路径与
transcript 迁移 → 既有 workspace 文件迁移。每一步都有独立完成标记；目标
冲突时不覆盖，`.claude.json` 的旧冲突副本会保留为
`runtime/.claude.json.legacy-config*`。桌面端或 Docker 在迁移前预创建的 runtime
目标文件也不阻塞启动：若源目标内容不同，迁移器先保留带
`.legacy-config*` 后缀的副本，再继续使用新 runtime 根。

## 7. Runtime 环境与配置

### 7.1 环境变量

runtime 环境必须由 allowlist 生成，不得继承 server 的完整环境。

允许传入的内容包括：

- 当前 provider/model 的必要配置；
- 当前 session 的短期 token；
- 当前 runtime 的 `HOME`、`PWD`、`TMPDIR`、config/cache 路径；
- 当前 workspace/project 的非敏感元数据；
- nxs/Claude 所需的协议和诊断开关。

禁止传入：

- app database URL 和数据库凭据；
- `CONNECTOR_CREDENTIALS_KEY`；
- 其他用户的 provider、connector、OAuth 或 session secret；
- server 内部监听、管理和部署凭据；
- 可关闭 mandatory policy、sandbox 或审计的安全开关。

### 7.2 配置和缓存

每个 runtime 使用自己的：

- `HOME`
- `NEXUS_CONFIG_DIR`
- `CLAUDE_CONFIG_DIR`
- `XDG_CONFIG_HOME`
- `XDG_CACHE_HOME`
- `TMPDIR`

`NEXUS_CONFIG_DIR` 与 `CLAUDE_CONFIG_DIR` 必须指向同一个用户级 `<user_root>`：

```text
NEXUS_CONFIG_DIR=<user_root>
CLAUDE_CONFIG_DIR=<user_root>
HOME=<user_root>/home
XDG_CONFIG_HOME=<user_root>/home/.config
XDG_CACHE_HOME=<user_root>/cache
TMPDIR=<user_root>/tmp
```

nxs 和 Claude 共用 `<user_root>/projects`。全局 Skill、二进制和只读模板使用 root-owned 只读目录；用户 Skill、npm/uv/pip cache 和临时文件写入 `<user_root>`。

server 的 host root 不得通过完整环境继承给 runtime。runtime 的 `NEXUS_CONFIG_DIR` 必须由 `owner_user_id -> UserScope -> user_root` 显式计算，不能由模型、Skill、project hook 或请求参数直接指定。

系统包安装不授予 runtime sudo。需要安装系统包时，由宿主执行固定 allowlist 的 broker；普通开发依赖优先使用用户级安装。

## 8. Hook 与最终访问校验

### 8.1 Mandatory PreToolUse policy

宿主为 nxs 和 Claude 注入同一份 `WorkspacePolicyHook`：

- 绑定 `owner_user_id`、Agent workspace root、当前 project roots 和 policy generation；
- 对 `Read/Write/Edit/Glob/Grep` 等路径工具执行路径归一化和 root containment；
- 对 Bash 默认拒绝，后续仅开放受控命令 broker；
- 阻断 `nexusctl` 的全局 scope、伪造 user scope 和管理入口；
- 不返回 `updatedInput`，只允许放行或拒绝；
- 超时、解析失败、策略缺失时拒绝；
- 对模型返回泛化原因，详细路径和身份写入内部审计事件。

### 8.2 不可信 hook

用户设置、project hook、Skill hook 和模型提示词不能修改或关闭 mandatory policy。

`NEXUS_SIMPLE`、`CLAUDE_CODE_SIMPLE`、`--bare` 或类似模式不能绕过最终访问校验。即使 runtime hook 被跳过，SDK/tool handler 前仍必须执行 final path guard。

### 8.3 Final path guard

最终 guard 在工具 handler 或系统调用前重新检查：

- 最终生效的输入，而不是 hook 之前的输入；
- 符号链接和重命名竞争；
- 相对路径、绝对路径和 Shell 展开结果；
- 读、写、创建、删除、执行的不同权限；
- 当前 OS identity 是否仍与 session policy 匹配。

PostToolUse 只负责审计和告警，不能作为阻止读取的手段。SessionStart/Setup/ConfigChange 用于校验 identity、workspace 和 policy 指纹。

## 9. 控制面授权

OS 权限不能替代业务授权。以下入口必须继续按真实认证用户过滤：

- Agent、workspace、Room、DM 和 transcript API；
- `nexusctl` 的 user/global scope；
- storage repository 的 `owner_user_id` 条件；
- workspace 列表、搜索、导出和恢复；
- MCP、connector、automation 和 provider 凭据读取。

任何从控制面返回其他用户 workspace、Agent 活动或凭据的路径，都必须在服务端修复；hook 只能作为额外阻断和审计。

## 10. 协作语义

### 10.1 默认

- 用户只能读写自己的 workspace。
- 同一用户的 Agent 共享该用户被授予的 private group。
- Agent 不自动获得其他用户的 workspace。

### 10.2 显式共享项目

- Nexus 的项目成员关系映射为项目 group/ACL。
- 加入项目后，成员获得项目声明的 read-only 或 read-write 权限。
- 移除成员时，停止或重启受影响 runtime，撤销后续 session 的项目组；不依赖已有进程自行刷新。
- 项目组不授予 app data、全局 transcript 或 connector key 权限。

### 10.3 运行时组范围

runtime 只携带当前 session 所需的 private group 和明确授权的 project groups。不能把所有系统组、所有项目组或 server 的附加组原样继承给子进程。

## 11. 平台与部署矩阵

| 部署形态 | 本规范状态 | 说明 |
| --- | --- | --- |
| 原生 Linux 服务端 | 首选 | POSIX UID/GID、setgid、ACL 和 launcher 可控 |
| Linux Docker + state volume | 支持目标 | 可继续使用单一 `.nexus` volume，但 `app/` 与 `users/` 必须是不同权限子树 |
| Linux Docker + 宿主 bind mount | 条件支持 | 必须验证宿主 UID/GID、ACL、备份和恢复语义 |
| Docker Desktop macOS/Windows bind mount | 暂不承诺 | 文件共享层的 UID/GID 语义和性能需单独验证 |
| Nexus macOS/Windows 桌面端 | 保持单用户 | 不为本地用户创建额外系统账号 |
| 每用户独立容器/VM | 更高隔离档 | 作为 hostile-tenant 部署选项，不是第一阶段默认方案 |

## 12. 迁移与发布阶段

### Phase 0：收口现有旁路

- 服务端和 runtime 环境改为 allowlist；
- 修复 app API 和 `nexusctl` 的 owner 授权；
- 注入 deny-only PreToolUse hook；
- 禁止安全关键环境变量覆盖 policy；
- 记录越界尝试，不改变现有数据布局。

### Phase 1：Linux per-user identity

- 建立 `owner_user_id -> RuntimeIdentity` 映射；
- 实现受控 launcher/worker；
- 新建 workspace 和用户 runtime root 使用新的 UID/GID、setgid 和 default ACL；
- 为 nxs/Claude 注入同一个用户级 `<user_root>`，共用固定的 `projects` 子目录；
- nxs/Claude 都通过同一身份启动；
- App 和 Web 都通过同一个 `UserScope` 进入 launcher；
- 通过 feature flag 仅对 Linux server 开启。

### Phase 2：存量迁移

- 停止受影响 runtime；
- 为每个用户创建 identity 和 `users/<owner_user_id>`；
- 按 owner 迁移 workspace、Skill、nxs/Claude session、`HOME` 和缓存；
- 使用 `stat`、ACL 检查和 checksum 验证迁移结果；
- 迁移失败时保留原目录，不覆盖原数据；
- 完成双用户负向访问测试后再切换默认根。

### Phase 3：项目协作与纵深隔离

- 引入项目 group/ACL 和成员变更收口；
- 引入 final path guard；
- Linux 环境启用 Landlock/bwrap/cgroup 等额外限制；
- 根据部署需求评估 per-user worker container。

## 13. 验收标准

### 13.1 负向测试

对用户 A、B 各创建 sentinel workspace，分别用 nxs 和 Claude 验证：

- `Read/Glob/Grep/Write/Edit` 访问 B 路径均失败；
- 相对路径、绝对路径、`..`、符号链接和硬链接均不能越界；
- Bash、Shell 展开、`find`、`cat`、`cp` 和 `nexusctl` scope 不能越界；
- simple/bare 模式不能绕过 final guard；
- runtime 环境中不存在 app DB、connector key 和 B 的 provider secret；
- `/proc`、共享 `/tmp`、共享 cache 不泄漏其他用户敏感数据。

### 13.2 正向测试

- A 可以无确认读写自己的 workspace；
- 同一用户的多个 Agent 保持现有协作体验；
- 明确加入项目的成员可以按项目权限读写；
- 只读成员不能写；
- 移除成员后新 session 立即失效；
- 重启、备份恢复和 runtime resume 保持 UID/GID 映射稳定。

### 13.3 运维测试

- workspace 创建、删除、恢复和迁移不会产生 world-readable 文件；
- launcher 不能被 runtime 用户直接调用或注入任意参数；
- server 重启不会留下可被其他用户继承的旧 UID/GID；
- Docker volume、bind mount 和备份工具保留预期权限；
- nxs 与 Claude 的启动诊断都能记录实际 UID、GID、workspace root 和 policy generation，但不记录 secret。

## 14. 待讨论决策

1. 第一阶段是否只承诺原生 Linux / Linux volume？
2. 采用 root-owned launcher，还是拆成独立 runtime worker？
3. 项目协作以 POSIX group 为主，还是以 default ACL 为主？
4. 同一用户的不同 Agent 是否默认共享全部用户项目，还是只授予当前 session 的项目组？
5. `app/` 与 `users/` 是否保持在同一个 `.nexus` volume 下；当前倾向共用 volume，但保持不同权限子树。
6. 系统包安装是否统一改为宿主 broker，取消 runtime 侧 sudo？

## 15. 参考

- [Linux inode(7)：目录 setgid 与继承的 group ownership](https://www.man7.org/linux/man-pages/man7/inode.7.html)
- [Linux acl(5)：access ACL 与 default ACL](https://man7.org/linux/man-pages/man5/acl.5.html)
- [Linux Landlock：非特权进程的叠加式文件系统限制](https://www.kernel.org/doc/html/latest/userspace-api/landlock.html)
- [Docker bind mounts：挂载默认可写及其宿主文件系统影响](https://docs.docker.com/engine/storage/bind-mounts/)
