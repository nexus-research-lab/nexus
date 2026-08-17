# Linux Runtime 隔离运维

Nexus 的强隔离模式让产品 server 继续以普通 `nexus-host` 用户运行；只有
root-owned launcher 负责创建不可登录的 runtime 用户、切换 UID/GID、应用
POSIX ACL，并在 `exec` nxs 或 Claude 前设置 `no_new_privs`、进入 Landlock
domain。

`ensure-host`、`ensure-user`、项目授权和 policy prepare 等管理子命令会在
校验 host 调用方与 launcher 文件后，将 real/effective/fs UID/GID 统一提升为
root，再遍历和修复 ACL。普通 runtime argv 不经过该提升路径，仍直接降到
owner UID/GID、清空非授权组并进入 Landlock。

## 启用条件

- 原生 Linux 或具备可靠 POSIX ACL/xattr 语义的 Linux volume；
- 内核支持 `openat2(2)` 的 `RESOLVE_NO_SYMLINKS` 与 Landlock ABI 3 及以上，
  且容器 seccomp 没有阻断这些 syscall；
- launcher 配置、runtime 二进制和状态卷都由可信管理员控制；
- state root 的父目录允许 runtime UID 仅做目录穿越；原生部署优先使用
  `/var/lib/nexus`，不要把它放在不可穿越的 `0700` 宿主 home 下面；
- 不使用 `no-new-privileges` 启动 setuid launcher。

当前 [Moby/Docker 默认 seccomp profile](https://github.com/moby/profiles/blob/main/seccomp/default.json)
已放行 `openat2` 和三个 Landlock syscall；自定义加固 profile 必须显式保留
`landlock_create_ruleset`、`landlock_add_rule`、`landlock_restrict_self`。
不应改用 `seccomp=unconfined` 绕过配置问题，`ensure-host` 会在启用时给出
fail-closed 诊断。

macOS/Windows App 保持单用户模型，不启用 OS identity。Docker Desktop 的
bind mount 也不声明等价安全等级。

`enforce` 只接受标准的 `<state_root>/users/<owner>/workspace` 与
`<state_root>/shared-workspaces`。仍把 `WORKSPACE_PATH` 指到状态根之外的旧部署
应先迁回标准布局，或暂时使用 `audit`。

## 安装

1. 创建普通宿主账号/组，例如 `nexus-host`，禁止 runtime 加入该组。
2. 构建 `./cmd/nexus-runtime-launcher`，安装为
   `/usr/local/libexec/nexus-runtime-launcher`。
3. 将 launcher 设为 `root:nexus-host`、mode `4750`。
4. 确认配置中列出的 nxs/Claude 可执行文件均由 root 持有，且 group/other
   不可写；复制 `deploy/runtime-isolation.example.json` 到
   `/etc/nexus/runtime-isolation.json`，填写真实 state root、宿主 UID/GID
   和两个 runtime 的固定可执行路径；文件必须由 root 持有，且 group/other
   不可写。state root 的 `read_only_roots` 会被整理为宿主组可维护、runtime
   只读的目录；`/opt` 等外部应用包不会被递归改权限，必须预先由 root 或
   `host_uid` 持有且 group/other 不可写。只读根不能覆盖 `users/`、共享项目或
   `app/data|config|logs|cache`，也不要把 `.env`、凭据或其他宿主秘密放进去；
   launcher 会拒绝把 `/etc`、`/usr`、`/var` 等宽泛系统根直接作为资源目录。
5. 设置：

   ```text
   NEXUS_RUNTIME_ISOLATION_MODE=enforce
   NEXUS_RUNTIME_LAUNCHER_PATH=/usr/local/libexec/nexus-runtime-launcher
   ```

### 启用 per-user cgroup 回收

第一阶段的 UID/GID + Landlock 已经限制文件访问；如果还要保证 runtime
主动 double-fork 后的后台进程在权限撤销时一并退出，可在 launcher 配置中
启用 cgroup v2：

```json
{
  "cgroup_root": "/sys/fs/cgroup/nexus",
  "cgroup_required": true
}
```

`cgroup_root` 必须是 cgroup v2 的 Nexus 子目录，不能直接指向
`/sys/fs/cgroup`，且必须由 root 持有、不可被 runtime 写入。launcher 会为每个
稳定 OS 用户创建一个同名子 cgroup，在 `exec` 前写入 `cgroup.procs`；控制面
撤销项目成员关系或关闭 owner runtime 时调用 `stop-user`，通过
`cgroup.kill` 回收整个进程树。`cgroup_required=true` 时，挂载类型、权限或
`cgroup.kill` 缺失都会让 `ensure-host` 和 runtime 启动失败。留空
`cgroup_root` 表示由 launcher 通过 owner UID 与 Unix session 校验处理中断、关闭
和同 session 子进程清理；主动 `setsid`/double-fork 脱离原 session 的 orphan
仍需 cgroup 才能可靠回收。

Docker 部署要把同一 cgroup v2 层以受控的可管理方式提供给 launcher；
Docker Desktop 不承诺这套语义，原生 Linux 或专用 Linux worker 才是目标现场。

启动时 `internal/migration/runtime_identity.go` 会为数据库中的既有用户恢复
OS 账号、收紧宿主 `app/data|config|logs|cache` 权限并迁移用户树 ACL。
launcher 将 state root 与 `users/` 设为宿主组可管理、其他 UID 仅可穿越；
真正的用户根仍由 private GID、named host ACL 和 `other=---` 隔离。
平台 Skill 和 `app/.agents/bin` 作为配置文件中显式声明的只读根保留给
runtime。映射保存在
`<state_root>/app/data/runtime-isolation/registry.json`，UID/GID 不因容器或
server 重启而重新分配。

用户边界根保持 root-owned；owner 的 `workspace/` 根由宿主 UID 持有并使用 setgid+sticky，
Agent workspace 边界保持 root-owned、private group 可写，边界内内容归对应
runtime UID。这个布局允许 runtime 正常开发，但不能把宿主文件 broker 随后会
打开的 workspace 根替换成 symlink。共享项目根保持
`root:<project_gid>`。launcher 只为当前 policy CWD/授权根注入临时 Git
`safe.directory`，不会使用全局 `safe.directory=*`。

启动迁移会拒绝跨用户、跨项目或指向隔离根外的存量硬链接。ACL/chown 作用于
inode，无法安全地区分这类路径别名；运维人员应先将报错路径复制成独立文件，
确认数据一致后再启用 `enforce`。

`audit` 模式只运行同一套 nxs/Claude PreToolUse 检查并记录越界事件，不切换
OS 身份，也不宣称具备强隔离。`off` 完全保持旧行为。

`enforce` 仍禁止普通 Agent 执行 `nexusctl`；官方容器把它设为 `root:agent 0750`，
由 DAC 阻止隔离 runtime UID 执行。`nexuscfg` 不直接向 runtime 打开数据库：宿主为
每个可信交互 round 注入 loopback broker 地址与随机 capability，CLI 只转发命令，
broker 再按当前 Agent/DM/Room 身份调用 configuration 服务。官方容器因此把
`/usr/local/bin/nexuscfg` 设为 `root:root 0755`，但缺少有效 capability、round 已结束、
并发 round 或越权 operation 都会失败。Agent-facing `/usr/local/bin/nexus` 使用相同部署
边界；当前 `automation` 子命令另由 Automation round capability 固定 source、Session
和可选 job/run，mutation 还必须经过 plan digest、revision 与当前会话真人确认。

Nexus 主智能体是宿主控制面主体，在 enforce 下保留宿主身份并通过
`NEXUSCTL_COMMAND_PATH` 调用当前 owner scope 的 CLI。宿主注入
`NEXUS_RUNTIME_SCOPE_MODE` 与 owner 后，CLI 帮助会隐藏人工作用域选择参数；显式覆盖
会返回可重试的 usage 错误。Hook 对 `nexusctl`、`nexuscfg` 和 `nexus` 的作用域/capability
覆盖继续早期拒绝；最终权限分别由 DAC/宿主 scope、configuration 角色矩阵与对应领域
command service 收口。

为兼容终端、临时文件和部分开发工具，当前 Landlock 规则允许 runtime 使用
`/dev/null`、`/dev/tty`，并对 `/dev/shm` 保留共享写入能力；这不是跨租户的
机密性边界。若部署把恶意用户视为同一宿主上的强对抗租户，应在 worker/container
层提供每用户的 IPC/shm 隔离，再把本 launcher 作为文件系统纵深防线。

Landlock 不约束 Nexus server 进程代 runtime 执行的 SDK MCP 或 workspace API
文件操作。这些入口已在 owner 授权后通过 `internal/infra/confinedfs` 持有
`os.Root` 目录 fd；workspace 下载直接使用已打开文件，用户 Skill、transcript、
artifact、偏好和图片也不再在校验后按原始绝对路径重新打开。`os.Root` 不隔离
bind mount 或设备文件，因此生产容器仍须移除 mount/mknod capability，宿主读接口
也只接受普通文件。

认证部署的 Skill 导入只接受上传归档或平台内部下载，不接受 HTTP `local_path`；
后者只保留给未启用认证的本地单用户部署。

## 共享项目 ACL

共享目录必须位于 `<state_root>/shared-workspaces/`。管理员或后续控制面使用
同一个 launcher 管理项目组：

```bash
/usr/local/libexec/nexus-runtime-launcher \
  project-ensure --project project-a \
  --path /var/lib/nexus/shared-workspaces/project-a \
  --owner user-a

/usr/local/libexec/nexus-runtime-launcher \
  project-grant --project project-a --owner user-a --access write

/usr/local/libexec/nexus-runtime-launcher \
  project-grant --project project-a --owner user-b --access read
```

`read` 使用 named user ACL，不把只读用户加入项目组；`write` 同时授予项目
GID。撤销使用 `--access none`。通过产品控制面变更成员后会自动取消该用户
运行中的 round，并关闭其全部热 runtime；新 session 只取得当前
workspace/read-root 实际需要的项目 GID。直接使用 launcher CLI 的运维流程仍
需显式调用 `stop-user` 关闭受影响 runtime。

需要单独回收某个 owner 的 orphan runtime 时，可调用：

```bash
/usr/local/libexec/nexus-runtime-launcher \
  stop-user --owner user-a
```

产品控制面使用同一 launcher，不直接修改系统组或 ACL：

```text
GET  <api_prefix>/projects
POST <api_prefix>/projects
PUT  <api_prefix>/projects/{project_id}/members/{owner_user_id}
```

`POST` 请求体为 `{"project_id":"project-a"}`；创建者的 write 授权与 registry
创建在同一 launcher 锁内完成。成员更新请求体为 `{"access":"read|write|none"}`。
普通成员只能看到自己已加入的项目，且不会看到其他成员；完整列表和成员变更仅
对 owner/admin（成员变更仅 admin）开放。CLI 可用 `project-list` 检查 launcher
registry。

runtime Manager 会把首个 client 的 `NEXUS_RUNTIME_USER_ID` 固定到 session；
同一 session key 之后若携带其他 owner 或缺失 owner 会直接失败，不能借热
client 跨租户复用。

## 失败语义

`enforce` 是 fail closed：配置权限错误、UID/GID 冲突、ACL/xattr 不可用、
launcher 票据失效、runtime 路径不在 allowlist 或 Landlock 不可用都会阻止
runtime 启动。不要通过改成宽松目录权限绕过错误；先保持 `audit`，修复宿主
能力后再切换 `enforce`。
