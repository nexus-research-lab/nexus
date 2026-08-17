# Workspace 隔离与多用户运行时规范

## 1. 文档状态

- 状态：目录布局、迁移、Linux UID/GID、项目 ACL 控制面、nxs/Claude Hook、
  项目成员管理 UI、Landlock launcher、主要宿主文件 broker 的 confined-fd
  边界和 opt-in per-user cgroup 回收已实现；默认仍为 `off`，原生 Linux 上需
  显式启用 `enforce` 并完成部署验收
- 核对日期：2026-08-11
- 适用范围：Linux 服务端多用户部署
- 当前结论：以操作系统 UID/GID 为主边界，项目组/ACL 负责显式协作，runtime hook 和最终路径校验负责策略收口；`.nexus` 是统一状态根，`app/` 保存 Nexus 宿主数据，runtime 配置和会话按用户独立存放

本文定义安全边界和运行契约。当前实现提供 opt-in 的 Linux 强隔离链路；由于本地
macOS 无法执行 setuid、POSIX ACL 和 Landlock，发布前仍需在目标 Linux 内核、文件系统
和容器 seccomp 配置上完成验收；`enforce` 配置不得关闭 Landlock。

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
- 用文件权限替代 HTTP、WebSocket、`nexusctl` / `nexuscfg` 和 storage 层的 owner 授权。
- 不实现每个 Agent 一个操作系统用户。
- 不承诺网络、CPU、内存、PID namespace 或设备 ioctl 隔离。Landlock 只负责
  runtime 进程的文件系统访问集合；cgroup 仅在显式配置时负责 owner 进程树回收。
- 在 macOS/Windows 桌面端强制创建本地系统用户。
- 不定义独立的组织/成员层级；当前租户边界是 `owner_user_id`。

## 3. 威胁模型

### 3.1 纳入的攻击者

- 能控制 Agent 提示词、Skill、project hook 或 Bash 输入的模型/运行时进程。
- 用户 A 试图读取用户 B 的 workspace、transcript、配置或缓存。
- 被误配置的 runtime 通过绝对路径、相对路径、符号链接、Shell 展开或控制面 CLI scope 访问越界资源。

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
4. **宿主根与用户根分区**：`.nexus` 是 Nexus 的统一状态命名空间；`app/` 只保存宿主控制面和宿主共享资源，当前 owner 的全部数据统一进入且只授权 `users/<owner_user_id>/`。
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
- 当前不提供 Agent 级 UID 隔离；用户级身份是本规范的安全基线。

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

启动期的跨用户、跨项目和隔离根外硬链接校验也由已提升完整 root 身份的
launcher 在修改 ACL 前完成；host app UID 不直接遍历 runtime 创建的 `0700`
私有会话目录。

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

桌面端改变数据目录时迁移的是完整 `NEXUS_STATE_ROOT`，不拆分 `app/` 与 `users/`。原生宿主在确认后退出 sidecar，离线复制状态根，通过宿主外的启动指针切换到目标并直接重启；新实例完成数据库、transcript 与 Room 结构化绝对路径重映射后才清理旧根，启动失败则回滚指针。Linux 服务端和 `enforce` 部署的状态根仍由部署配置与权限模型管理，不提供应用内迁移。

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
        .rooms/                       # owner 级公共附件，runtime 可读写但不承载控制状态
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
      state/                          # 宿主持久化的 owner 状态，不属于 runtime config 根
        rooms/                        # owner 级 Room overlay、消息游标、handoff 与延迟唤醒

  shared-workspaces/
    <shared_workspace_id>/            # 项目 group/ACL 共享目录
```

桌面端默认使用 `~/.nexus`，也可以整体迁移这个状态根；Docker 可以把宿主目录挂载到 `/home/agent/.nexus`；服务端也可以把整个状态根映射到 `/var/lib/nexus`。`app/` 与 `users/` 必须是不同权限子树，但始终属于同一个状态根。

权限约束：

- `.nexus` 与 `users/` 只给 runtime UID 目录穿越位；`nexus-host` 通过宿主组
  继续创建用户目录，runtime 不能列举根目录；
- `.nexus/app` 及其 `data/config/logs` 只允许 `nexus-host` 访问；当前 runtime
  对自己的 `users/<owner_user_id>` 数据根读写，不能继承 `app/` 或其他 owner 根。
- `users/<owner_user_id>` 的边界目录由 root 与对应 private group 控制；父目录只允许穿越，不允许 runtime 列举全部用户，边界内 `workspace/runtime/state` 使用同一 private group。
- owner 的 `workspace/` 根由 `nexus-host` 持有并启用 setgid+sticky；每个
  `workspace/<agent_id>` 边界也保持 root-owned、private group 可写，runtime
  可以正常改其内容，却不能把宿主随后会打开的根目录替换成 symlink。边界内文件
  与用户 runtime 内容归对应 runtime UID，宿主通过 named-user ACL 访问。
- workspace 和 shared workspace 使用 setgid；default ACL 只授予 Nexus 宿主、当前运行组和明确的项目组。
- 普通文件默认不允许 `other` 读写，runtime 使用 `umask 0007`。
- shared workspace 成员关系由 Nexus 控制，Agent 不能自行改组、改 ACL 或扩大 scope。
- 宿主打开 `state/rooms` 时必须从 `NEXUS_STATE_ROOT` 的目录 fd 逐级进入，
  每个目录段拒绝 symlink 并核对 inode；ledger 文件同时拒绝 symlink、
  多硬链接与校验后替换，不能只依赖字符串前缀或一次 `EvalSymlinks`。
- 宿主读取 Agent workspace 与 `workspace/.rooms` 附件时同样从 owner 管理根
  逐级固定目录 fd；图片内容直接从校验后打开的文件 fd 读取，不能先返回绝对
  路径再由宿主重新打开。

### 6.2 nxs 与 Claude 的用户级配置根

`projects` 是 runtime transcript store，不是用户协作项目目录。`<user_root>` 指当前用户专属的 runtime 根，nxs 和 Claude 都直接使用它：

```text
NEXUS_CONFIG_DIR=<user_root>
CLAUDE_CONFIG_DIR=<user_root>
HOME=<user_root>/home
```

nxs 和 Claude 都固定使用同一个 `<user_root>/projects`。Claude 不能独立配置 `projects` 子目录，但可以配置整个 `CLAUDE_CONFIG_DIR`；只要父级指向当前用户 runtime 根，就不构成隔离障碍。

bridge 可以继续把 `CLAUDE_CONFIG_DIR` 与 `NEXUS_CONFIG_DIR` 保持同步，但同步源必须是宿主按 `owner_user_id` 计算出的 `<user_root>`，不能继承 server 的 `.nexus/app` host root。

`NEXUS_CONFIG_DIR` 的语义分成两层：

- server 进程的宿主路径由 `appfs.AppDir()` 计算为 `.nexus/app`；`NEXUS_STATE_ROOT`
  是状态根的唯一新配置，`NEXUS_CONFIG_DIR` 只作为旧版本状态根输入兼容。
- runtime 子进程使用的统一 `<user_root>`：每次按 `owner_user_id` 注入，不能继承
  server 的状态根或宿主目录。

宿主读取 transcript 时不能继续只依赖 server 进程全局的 `NEXUS_CONFIG_DIR`。Agent/session 必须携带或可推导自己的 `RuntimeConfigDir`，`AgentHistoryStore` 从该用户级 `<user_root>/projects` 读取。

关键实现入口：

- `nexus/internal/service/agent/workspace.go` / `ready.go`
- `nexus/internal/storage/workspace/transcript_path.go`
- `nexus/internal/runtime/clientopts/agent_client.go`
- `nexus/deploy/docker-compose.yml`
- `nexus/internal/infra/appfs/config_dir.go`

当前版本只把 canonical `.nexus/app`、`.nexus/users/<owner>` 与
`.nexus/shared-workspaces` 作为运行时读写布局。启动期仍保留版本化的旧状态根与
workspace 迁移，以允许用户跨多个发布版本直接升级；迁移只执行 rename、不覆盖式
合并和 owner 数据库映射，不提供旧路径运行时回读。v0.1.30 首次启用最终的
owner 目录布局时曾遗漏 v0.1.27 与 v0.1.28 的直接升级入口；若受影响版本已误建
`app/data/nexus.db` 与 `users/` 数据，先把两个新分支隔离到
`app/.migration-quarantine/skipped-state-layout-v1/`，恢复旧库、Agent workspace、
transcript 与 Room 源文件后，再以旧数据优先、外键完整的单事务补入新库非冲突
记录，并把不冲突的 owner 文件并回 canonical `users/`。文件冲突继续保留在隔离区。

针对旧版曾将 Room 文件写入共享 `app/rooms` 的安全问题，启动期保留一个有完成标记的定向迁移：

- 先从数据库按 `conversation_id` 确认 `owner_user_id`，再迁入对应
  `users/<owner>/state/rooms`，不能从目录名或文件内声明猜测 owner；
- `overlay.jsonl`、directed message、消费游标、public handoff 与 delayed
  wake 均按 owner 拆分，JSONL 采用规范化内容去重，支持中断后重试；
- 旧 `attachments/` 单独迁入 `users/<owner>/workspace/.rooms/<conversation>/`
  的对应相对路径；Room ledger 迁入同一 owner 数据根后也遵循 owner 根权限；
- 无法确认 owner、混合多个 conversation、包含符号链接/硬链接/特殊文件的状态移入宿主私有
  `app/.migration-quarantine/room-state-v1`，不会暴露给任意 runtime；
- 迁移告警不阻断 server 启动，但新版本不会再回读 `app/rooms`。完成后移除
  旧目录并在 `app/.migrations` 写入标记。

除这类明确版本化、可审计的安全升级迁移外，运行中的 server 不猜测历史目录
归属，也不把 `app/` 当作用户 runtime 的兼容回退。

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

nxs 和 Claude 共用 `<user_root>/projects`。全局 Skill、二进制和只读模板使用 root-owned 只读目录；用户 Skill、npm/uv/pip cache 和私有临时文件写入 `<user_root>`。

为保持桌面 App 与 Web runtime 的命令行为一致，Unix runtime 另外把 `/tmp` 作为显式共享兼容读写根。`/tmp` 依赖操作系统的 sticky bit 与每用户 UID/GID 防止用户删除或覆盖其他用户拥有的文件，但目录中的可读文件名和权限宽松的内容可能被其他 runtime 看见；凭据、provider 响应和其他敏感数据仍必须使用 `$TMPDIR`，不能写入 `/tmp`。

server 的 host root 不得通过完整环境继承给 runtime。runtime 的 `NEXUS_CONFIG_DIR` 必须由 `owner_user_id -> UserScope -> user_root` 显式计算，不能由模型、Skill、project hook 或请求参数直接指定。

Nexus 管理的 nxs 长期记忆根固定为当前 Agent workspace：

```text
NEXUS_MEMORY_DIR=<agent_workspace>
<agent_workspace>/MEMORY.md
<agent_workspace>/memory/
```

宿主继承环境与请求级 `ExtraEnv` 都不能改写该值，`NEXUS_ENABLE_REMOTE_MEMORY`
和 `NEXUS_REMOTE_MEMORY_DIR` 在受管 runtime 中固定关闭。这样 SDK 读写、Web
只读投影与 workspace policy 始终引用同一个 owner/Agent 路径；SDK 独立运行时
自己的可配置记忆根不受这一产品侧约束影响。

系统包安装不授予 runtime sudo。需要安装系统包时，由宿主执行固定 allowlist 的 broker；普通开发依赖优先使用用户级安装。

## 8. Hook 与最终访问校验

### 8.1 Mandatory PreToolUse policy

宿主为 nxs 和 Claude 注入同一份 `WorkspacePolicyHook`：

- 绑定 `owner_user_id`、owner 数据根、当前 project roots 和 policy generation；
- 对 `Read/Write/Edit/Glob/Grep` 等路径工具执行路径归一化和 root containment；
- 当前 owner 的 `users/<owner_user_id>` 整棵数据根统一允许读写，不再为
  transcript、session-memory 或 `state` 维护单文件例外；跨 owner 路径仍拒绝；
- 对 Bash 只做显式绝对路径、`..` 路径和 `nexusctl` / `nexuscfg` / `nexus` 管理入口的早期检查；普通系统命令
  仍可运行，最终写入/删除/重命名由 OS DAC/ACL 与 Landlock 决定；
- enforce Hook 对普通 Agent 的 `nexusctl` 管理命令做早期拒绝；打包部署额外把
  `nexusctl` executable 设为宿主组专用。Nexus 主智能体是宿主控制面主体，可使用
  `NEXUSCTL_COMMAND_PATH` 的当前 owner scope。`nexuscfg` 对所有交互 Agent 可执行，但
  只把命令转发给宿主 loopback broker；宿主签发的 round capability 固定 owner、Agent、
  DM/Room 和 runtime lease，configuration 角色矩阵决定最终 operation。CLI 作用域或
  capability 覆盖返回可重试错误，Hook 对这类 shell 文本仍做早期拒绝。Agent-facing
  `nexus automation` 同样只转发给 loopback broker；后台 run 固定 job/run 且只读，
  交互 mutation 还需 service plan/revision/digest 与当前会话真人确认；
- 不返回 `updatedInput`，只允许放行或拒绝；
- Hook 本身不返回 `allow` 决策，避免覆盖其他 hook 或用户权限处理；越界时返回
  `deny`。Hook 失效不构成安全放行，enforce 进程仍必须通过 launcher 的最终边界；
- 对模型返回泛化原因，详细路径和身份写入内部审计事件。

### 8.2 不可信 hook

宿主把 mandatory policy 放在初始化时已知 Hook 的最后，使它检查前序 Hook
更新后的输入并保留最终否决权。用户设置、project hook 和模型提示词不能从
宿主 options 中移除它；nxs 运行期动态注册的 Skill hook 仍可能改变 Hook 合并
顺序，因此 Hook 的 `deny` 不是可信的最终安全边界。

`NEXUS_SIMPLE`、`CLAUDE_CODE_SIMPLE`、`--bare` 或类似模式不能绕过最终访问校验；
launcher 会拒绝禁用 hook 的 argv/环境。即使 runtime hook 被跳过，Landlock 仍在
整个 nxs/Claude 进程（包括其子进程）上生效。`nexusctl`、`nexuscfg` 与 Agent-facing `nexus` 属于控制面而不是
用户文件系统。生产部署必须继续通过 DAC/容器镜像边界让普通 runtime UID 无法执行
`nexusctl`；`nexuscfg` / `nexus` 的最终边界是宿主 round capability 与对应领域服务授权，
不能只依赖 Hook 文本识别。主智能体保留宿主身份是明确的控制面信任边界，不宣称
具备普通 Agent 的 Landlock 隔离等级。

### 8.3 Final path guard

最终 guard 由 root-owned launcher 在 runtime `exec` 前安装 Landlock ABI 3+ ruleset，并由
Linux 内核在每个受控文件系统系统调用上重新检查：

- 最终生效的输入，而不是 hook 之前的输入；
- 符号链接和重命名竞争；
- 相对路径、绝对路径和 Shell 展开结果；
- 读、写、创建、删除、执行的不同权限；
- 当前 OS identity 是否仍与 session policy 匹配；
- 允许的 workspace、用户 runtime、显式只读资源和项目 root；
- `make/rename/link/remove/truncate` 等创建与变更操作。

Hook 负责用户可见的早期反馈和审计，不能代替内核校验。当前实现没有独立 PID
namespace；`/proc` 元数据仍遵循宿主内核的常规 DAC/ptrace 语义，不应宣传为
进程表隐藏。

Landlock 只约束被 launcher `exec` 的 runtime 及其子进程。SDK 进程内 MCP、
HTTP workspace API 和其他宿主 broker 的文件系统调用发生在 `nexus-host`
进程中，不在该 Landlock domain 内。当前 workspace 读写/下载、附件与图片、
automation artifact、偏好、runtime settings、transcript/JSONL 和用户 Skill
registry 已统一先校验 owner，再通过 `internal/infra/confinedfs` 持有目录 fd
访问；下载直接消费已打开文件，不会在校验后重新按绝对路径打开。

`os.Root` 只在根目录 fd 成功打开后提供 containment，因此可被 runtime 替换的
路径不能直接充当信任根。launcher 将 host-owned 的 owner workspace 顶层设为
sticky，并把 Agent workspace 边界设为 root-owned；迁移遇到 workspace 顶层 symlink 会
fail closed。这样宿主打开的根 inode 与 runtime 可写内容分离，而不牺牲同一用户
在 workspace 内的正常协作。

Room ledger、owner-aware InputQueue 与 Room transcript 引用会先按 owner 计算
managed root，再从该根的目录 fd 逐段打开 workspace/session 路径；持久化的
绝对路径只用于定位，不能直接升级为可信根。普通文件打开时还会核对打开前后
inode，并在 Darwin/Linux 上拒绝多硬链接文件。

`os.Root` 不隔离 bind mount、设备文件或文件系统边界，因此宿主读接口同时拒绝
符号链接和非普通文件；部署仍不得给 runtime `CAP_SYS_ADMIN`、`CAP_MKNOD` 或可写
宿主 mount namespace。平台 Skill 构建、首次 owner/workspace 根创建和迁移器仍会
操作宿主推导出的固定路径，它们不接受 runtime 提供的任意目标路径，并由 root-owned
父目录、迁移锁和 launcher ACL 保护。

## 9. 控制面授权

OS 权限不能替代业务授权。以下入口必须继续按真实认证用户过滤：

- Agent、workspace、Room、DM 和 transcript API；
- `nexusctl` / `nexuscfg` 的 user/global scope；
- storage repository 的 `owner_user_id` 条件；
- workspace 列表、搜索、导出和恢复；
- MCP、connector、automation 和 provider 凭据读取。

任何从控制面返回其他用户 workspace、Agent 活动或凭据的路径，都必须在服务端修复；hook 只能作为额外阻断和审计。

认证部署不得接受 `local_path` 让 HTTP 用户要求宿主读取任意本地目录。外部 Skill
必须通过受限归档上传或由宿主内部下载到私有 staging；本地单用户部署保留
`local_path` 兼容能力。

## 10. 协作语义

### 10.1 默认

- 用户只能读写自己的 workspace。
- 同一用户的 Agent 共享该用户被授予的 private group。
- Agent 不自动获得其他用户的 workspace。

### 10.2 显式共享项目

- Nexus 的项目成员关系映射为项目 group/ACL。
- `GET /projects` 对普通成员只返回其已加入项目，并隐藏其他成员标识；owner/admin
  可查看完整项目 registry。
- `POST /projects` 创建项目时由 launcher 在同一 registry 锁内给创建者授予 write；
  对既有项目执行 ensure 不会自动加入调用者。
- `PUT /projects/{project_id}/members/{owner_user_id}` 仅允许 admin 变更
  `read` / `write` / `none`。
- 加入项目后，成员获得项目声明的 read-only 或 read-write 权限。
- 移除成员时，停止或重启受影响 runtime，撤销后续 session 的项目组；不依赖已有进程自行刷新。
- 项目组不授予 app data、全局 transcript 或 connector key 权限。

### 10.3 运行时组范围

runtime 只携带当前 session 所需的 private group 和明确授权的 project groups。不能把所有系统组、所有项目组或 server 的附加组原样继承给子进程。

## 11. 平台与部署矩阵

| 部署形态 | 本规范状态 | 说明 |
| --- | --- | --- |
| 原生 Linux 服务端 | 首选 | POSIX UID/GID、setgid、ACL 和 launcher 可控 |
| Linux Docker + state volume | 条件支持 | 可继续使用单一 `.nexus` volume，但 `app/` 与 `users/` 必须是不同权限子树 |
| Linux Docker + 宿主 bind mount | 条件支持 | 必须验证宿主 UID/GID、ACL、备份和恢复语义 |
| Docker Desktop macOS/Windows bind mount | 暂不承诺 | 文件共享层的 UID/GID 语义和性能需单独验证 |
| Nexus macOS/Windows 桌面端 | 保持单用户 | 不为本地用户创建额外系统账号 |
| 每用户独立容器/VM | 当前合同外 | 可由部署方作为额外 hostile-tenant 隔离层 |

## 12. 当前交付状态

### 12.1 基础控制面（已实现）

- 服务端和 runtime 环境改为 allowlist；
- 修复 app API 和控制面 CLI 的 owner 授权；
- 注入 deny-only PreToolUse hook；
- 禁止安全关键环境变量覆盖 policy；
- 记录越界尝试，不改变现有数据布局。

### 12.2 Linux per-user identity（已实现，默认关闭）

- 建立 `owner_user_id -> RuntimeIdentity` 映射；
- 实现受控 launcher/worker；
- 新建 workspace 和用户 runtime root 使用新的 UID/GID、setgid 和 default ACL；
- 为 nxs/Claude 注入同一个用户级 `<user_root>`，共用固定的 `projects` 子目录；
- nxs/Claude 都通过同一身份启动；
- App 和 Web 都通过同一个 `UserScope` 进入 launcher；
- `NEXUS_RUNTIME_ISOLATION_MODE` 是 runtime isolation 的唯一选择：`enforce` 仅在 Linux
  server 上可用；`audit` 只启用 Hook 和日志，`off` 保持兼容行为。认证状态不覆盖该配置。

### 12.3 存量迁移（启动迁移已实现，目标部署仍需验收）

- 停止受影响 runtime；
- 为每个用户创建 identity 和 `users/<owner_user_id>`；
- 按 owner 迁移 workspace、Skill、nxs/Claude session、`HOME` 和缓存；
- 使用 `stat`、ACL 检查和 checksum 验证迁移结果；
- 迁移失败时保留原目录，不覆盖原数据；
- 完成双用户负向访问测试后再切换默认根。

### 12.4 项目协作与纵深隔离（已实现，Linux 现场验收待完成）

- launcher 已提供 `project-ensure` / `project-grant` / `project-list`，HTTP
  控制面按角色和项目成员过滤；
- final path guard 已由 Landlock 覆盖整个 runtime；
- 主要宿主文件 broker 已统一接入 `os.Root` confined-fd 边界；
- 项目成员关系变化后，Manager 会按 `owner_user_id` 取消 round 并回收全部热
  runtime；session key 同时绑定 owner，拒绝跨 owner 复用；
- 运营设置已提供项目创建和成员 `read` / `write` / `none` 管理 UI；前端仅按
  角色调整交互，最终授权仍由服务端 owner/admin 规则判定；
- cgroup 仍需在目标 Linux 部署中显式配置并完成现场验收。

### 12.5 会话与 owner 进程树回收（已实现）

- bridge 的中断、关闭和遗留子进程清理统一调用 root-owned launcher；launcher
  先固定 pidfd，再校验目标属于当前 owner UID/cgroup 和对应 Unix session，避免
  跨 UID 信号失败或 PID 复用误杀；
- 未启用 cgroup 时仍可可靠回收同 session 进程；主动 `setsid`/double-fork 脱离
  session 的 orphan 不在该保证内；
- launcher 支持 cgroup v2 的 per-user 子 cgroup，并在 runtime `exec` 前写入
  `cgroup.procs`；
- `stop-user` 使用 `cgroup.kill` 回收主动 double-fork 的 orphan descendant；
- Manager 在项目权限撤销、owner 级关闭和最后一个热 session 关闭时触发回收，
  并在回收期间阻止新的同 owner session 插入；
- 默认不创建 cgroup；将 `cgroup_root` 指向 root-owned cgroup v2 子目录并设置
  `cgroup_required=true` 后，能力缺失会 fail closed；
- worker container、PID namespace 与部署级 seccomp 不属于当前合同。

## 13. 验收标准

### 13.1 负向测试

对用户 A、B 各创建 sentinel workspace，分别用 nxs 和 Claude 验证：

- `Read/Glob/Grep/Write/Edit` 访问 B 路径均失败；
- 相对路径、绝对路径、`..`、符号链接和硬链接均不能越界；
- Bash、Shell 展开、`find`、`cat` 和 `cp` 不能越过文件系统边界；
- 普通 Agent 的 `nexusctl` 管理命令被 Hook 拒绝，生产部署同时由 DAC 阻止 runtime
  UID 执行；普通 Agent 的 `nexuscfg` / `nexus` 只能使用宿主签发的当前 round capability，
  无法覆盖身份、作用域、job/run 或越权修改其他 Agent；
- simple/bare 模式不能绕过 final guard；
- runtime 环境中不存在 app DB、connector key 和 B 的 provider secret；
- `/proc` 不应暴露其他用户受 DAC 保护的环境/文件；共享 `/tmp`、共享 cache
  不作为用户 runtime 写入根。

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
- cgroup v2 启用时，关闭 owner 或撤销项目成员关系后，父进程及其
  double-fork 子进程均从目标 cgroup 消失。

## 14. 已决策与部署限制

1. 已决定仅对原生 Linux / 可靠 Linux volume 承诺 `enforce`；Docker Desktop 保持
   `off/audit` 兼容档。
2. 已采用 root-owned setuid launcher；独立 worker/container 不属于当前合同。
3. 项目协作采用“项目 GID + named-user ACL”的混合模型：write 成员加入项目组，
   read 成员只写 named ACL。
4. 同一用户的不同 Agent 共享用户私有组；项目组只按当前启动票据授予。
5. `app/` 与 `users/` 继续共用 `.nexus` volume，但由 launcher 收紧宿主 app 子树。
6. 现有系统包 broker/sudo 合同暂不在本变更扩大，runtime 不能通过 launcher 获得
   额外 sudo 权限。
7. Linux hostile-tenant 与 cgroup v2 仍需在目标内核现场验收；PID namespace、
   worker container 和部署级 seccomp profile 不属于当前合同。

## 15. 参考

- [Linux inode(7)：目录 setgid 与继承的 group ownership](https://www.man7.org/linux/man-pages/man7/inode.7.html)
- [Linux acl(5)：access ACL 与 default ACL](https://man7.org/linux/man-pages/man5/acl.5.html)
- [Linux Landlock：非特权进程的叠加式文件系统限制](https://www.kernel.org/doc/html/latest/userspace-api/landlock.html)
- [Docker bind mounts：挂载默认可写及其宿主文件系统影响](https://docs.docker.com/engine/storage/bind-mounts/)
