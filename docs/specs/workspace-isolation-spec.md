# Workspace 隔离与多用户运行时规范

## 1. 文档状态

- 状态：已实现目录布局、迁移、Linux UID/GID、项目 ACL 控制面、nxs/Claude Hook、项目成员管理 UI、Landlock launcher、主要宿主文件 broker 的 confined-fd 边界、opt-in per-user cgroup 回收。
- 默认模式为 `off`；原生 Linux 上需显式启用 `enforce` 并完成部署验收。
- 发布前必须在目标 Linux 内核、文件系统和容器 seccomp 配置上验收（本地 macOS 无法执行 setuid、POSIX ACL 和 Landlock）。
- `enforce` 配置不得关闭 Landlock。
- 核对日期：2026-08-11；适用范围：Linux 服务端多用户部署。
- 结论：OS UID/GID 是主边界；项目组/ACL 负责显式协作；runtime hook 和最终路径校验负责策略收口；`.nexus` 是统一状态根，`app/` 保存宿主数据，runtime 配置和会话按用户独立存放。

## 2. 目标与非目标

### 2.1 目标

1. 用户 A 的 runtime 不能通过文件工具、Shell、nxs、Claude 或普通进程直接读取或修改用户 B 的 workspace。
2. 同一用户的多个 Agent 继续共享该用户被授予的资源。
3. 跨用户协作只能通过显式项目成员关系授予，不依赖路径猜测或模型自律。
4. 在允许的 workspace 内保持无额外确认的正常开发体验。
5. nxs 和 Claude 使用同一份用户身份、配置根与 workspace policy。
6. server、runtime、控制面数据和用户数据拥有清晰的权限边界。
7. App 与 Web 使用同一套用户/租户模型；App 只是自动登录的单用户部署。

### 2.2 非目标

- 防御宿主 root、容器运行时、内核或文件系统本身被攻破。
- 用文件权限替代 HTTP、WebSocket、`nexusctl` / `nexuscfg` 和 storage 层的 owner 授权。
- 每个 Agent 一个操作系统用户。
- 网络、CPU、内存、PID namespace 或设备 ioctl 隔离。Landlock 只约束 runtime 进程的文件系统访问；cgroup 仅在显式配置时负责 owner 进程树回收。
- 在 macOS/Windows 桌面端强制创建本地系统用户。
- 独立的组织/成员层级；当前租户边界是 `owner_user_id`。

## 3. 威胁模型

纳入：

- 能控制 Agent 提示词、Skill、project hook 或 Bash 输入的模型/运行时进程。
- 用户 A 试图读取用户 B 的 workspace、transcript、配置或缓存。
- 误配置的 runtime 通过绝对/相对路径、符号链接、Shell 展开或控制面 CLI scope 访问越界资源。

不纳入：

- 已获得 `nexus-host` 或宿主 root 权限的攻击者。
- 利用 Docker、Linux 内核、文件系统驱动或硬件漏洞逃逸的攻击者。
- 仅通过业务 API 伪造 owner 身份的攻击者；必须由控制面授权单独阻断（§9）。

在“宿主和内核可信、runtime 不可信”的前提下，独立 UID/GID 是跨用户文件隔离的主边界。Hook 只是防御纵深和审计入口，不能代替 DAC、ACL 或最终系统调用边界。

## 4. 核心原则

1. **身份先于路径**：workspace 路径只是定位信息，最终权限由 OS 身份和文件系统权限决定。
2. **默认拒绝**：没有明确授予的用户、组、路径和操作均拒绝。
3. **宿主托管**：安全策略、运行身份、环境变量和启动参数由宿主生成；用户配置、Skill 和 project hook 不能降低安全边界。
4. **宿主根与用户根分区**：`app/` 只保存宿主控制面和宿主共享资源；owner 的全部数据只进入 `users/<owner_user_id>/`。
5. **协作显式化**：共享目录必须有项目成员关系；加入项目组意味着成员之间互相信任并可按项目权限操作。
6. **身份稳定**：用户到 UID/GID 的映射必须持久化，不能因重启、恢复或用户删除而意外复用。
7. **平台诚实**：只在具备可靠 POSIX 权限语义的部署上承诺该隔离等级。
8. **端无关**：桌面 App 的本地免登录只是认证适配器，不能形成第二套 owner、workspace 或 runtime 规则。

## 5. 身份模型

### 5.1 产品用户与运行身份

每个 `owner_user_id` 对应一个产品用户、一个用户数据根和一个 runtime OS identity；同一用户的多个 Agent 复用该身份。当前不提供 Agent 级 UID 隔离。

```text
UserScope                 # 逻辑派生链，不是单一 Go 类型
  principal               # authctx.Principal：认证适配器写入请求上下文；auth_method = local | password 等，不参与数据归属判定
  owner_user_id           # authctx.OwnerUserID：principal.user_id，缺失时为 SystemUserID
  user_root               # appfs.UserDataRoot(owner_user_id)
  workspace_root          # appfs.UserWorkspaceRoot(owner_user_id)
  runtime_root            # appfs.UserRuntimeRoot(owner_user_id)

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

- `uid`、`private_gid` 和目录路径由宿主生成，不直接使用用户输入。
- 账号无密码、不可交互登录、无 sudo，默认 shell 为 `nologin`。
- 映射记录持久化在 `app/data`。
- 删除用户后，只有在其文件完成清理或迁移后才允许回收 UID/GID。
- `supplementary_gids` 默认为空，仅加入当前 session 明确需要的项目组（§10.3）。
- `owner_user_id` 是控制面查询、workspace、Agent、Room、凭据、Skill、automation 和 runtime 目录的统一归属键。

### 5.2 身份启动

- Linux 多用户强隔离由 root-owned setuid `nexus-runtime-launcher` 执行；server 保持 `nexus-host` 普通用户，不以 root 运行，也不直接把任意 UID/GID 交给 runtime。
- 禁止为省去 launcher 让整个 server 以 root 运行；禁止挂载 Docker socket 让 runtime 自己创建容器。

launcher 至少校验：

- 调用方是受信任的 `nexus-host`；
- runtime executable 位于固定 allowlist；
- `CWD` 位于该 identity 被授予的 workspace/project root；
- UID、主 GID、附加 GID 来自已持久化映射；
- 环境变量来自宿主 allowlist；
- 调用方不能注入任意 `argv`、`LD_PRELOAD`、动态 loader 或额外文件描述符；
- 启动后立即丢弃不必要的 capability。

启动期的跨用户、跨项目和隔离根外硬链接校验，由已提升完整 root 身份的 launcher 在修改 ACL 前完成；host app UID 不直接遍历 runtime 创建的 `0700` 私有会话目录。

### 5.3 App 与 Web 统一租户模型

- 两端都通过同一个 `UserScope` 进入业务层，不设两套租户模型。
- Web：登录 Session、Bearer Token 等认证适配器解析出 `principal`，再得到 `owner_user_id`。
- App：本地免登录适配器自动绑定现有 `SystemUserID` 对应的本地用户；它不是“无用户”或“全局 system scope”。
- Handler、service、repository、runtime launcher、Hook 和 transcript store 只消费 `UserScope`/`owner_user_id`；数据归属不得按 `auth_method`（App/Web 端差异）分叉，禁止“桌面走 system scope、Web 走 owner scope”的双轨逻辑。
- 两端差异仅限认证方式、进程部署和 UI；Agent、workspace、Room、DM、provider、connector、Skill、automation、quota 和审计使用同一套归属规则。
- Web 后续增加组织/成员关系时，在 `owner_user_id` 之上增加 `tenant_id`；不引入 App 专属的第二套 owner 语义。
- 未认证部署不等于全局管理员：没有认证主体的 HTTP/Web 请求只绑定 `SystemUserID`；只有显式标记的内部维护上下文才允许无 owner 查询。缺少 principal 不得回退为枚举所有用户。

## 6. 文件系统布局与权限

### 6.1 `.nexus` 状态根

`nexus_state_root` 固定为 `.nexus`，内部不再嵌套 `.nexus`。宿主控制面数据放在 `app/`，用户数据放在 `users/<owner_user_id>/`。新增宿主或 runtime 文件必须直接落在对应的 `app/` 或用户根目录。

```text
.nexus/                               # nexus_state_root
  app/                                # 宿主根，nexus-host 私有
    data/                             # app DB、迁移状态
    config/                           # Nexus 配置、密钥
    logs/                             # server 日志
    cache/                            # 宿主共享 cache
    shared/                           # root-owned 只读 Skill、二进制、模板

  users/
    <owner_user_id>/                  # 用户数据根，private group
      workspace/
        .rooms/                       # owner 级公共附件，runtime 可读写但不承载控制状态
        <agent_id>/                   # Agent 工作目录
      runtime/                        # <user_root>；= NEXUS_CONFIG_DIR = CLAUDE_CONFIG_DIR
        projects/                     # nxs/Claude transcript store
        .claude.json                  # Claude 全局配置
        settings.json                 # Claude 用户级 settings
        .claude/                      # Claude 用户级扩展与兼容文件
        home/                         # HOME
        cache/
        logs/
        tmp/
      state/                          # 宿主持久化的 owner 状态，不属于 runtime config 根
        rooms/                        # Room ledger：overlay、消息游标、handoff、延迟唤醒

  shared-workspaces/
    <shared_workspace_id>/            # 项目 group/ACL 共享目录
```

- 桌面默认 `~/.nexus`；Docker 可挂载到 `/home/agent/.nexus`；服务端可映射到 `/var/lib/nexus`。
- `app/` 与 `users/` 必须是不同权限子树，但始终属于同一个状态根。
- `NEXUS_CONFIG_DIR` / `CLAUDE_CONFIG_DIR` 产生的 runtime 用户文件不得写入 `app/`。

桌面数据目录迁移：

- 只迁移完整 `NEXUS_STATE_ROOT`；不支持拆分 `app/` 与 `users/`，业务进程不支持在线迁移局部子树。
- 原生宿主在确认后退出 sidecar，离线复制状态根，切换宿主外的启动指针并直接重启。
- 启动提交阶段必须先完成数据库、transcript 与 Room 结构化绝对路径重映射，以及路径派生的 Session 删除恢复文件名重映射，再通过健康检查提交新根，之后才清理旧根；启动失败回滚指针。
- Linux 服务端和 `enforce` 部署的状态根由部署配置与权限模型管理，不提供应用内迁移。

权限约束：

- `.nexus` 与 `users/` 只给 runtime UID 目录穿越位；`nexus-host` 通过宿主组创建用户目录；runtime 不能列举根目录或全部用户。
- `.nexus/app` 及其 `data/config/logs` 只允许 `nexus-host` 访问；runtime 不能继承 `app/` 或其他 owner 根。
- 当前 owner 的 runtime 对整棵 `users/<owner_user_id>` 读写，跨 owner 访问拒绝。
- `users/<owner_user_id>` 边界目录由 root 与对应 private group 控制；边界内 `workspace/runtime/state` 使用同一 private group。
- owner 的 `workspace/` 根由 `nexus-host` 持有并启用 setgid+sticky；每个 `workspace/<agent_id>` 边界保持 root-owned、private group 可写。runtime 可改其内容，但不能把宿主随后会打开的根目录替换成 symlink。
- 边界内文件与 runtime 内容归对应 runtime UID，宿主通过 named-user ACL 访问。
- workspace 和 shared workspace 使用 setgid；default ACL 只授予宿主、当前运行组和明确的项目组。
- 普通文件默认不允许 `other` 读写；runtime 使用 `umask 0007`。
- shared workspace 成员关系由 Nexus 控制；Agent 不能自行改组、改 ACL 或扩大 scope。
- 宿主打开 `state/rooms` 时必须从 `NEXUS_STATE_ROOT` 的目录 fd 逐级进入，每段拒绝 symlink 并核对 inode；ledger 文件同时拒绝 symlink、多硬链接与校验后替换，不能只依赖字符串前缀或一次 `EvalSymlinks`。
- 宿主读取 Agent workspace 与 `workspace/.rooms` 附件时同样从 owner 管理根逐级固定目录 fd；图片内容直接从校验后打开的 fd 读取，不能先返回绝对路径再重新打开。

### 6.2 nxs 与 Claude 的用户级配置根

- `projects` 是 runtime transcript store，不是用户协作项目目录。
- `<user_root>` 是当前用户专属 runtime 根，nxs 与 Claude 都直接使用它并共用 `<user_root>/projects`（环境变量见 §7.2）。Claude 不能单独配置 `projects` 子目录，只能配置整个 `CLAUDE_CONFIG_DIR`。
- `NEXUS_CONFIG_DIR` 分两层语义：
  - server 进程：宿主路径由 `appfs.AppDir()` 计算为 `.nexus/app`；`NEXUS_STATE_ROOT` 是状态根的唯一新配置，`NEXUS_CONFIG_DIR` 只作为旧版本状态根输入兼容。
  - runtime 子进程：每次按 `owner_user_id` 注入 `<user_root>`，不能继承 server 的状态根或宿主目录。
- bridge 可以保持 `CLAUDE_CONFIG_DIR` 与 `NEXUS_CONFIG_DIR` 同步，但同步源必须是宿主按 `owner_user_id` 计算的 `<user_root>`。
- 宿主读取 transcript 不能只依赖 server 全局 `NEXUS_CONFIG_DIR`；Agent/session 必须携带或可推导自己的 `RuntimeConfigDir`，`AgentHistoryStore` 从该 `<user_root>/projects` 读取。

关键实现入口：

- `nexus/internal/service/agent/workspace.go` / `ready.go`
- `nexus/internal/storage/workspace/transcript_path.go`
- `nexus/internal/runtime/clientopts/agent_client.go`
- `nexus/deploy/docker-compose.yml`
- `nexus/internal/infra/appfs/config_dir.go`

### 6.3 历史布局迁移

- 运行时只读写 canonical `.nexus/app`、`.nexus/users/<owner>` 与 `.nexus/shared-workspaces`。
- 历史数据只能经 `internal/migration/state_layout.go`、`workspace_layout.go` 这类版本化、可重试、可审计的启动迁移进入 canonical 布局；不提供旧路径运行时回读。
- 迁移不能因版本发布而提前移除，必须允许跨多个版本直接升级。
- 迁移只执行 rename、不覆盖式合并和 owner 数据库映射。
- 运行中的 server 不猜测历史目录归属，也不把 `app/` 当作用户 runtime 的兼容回退。

v0.1.27/v0.1.28 直接升级到 v0.1.30 的修复迁移（v0.1.30 曾遗漏该入口）：

- 若受影响版本已误建 `app/data/nexus.db` 与 `users/` 数据，先把两个新分支隔离到 `app/.migration-quarantine/skipped-state-layout-v1/`。
- 恢复旧库、Agent workspace、transcript 与 Room 源文件。
- 以旧数据优先、外键完整的单事务补入新库非冲突记录；不冲突的 owner 文件并回 canonical `users/`；文件冲突保留在隔离区。

旧版共享 `app/rooms` 的 Room 文件迁移（有完成标记）：

- 先从数据库按 `conversation_id` 确认 `owner_user_id`，再迁入 `users/<owner>/state/rooms`；不能从目录名或文件内声明猜测 owner。
- `overlay.jsonl`、directed message、消费游标、public handoff 与 delayed wake 按 owner 拆分；JSONL 按规范化内容去重，支持中断后重试。
- 旧 `attachments/` 迁入 `users/<owner>/workspace/.rooms/<conversation>/` 的对应相对路径。
- 无法确认 owner、混合多个 conversation、含符号链接/硬链接/特殊文件的状态移入宿主私有 `app/.migration-quarantine/room-state-v1`。
- 迁移告警不阻断启动；完成后移除旧目录并在 `app/.migrations` 写入标记，不再回读 `app/rooms`。

## 7. Runtime 环境与配置

### 7.1 环境变量

runtime 环境必须由 allowlist 生成，不得继承 server 的完整环境。

允许：

- 当前 provider/model 的必要配置；
- 当前 session 的短期 token；
- 当前 runtime 的 `HOME`、`PWD`、`TMPDIR`、config/cache 路径；
- 当前 workspace/project 的非敏感元数据；
- nxs/Claude 所需的协议和诊断开关。

禁止：

- app database URL 和数据库凭据；
- `CONNECTOR_CREDENTIALS_KEY`；
- 其他用户的 provider、connector、OAuth 或 session secret；
- server 内部监听、管理和部署凭据；
- 可关闭 mandatory policy、sandbox 或审计的安全开关。

### 7.2 配置、缓存与记忆

每个 runtime 使用自己的配置与缓存根：

```text
NEXUS_CONFIG_DIR=<user_root>
CLAUDE_CONFIG_DIR=<user_root>
HOME=<user_root>/home
XDG_CONFIG_HOME=<user_root>/home/.config
XDG_CACHE_HOME=<user_root>/cache
TMPDIR=<user_root>/tmp
```

- `<user_root>` 必须由 `owner_user_id -> UserScope -> user_root` 显式计算，不能由模型、Skill、project hook 或请求参数指定。
- 全局 Skill、二进制和只读模板使用 root-owned 只读目录；用户 Skill、npm/uv/pip cache 和私有临时文件写入 `<user_root>`。
- Unix runtime 额外获得 `/tmp` 共享兼容读写根，以保持 App 与 Web 命令行为一致。`/tmp` 依赖 sticky bit 与每用户 UID/GID 防止删除或覆盖他人文件，但文件名和宽松权限的内容可能被其他 runtime 看见；凭据、provider 响应等敏感数据必须写入 `$TMPDIR`，不得写入 `/tmp`。
- 系统包安装不授予 runtime sudo，runtime 也不能通过 launcher 获得额外 sudo。需要系统包时由宿主执行固定 allowlist 的 broker；普通开发依赖优先用户级安装。

Nexus 管理的 nxs 长期记忆根固定为当前 Agent workspace：

```text
NEXUS_MEMORY_DIR=<agent_workspace>
<agent_workspace>/MEMORY.md
<agent_workspace>/memory/
```

- 宿主继承环境与请求级 `ExtraEnv` 都不能改写该值。
- 受管 runtime 中 `NEXUS_ENABLE_REMOTE_MEMORY` 与 `NEXUS_REMOTE_MEMORY_DIR` 固定关闭。
- SDK 读写、Web 只读投影与 workspace policy 因此引用同一个 owner/Agent 路径；SDK 独立运行时自己的记忆根不受此约束。
- 会话摘要仍独立位于 owner 的 `runtime/projects/`。

## 8. Hook 与最终访问校验

### 8.1 Mandatory PreToolUse policy

宿主为 nxs 和 Claude 注入同一份 `WorkspacePolicyHook`：

- 绑定 `owner_user_id`、owner 数据根、当前 project roots 和 policy generation。
- 对 `Read/Write/Edit/Glob/Grep` 等路径工具执行路径归一化和 root containment。
- 当前 owner 的整棵 `users/<owner_user_id>` 统一允许读写，不为 transcript、session-memory 或 `state` 维护单文件例外；跨 owner 路径拒绝。
- 对 Bash 只做显式绝对路径、`..` 路径和 `nexusctl` / `nexuscfg` 管理入口的早期检查；普通系统命令可运行，最终写入/删除/重命名由 OS DAC/ACL 与 Landlock 决定。
- 控制面命令：
  - enforce Hook 早期拒绝普通 Agent 的 `nexusctl` 管理命令；打包部署额外把 `nexusctl` executable 设为宿主组专用。
  - Nexus 主智能体是宿主控制面主体，保留 host identity，可使用 `NEXUSCTL_COMMAND_PATH` 的当前 owner scope。
  - `nexuscfg` 对所有交互 Agent 可执行，但只把命令转发给宿主 loopback broker。宿主签发的 round capability 固定 owner、Agent、DM/Room 和 runtime lease；configuration 角色矩阵决定最终 operation。
  - CLI 作用域或 capability 覆盖返回可重试错误；Hook 对这类 shell 文本仍做早期拒绝。
  - Goal、Execution 与 Automation 使用进程内 round-scoped `nexus.command`。后台 run 固定 job/run 且只读；交互 mutation 还需 service plan/revision/digest 与当前会话真人确认。
  - 结构化 input 经 SDK MCP 通道直达宿主，不创建临时文件、不进入 shell、不扩大文件系统权限。
- 不返回 `updatedInput`，只放行或拒绝。
- Hook 不返回 `allow` 决策，避免覆盖其他 hook 或用户权限处理；越界时返回 `deny`。
- Hook 失效不构成安全放行；enforce 进程仍必须通过 launcher 的最终边界。
- 对模型返回泛化原因；详细路径和身份写入内部审计事件。

### 8.2 不可信 hook

- 宿主把 mandatory policy 放在初始化时已知 Hook 的最后，使它检查前序 Hook 更新后的输入并保留最终否决权。
- 用户设置、project hook 和模型提示词不能从宿主 options 中移除它。
- nxs 运行期动态注册的 Skill hook 仍可能改变合并顺序，因此 Hook 的 `deny` 不是可信的最终安全边界。
- `NEXUS_SIMPLE`、`CLAUDE_CODE_SIMPLE`、`--bare` 或类似模式不能绕过最终访问校验；launcher 拒绝禁用 hook 的 argv/环境。即使 hook 被跳过，Landlock 仍作用于整个 nxs/Claude 进程及其子进程。
- `nexusctl`、`nexuscfg` 与 SDK 内的 `nexus.command` 属于控制面而非用户文件系统，最终边界不能只依赖 Hook 文本识别：
  - `nexusctl`：生产部署必须通过 DAC/容器镜像边界让普通 runtime UID 无法执行。
  - `nexuscfg`：宿主 round capability 与配置服务授权。
  - 结构化 command：进程内 round actor 与对应领域服务授权。
- 主智能体保留宿主身份是明确的控制面信任边界，不具备普通 Agent 的 Landlock 隔离等级。

### 8.3 Final path guard

root-owned launcher 在 runtime `exec` 前安装 Landlock ABI 3+ ruleset，由内核在每个受控文件系统系统调用上重新检查：

- 最终生效的输入，而不是 hook 之前的输入；
- 符号链接和重命名竞争；
- 相对路径、绝对路径和 Shell 展开结果；
- 读、写、创建、删除、执行的不同权限；
- 当前 OS identity 是否仍与 session policy 匹配；
- 允许的 workspace、用户 runtime、显式只读资源和项目 root；
- `make/rename/link/remove/truncate` 等创建与变更操作。

当前没有独立 PID namespace；`/proc` 元数据遵循宿主内核常规 DAC/ptrace 语义，不得宣传为进程表隐藏。

宿主 broker（不在 Landlock domain 内）：

- Landlock 只约束 launcher `exec` 的 runtime 及其子进程；SDK 进程内 MCP、HTTP workspace API 和其他宿主 broker 的文件调用发生在 `nexus-host` 进程中。
- 宿主代 runtime 操作 workspace、transcript、artifact、用户 Skill 或 Room 状态时必须使用 `internal/infra/confinedfs`：先校验 owner，再持有目录 fd 访问；owner 校验后不得把用户可控绝对路径直接交给 `os.*`。
- 已覆盖：workspace 读写/下载、附件与图片、automation artifact、偏好、runtime settings、transcript/JSONL、用户 Skill registry。下载直接消费已打开文件，不在校验后按绝对路径重开。
- Room ledger、owner-aware InputQueue 与 Room transcript 引用先按 owner 计算 managed root，再从该根的目录 fd 逐段打开 workspace/session 路径；持久化绝对路径只用于定位，不能升级为可信根。
- 普通文件打开时核对打开前后 inode，并在 Darwin/Linux 上拒绝多硬链接文件。

信任根约束：

- `os.Root` 只在根目录 fd 成功打开后提供 containment，可被 runtime 替换的路径不能充当信任根。launcher 把 host-owned 的 owner workspace 顶层设为 sticky、Agent workspace 边界设为 root-owned；迁移遇到 workspace 顶层 symlink 会 fail closed。
- `os.Root` 不隔离 bind mount、设备文件或文件系统边界，因此宿主读接口同时拒绝符号链接和非普通文件。
- 部署不得给 runtime `CAP_SYS_ADMIN`、`CAP_MKNOD` 或可写宿主 mount namespace。
- 平台 Skill 构建、首次 owner/workspace 根创建和迁移器操作宿主推导出的固定路径，不接受 runtime 提供的目标路径，并由 root-owned 父目录、迁移锁和 launcher ACL 保护。

## 9. 控制面授权

OS 权限不能替代业务授权。以下入口必须按真实认证用户过滤：

- Agent、workspace、Room、DM 和 transcript API；
- `nexusctl` / `nexuscfg` 的 user/global scope；
- storage repository 的 `owner_user_id` 条件；
- workspace 列表、搜索、导出和恢复；
- MCP、connector、automation 和 provider 凭据读取。

任何从控制面返回其他用户 workspace、Agent 活动或凭据的路径都必须在服务端修复；hook 只作额外阻断和审计。

认证部署不得接受 `local_path` 让 HTTP 用户要求宿主读取任意本地目录；外部 Skill 必须通过受限归档上传或由宿主内部下载到私有 staging。本地单用户部署保留 `local_path` 兼容能力。

## 10. 协作语义

### 10.1 默认

- 用户只能读写自己的 workspace。
- 同一用户的 Agent 共享该用户的 private group。
- Agent 不自动获得其他用户的 workspace。

### 10.2 显式共享项目

- 项目协作采用“项目 GID + named-user ACL”混合模型：write 成员加入项目组，read 成员只写 named ACL。
- launcher 提供 `project-ensure` / `project-grant` / `project-list`；HTTP 控制面按角色和项目成员过滤。
- `GET /projects` 对普通成员只返回已加入项目并隐藏其他成员标识；owner/admin 可查看完整项目 registry。
- `POST /projects` 由 launcher 在同一 registry 锁内给创建者授予 write；对既有项目执行 ensure 不会自动加入调用者。
- `PUT /projects/{project_id}/members/{owner_user_id}` 仅允许 admin 变更 `read` / `write` / `none`。运营设置提供对应 UI；前端只按角色调整交互，最终授权由服务端 owner/admin 规则判定。
- 成员关系变化后，Manager 按 `owner_user_id` 取消 round 并回收全部热 runtime，撤销后续 session 的项目组，不依赖已有进程自行刷新。
- session key 绑定 owner，拒绝跨 owner 复用。
- 项目组不授予 app data、全局 transcript 或 connector key 权限。

### 10.3 运行时组范围

普通 Agent runtime 只携带自己的 private group 和当前启动票据明确授权的 project groups；不得继承所有系统组、所有项目组或 server 的附加组。

## 11. 平台与部署矩阵

| 部署形态 | 状态 | 说明 |
| --- | --- | --- |
| 原生 Linux 服务端 | 首选，承诺 `enforce` | POSIX UID/GID、setgid、ACL 和 launcher 可控 |
| Linux Docker + state volume | 条件支持 | `app/` 与 `users/` 共用 `.nexus` volume，但必须是不同权限子树，由 launcher 收紧宿主 app 子树 |
| Linux Docker + 宿主 bind mount | 条件支持 | 必须验证宿主 UID/GID、ACL、备份和恢复语义 |
| Docker Desktop macOS/Windows bind mount | 暂不承诺 | 保持 `off`/`audit`；文件共享层 UID/GID 语义和性能需单独验证 |
| Nexus macOS/Windows 桌面端 | 单用户 | 不为本地用户创建额外系统账号 |
| 每用户独立容器/VM | 当前合同外 | 可由部署方作为额外 hostile-tenant 隔离层 |

## 12. 模式与进程回收

### 12.1 隔离模式

- `NEXUS_RUNTIME_ISOLATION_MODE` 是 runtime isolation 的唯一选择，认证状态不覆盖它：
  - `enforce`：仅在 Linux server 上可用；
  - `audit`：只启用 Hook 和日志；
  - `off`：保持兼容行为。
- 不论模式，服务端与 runtime 环境均为 allowlist，注入 deny-only PreToolUse hook，禁止安全关键环境变量覆盖 policy，并记录越界尝试。

### 12.2 存量迁移流程

启动迁移已实现，目标部署仍需验收：

- 停止受影响 runtime；
- 为每个用户创建 identity 和 `users/<owner_user_id>`；
- 按 owner 迁移 workspace、Skill、nxs/Claude session、`HOME` 和缓存；
- 用 `stat`、ACL 检查和 checksum 验证迁移结果；
- 失败时保留原目录，不覆盖原数据；
- 完成双用户负向访问测试后再切换默认根。

### 12.3 会话与 owner 进程树回收

- bridge 的中断、关闭和遗留子进程清理统一调用 root-owned launcher；launcher 先固定 pidfd，再校验目标属于当前 owner UID/cgroup 和对应 Unix session，避免跨 UID 信号失败或 PID 复用误杀。
- 未启用 cgroup 时仍可可靠回收同 session 进程；主动 `setsid`/double-fork 脱离 session 的 orphan 不在该保证内。
- launcher 支持 cgroup v2 per-user 子 cgroup，并在 runtime `exec` 前写入 `cgroup.procs`；`stop-user` 用 `cgroup.kill` 回收 double-fork 的 orphan descendant。
- Manager 在项目权限撤销、owner 级关闭和最后一个热 session 关闭时触发回收，回收期间阻止新的同 owner session 插入。
- 默认不创建 cgroup；`cgroup_root` 指向 root-owned cgroup v2 子目录并设置 `cgroup_required=true` 后，能力缺失会 fail closed。cgroup 需在目标 Linux 部署显式配置并现场验收。
- worker container、PID namespace 与部署级 seccomp profile 不属于当前合同。

## 13. 验收标准

### 13.1 负向测试

对用户 A、B 各创建 sentinel workspace，分别用 nxs 和 Claude 验证：

- `Read/Glob/Grep/Write/Edit` 访问 B 路径均失败；
- 相对路径、绝对路径、`..`、符号链接和硬链接均不能越界；
- Bash、Shell 展开、`find`、`cat` 和 `cp` 不能越过文件系统边界；
- 普通 Agent 的 `nexusctl` 管理命令被 Hook 拒绝，生产部署同时由 DAC 阻止 runtime UID 执行；普通 Agent 的 `nexuscfg` / `nexus` 只能使用当前 round capability，无法覆盖身份、作用域、job/run 或越权修改其他 Agent；
- simple/bare 模式不能绕过 final guard；
- runtime 环境中不存在 app DB、connector key 和 B 的 provider secret；
- `/proc` 不应暴露其他用户受 DAC 保护的环境/文件；共享 cache 不作为用户 runtime 写入根；共享 `/tmp` 中 B 的文件不能被 A 删除或覆盖。

### 13.2 正向测试

- A 可以无确认读写自己的 workspace；
- 同一用户的多个 Agent 保持现有协作体验；
- 明确加入项目的成员可以按项目权限读写；只读成员不能写；
- 移除成员后新 session 立即失效；
- 重启、备份恢复和 runtime resume 保持 UID/GID 映射稳定。

### 13.3 运维测试

- workspace 创建、删除、恢复和迁移不产生 world-readable 文件；
- launcher 不能被 runtime 用户直接调用或注入任意参数；
- server 重启不留下可被其他用户继承的旧 UID/GID；
- Docker volume、bind mount 和备份工具保留预期权限；
- nxs 与 Claude 的启动诊断记录实际 UID、GID、workspace root 和 policy generation，但不记录 secret；
- cgroup v2 启用时，关闭 owner 或撤销项目成员关系后，父进程及其 double-fork 子进程均从目标 cgroup 消失。

## 14. 参考

- [Linux inode(7)：目录 setgid 与继承的 group ownership](https://www.man7.org/linux/man-pages/man7/inode.7.html)
- [Linux acl(5)：access ACL 与 default ACL](https://man7.org/linux/man-pages/man5/acl.5.html)
- [Linux Landlock：非特权进程的叠加式文件系统限制](https://www.kernel.org/doc/html/latest/userspace-api/landlock.html)
- [Docker bind mounts：挂载默认可写及其宿主文件系统影响](https://docs.docker.com/engine/storage/bind-mounts/)
