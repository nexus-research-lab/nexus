# Codex Windows 沙箱源码差异审计

状态：non-normative，2026-09-15，审计进行中。本文记录已读源码和影响实施顺序的差异，不代表 Nexus 或 Codex 安装包实机验收。当前 Nexus 产品合同仍在 `docs/specs/desktop-sandbox-spec.md`。

## 可复现来源

从 OpenAI 官方仓库获取并固定到 [`4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd`](https://github.com/openai/codex/tree/4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd)。本轮读取 Windows sandbox、Windows sandbox service、core 工具编排源码。公开仓库状态不证明当前安装的 Codex App 使用相同版本。

## 已确认的链路与差异

| 阶段 | 固定源码证据 | 对 Nexus 的影响 |
| --- | --- | --- |
| 系统资源配置 | `windows-sandbox-service/src/ipc/request.rs` 的 `ServiceRequest` 只有 RegisterInstallation 与 ProvisionSandbox；`provisioning.rs` 执行配置流程 | 不应把 provisioning service 推导为每条命令的高权限执行 broker；原 broker 计划退回候选 |
| 配置身份 | `windows-sandbox-service/src/package_identity.rs` 验证安装包身份与管道模拟用户对应的进程 token；`ipc/authentication.rs` 拒绝服务账号和沙箱账号发起配置 | Nexus 的普通用户 Inno 安装不能直接照搬包身份校验；需单独形成部署信任设计 |
| 专用账号 | `windows-sandbox-rs/src/setup_provisioning/sandbox_users.rs` 配置 online/offline 两个账号、专用组、随机密码和秘密存储 | 只创建一个临时普通账号的测试并未复现完整账户/网络策略 |
| runner 启动 | `windows-sandbox-rs/src/elevated/runner_client.rs:324` 起，宿主 `CreateProcessWithLogonW` 启动 runner；注册 alias 路径用 LOGON_WITH_PROFILE，其他路径为 0；管道握手前保留进程句柄并验证目标 | 最新 Nexus broker 测试使用 CreateProcessWithTokenW，不能视为同一启动环境；也不能断言始终需要加载 profile |
| token | `windows-sandbox-rs/src/bin/command_runner/win.rs` 从 runner 当前 token 派生；`token.rs:450` 起使用 DISABLE_MAX_PRIVILEGE、LUA_TOKEN、WRITE_RESTRICTED，restricting SIDs 包含 capability、执行账号、可选额外身份、logon、World | Nexus 当前工厂使用完整限制且不包含执行账号；最新 broker 测试也省略执行账号。两者均非逐项复现，不应单凭其 DLL/后代失败判断 Codex 设计不可行 |
| 默认对象 ACL | `token.rs:513` 附近将 logon、World、capability 交给 default DACL，明确排除额外身份 marker | Nexus 自定义 token 对象及默认 DACL 是另一处差异；必须连同进程/线程访问测试审查，不能孤立恢复标志位 |
| 桌面 | `desktop.rs:275` 起，宿主按账号和有效策略持有 private desktop；注释说明 LogonW 的 logon SID 共享，ACL 管理权限给宿主 user SID；runner 打开此桌面 | 原始启动 API、window station、桌面 ACL、profile 必须作为一组核验。现有 0xc0000142 只证明实验失败，根因仍未证实 |
| 进程创建 | `process.rs:95` 起，CreateProcessAsUserW 配合显式 lpDesktop、环境、Job 属性和 stdio handle list；注释直接提及 PowerShell 的 STATUS_DLL_INIT_FAILED | Nexus 已有 Job/句柄组件可继续复用，但不能把 suspended 后赋 Job 的测试当作原子准入验收 |
| 文件和网络 | `setup_provisioning.rs` 调用 read ACL、persistent deny-read、能力 SID deny-write；offline firewall 配置单独建立 | WRITE_RESTRICTED 不是完整文件读取或网络边界；需继续读实际 ACL 与网络策略，禁止只复制 token |
| 拒绝后的审批 | `core/src/tools/orchestrator.rs:360` 起按工具能力、审批模式、是否允许越界分支；strict auto-review 的沙箱外重试重新审核，Never/OnRequest 通常返回原拒绝 | “沙箱拒绝必然提权”不成立。Nexus 保持工具和越界授权独立，并继续遵守结果不明副作用不自动重放要求；不能机械照搬重跑分支 |

## 文件、网络与凭据的组合边界

继续读取同一固定版本后确认：

- `windows-sandbox-rs/src/setup_provisioning.rs:900` 起，显式 deny-read ACL 同步完成后才继续；一般 read grant 可走后台 helper。二者的失败与等待语义不同，不能统一当作可异步补齐的兼容权限。
- 同文件的 write-root 循环根据根目录生成 capability SID，并检查 sandbox group 与该 capability 的授权；deny-write carveout 按关联能力设置。Nexus 的 owner 隔离、并发任务与撤销仍需另行验证，不能把长期目录授权当作单命令租约。
- `setup_provisioning/firewall.rs` 以 offline 账号 SID 建立非 loopback 入站/出站、loopback TCP 排除代理端口、loopback UDP 等规则，并检查本地规则是否能够生效。域策略阻止本地规则不能被当作“配置成功”。
- `wfp/filter_specs.rs` 另含 IPv4/IPv6 ICMP、DNS 53 和 DNS-over-TLS 853 等阻断；`setup_provisioning.rs:746` 起，常规配置对 WFP 安装采用 best-effort，修复被禁用账号时则要求成功后才重新允许登录。这是参考源码的真实差异，不证明所有网络路径已经隔离。Nexus 对其声称强制的网络边界仍必须 fail closed，不能照搬 best-effort 后声称完整验收。
- `setup_provisioning/sandbox_users.rs` 的 `write_secrets` 使用 DPAPI；`dpapi.rs` 使用 LOCAL_MACHINE 范围。`setup_provisioning.rs:808` 起另对秘密目录设置 sandbox group DENY_ACCESS。因此凭据保护是加密与目录 ACL 的组合，机器级加密本身不是账号间访问边界；不能复用 Nexus 现有 Connector 明文回退。

以上来源均在固定提交的 [Windows sandbox 源码目录](https://github.com/openai/codex/tree/4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd/codex-rs/windows-sandbox-rs/src)。这是静态源码事实，未运行其 Windows 配置流程，也未修改本机账号、系统 ACL 或防火墙。

## macOS 与完全访问的源码交叉核验

同版本 `codex-rs/sandboxing/src/seatbelt.rs` 固定使用 `/usr/bin/sandbox-exec`；`seatbelt_base_policy.sbpl` 以 deny default 开始，允许 process-exec/process-fork，并明确注明后代继承。文件和网络例外由策略生成加入。这支持复用 nxs 的 Seatbelt 路径，但不能据此声称普通宿主内的文件工具也被覆盖。

`sandboxing/src/policy_transforms.rs:646` 的 `should_require_platform_sandbox` 先检查 managed network；有强制网络要求时仍返回 true。没有该要求且网络已启用时，无限制文件策略返回 false。`manager.rs` 再选平台后端或 None。故“完全访问”不能被抽象为无条件忽略所有组织/网络要求，也不等于管理员身份。Nexus 保留当前产品合同：停止额外桌面命令限制，既有业务授权、OS 权限与服务器 owner 隔离不变。

来源：[跨平台策略实现](https://github.com/openai/codex/tree/4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd/codex-rs/sandboxing/src)。本轮只核验策略选择与继承声明，未复现 Codex App 设置到执行请求的完整映射。

## 下一阶段参考测试与实施顺序

先增加独立的参考路径测试，不能修改现有失败断言来获得绿色结果。参考 fixture 的职责固定为：宿主用 LogonW 启动普通账号 runner，runner 从自身 token 派生与固定源码一致的 SID/flags/default DACL，使用明确桌面、环境、stdio 与原子 Job 创建固定测试程序。runner 不继承 CI 凭据，不接收任意外部命令。该 fixture 是组合行为核验，不构成完整 Codex 复刻或生产后端。

| 必测行为 | 需要的证据 | 不足以替代的证据 |
| --- | --- | --- |
| 系统 PowerShell 与 Go 后代 | 启动成功、预定退出码、实际受限 token、子孙仍在 Job | cmd.exe 成功或仅交叉编译 |
| 控制面访问 | 子进程对宿主及 runner 的 process/thread/token 权限逐项结果 | 仅证明宿主账号不同 |
| 文件授权 | 允许根写入、根外拒绝、deny-read、deny-write、重解析别名与并发策略 | 只验证 token 的 IsTokenRestricted |
| 网络授权 | IPv4/IPv6、TCP/UDP、DNS、loopback、代理批准/拒绝与后代直连 | 仅检查 HTTP_PROXY 或规则存在 |
| 取消与故障 | 精确 Job 终止和空确认、断管道及崩溃时不重放 | 根进程退出或句柄关闭 |

先完成启动与控制面比较；若参考组合仍不能满足 Nexus 的隔离要求，记录哪一条具体边界失败，再决定 runner 加固还是额外 broker。之后实现资源配置、安装信任与完整文件/网络边界，最后接入产品权限链并完成跨平台回归。不得为提高兼容性放宽已有领域授权。

## 实施前仍需完成

1. 已读上述资源配置组合；仍需追完 online/offline 选择、连接身份绑定、并发撤销，并实测普通 runner 与受限子进程间的进程、线程和 token 边界。现有 Nexus 同账号探针是本地设计的负面证据，不是已证明的 Codex 漏洞。
2. 以同一固定版本建立明确的参考测试：保持启动 API、账号权限、桌面、token SID/default DACL、环境及 Job 一致，再逐项施加 Nexus 的额外隔离要求。保留失败证据，不通过拓宽系统 ACL 消除测试失败。
3. 分别决定 provisioning 与执行职责、安装包信任和升级策略。只有证据证明参考路径无法满足目标时，才选额外命令 broker，并写清必要性和回归成本。
4. 继续追 macOS 的策略生成/子进程继承与工具覆盖、Full Access 配置映射、审批取消与恢复。此次审计尚未证明这些完整链路，不能据此关闭 Goal。

本轮只调整设计依据，没有启用 Windows 后端、变更权限规则或运行新的原生实验。文档检查不能替代 Windows 10/11、macOS 安装包与 Linux 隔离验收。
