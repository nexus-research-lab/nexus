# 桌面沙箱现状、问题与 Codex 差距

状态：**2026-09-15 的审计快照，non-normative**。本文解释已有代码与证据，不定义新的产品行为。
当前合同见 [桌面沙箱规范](../../specs/desktop-sandbox-spec.md)；后续工作与状态只在 [开发计划](development-plan.md) 维护。

## 结论

Nexus 已有可选择启用的桌面命令沙箱、独立越界/网络审批、自动审核接入、权限切换时的 runtime 退休，以及显式支持检测。macOS 已有真实命令隔离测试；最新 SDK 又加入文件工具执行进程。**当前仍是默认关闭的实验集成，尚未形成完成安装、运行、恢复和升级验收的跨平台产品。**

主要缺口集中在：

1. Windows 的命令兼容性与控制面隔离还没有同时通过，生产后端仍明确不可用。
2. SDK 文件工具的进展没有形成可协商、可发布、可在任务界面确认的完整覆盖承诺。
3. 命令开始后的部分副作用、断连、进程树清理和批准恢复缺少完整的持久结果闭环。
4. 安装包版本组合、受限模式设置/修复、精确资源授权和跨平台验收仍未收口。

不需要重写现有权限系统。下一步应补齐这些边界，同时保留已验证的宿主装配、审批与原生组件。

## 审计基线与证据等级

| 对象 | 本次读取的基线 | 对结论的限制 |
| --- | --- | --- |
| Nexus | 928c7e803e22a8af605753a9c584a33faab2a39b；codex/desktop-sandbox-approvals | 比跟踪分支领先 4 个提交；另有会话滚动测试未跟踪文件，不属于本次改造 |
| Bridge | 3da56a2e05349044a0fe359aa237dfec078cddda | 比跟踪分支领先 1 个提交；Nexus 的 go.mod 已固定对应伪版本 |
| SDK | c824526230860fbfc2a28736980b5fa720dbffe4 | 另有 Skill/配置读取相关未提交改动；本次原生复测使用该提交的独立导出，未纳入这些改动 |
| 本地依赖 | Nexus 的 go.work 引用同级 Bridge | 普通 workspace 测试不能替代 GOWORK=off 的依赖验证 |
| Codex 公开源码 | 4e6450bbfd60bdfa845182f30aaa9d6f068e8bbd | 已核对本地固定副本；不等于用户当前安装的 Codex App 版本 |
| Codex 产品说明 | 2026-09-15 打开的官方文档 | 是当日公开行为说明；beta 与平台限制按原文保留 |

本文区分五种证据：源码存在、定向测试通过、原生 OS 行为通过、跨仓真实进程集成通过、安装包实机验收通过。后一项不能由前一项推断。历史原生 Windows 的具体阶段结果保留在 [源码审计](codex-source-audit.md) 和 [历史记录](implementation-history.md)；本次重新查询确认 run 34918708741 为 completed/failure，没有重新执行 Windows 安装或测试。

## 已经开发了什么

| 能力 | 已有实现与代码入口 | 目前能说明什么 |
| --- | --- | --- |
| 桌面策略装配 | [clientopts/desktop_sandbox.go](../../../internal/runtime/clientopts/desktop_sandbox.go)、[config.go](../../../internal/config/config.go) | 显式 rollout 开关默认 false；仅 Desktop macOS/Windows；restricted 模式强制 nxs 协商；Skill 读目录和挂载写目录由宿主给出 |
| 不支持时拒绝 | SDK 的 RequireSandbox、Bridge required_sandbox_v1 | 旧 runtime、Claude 和不可用后端不能静默降为直接执行；能力握手不证明 Windows 后端已完成 |
| 三种审批模式 | default / auto / bypassPermissions；[sandbox_policy.go](../../../internal/runtime/sandbox_policy.go) | 审批方式和 OS 执行边界已分开；Full Access 不授予管理员或越过领域权限 |
| 精确越界审批 | [sandbox_approval_test.go](../../../internal/runtime/permission/sandbox_approval_test.go)、SDK executor/sandbox_permission.go | sandbox_escape 与普通工具批准分开；只能本次批准；拒绝永久规则、输入偷换和迟到批准；固定审批时 cwd |
| 网络审批 | SDK executor/sandbox_network_approval.go、sandboxexec/network_lifecycle.go | exact tool/命令 epoch/host/port 绑定待连接；批准恢复连接；代理关闭取消回调并拒绝迟到 allow |
| 自动审核 | 现有 auto_review_v1 与 permission pipeline | 复用独立 reviewer 和人工回退；沙箱审批不是持久授权；既有人类专属审批仍保留 |
| 模式切换 | [runtime 替换测试](../../../internal/runtime/sandbox_replacement_test.go)、[Room 切换](../../../internal/service/room/realtime/sandbox_policy.go) | 跨 Full Access 退休旧 runtime；Room 取消精确 slot/批准；清理错误显式返回；不重发用户请求 |
| macOS 命令 | SDK sandboxexec/seatbelt.go、executor/sandbox_darwin_integration_test.go | 已有真实文件拒绝、子进程继承、直连拒绝、批准后连接与后台连接测试 |
| macOS 文件数据面 | SDK executor/file_sandbox.go、file/sandboxfs、environment/filesystem/worker | Read/Write/Edit、状态回调、PDF/Git 辅助命令与动态指令导入已有实现；新版 SDK 才具备，不能由旧握手推断 |
| Windows 组件 | SDK sandboxexec/windows_* | 已有 token、Job、private desktop、路径句柄、环境、stdio 白名单、命名管道认证/取消与探针；尚未组成可启用后端 |
| 支持检测 | [nxsruntime/service.go](../../../internal/service/nxsruntime/service.go)、runtime 设置页 | 显式查询 nxs --sandbox-status；区分未知、不支持、依赖缺失、依赖齐全；不改变权限，不代表当前任务已受限 |
| Linux/外部能力 | 现有 owner launcher、confinedfs、Connector/MCP 授权 | 继续各自拥有边界；桌面开关没有替代这些机制 |

### 文件工具进展需要特别纠正

旧计划中的“文件工具完全没开始”已不准确。c8245262 已包含每次文件操作启动独立 Seatbelt helper 的接口，辅助 PDF/Git 也通过同一边界执行；动态 AGENTS/CLAUDE 指令导入已有受限读取。后续 Skill 发现与配置装载仍存在未提交工作，本次不据此宣布完成。

但是 required_sandbox_v1 没有区分“只有命令隔离”和“文件工具也受限”的版本。桌面打包仍可以使用 nxs-stable 或外部指定的二进制，因此**SDK HEAD 的实现不能直接变成所有 Nexus 安装包的覆盖声明**。

另一个容易误读的点：当前 macOS profile 先允许一般文件读取，再叠加 DenyRead；AllowRead 并不意味着其他目录都不可读。工作区写限制和文件保密读取是两种不同承诺。需要逐项说明私钥、宿主配置、Skill、系统运行库和临时目录的实际规则，不能把“沙箱开启”解释成“只能看项目文件”。

## 目前的问题

| 编号 | 问题与影响 | 处理方向 |
| --- | --- | --- |
| D01 | Windows 完整限制 token 能阻断已测 runner 控制访问，但 PowerShell/DLL 与正常后代仍失败 | 先完成固定启动组合的对照实验，再选平台实现 |
| D02 | 参考 token 组合恢复 PowerShell/后代后，同账号 runner 线程拒绝测试失败 | 不替换生产工厂；完整验证 host、runner、command 三者的控制对象边界 |
| D03 | 分离账号测试虽通过部分 host 访问拒绝，PowerShell 仍报 0xc0000142 | 定位 LogonW/WithToken、登录会话、window station、desktop、profile/环境差异；不是继续增加任意 SID |
| D04 | 命令、文件、helper、Skill 与启动时读取没有独立能力和覆盖证明 | 建立分项协商与覆盖清单；旧二进制只能承诺实际具备的能力 |
| D05 | 读取默认范围、兼容临时目录、元数据保护与额外挂载仍缺一份完整资源模型 | 明确 allow/deny 优先级、资源来源、路径别名与撤销；新策略不能偷偷扩大现有权限 |
| D06 | 当前有取消和不重放保护，但没有覆盖所有命令/后台任务的持久执行效果记录 | 分开进程状态与副作用状态；unknown 只能对账，不自动重跑 |
| D07 | runtime 主进程退出不等于所有后代、代理和后台连接都结束 | 增加进程树/Job 空确认及租约回收证据；未确认前不展示“已全部切换” |
| D08 | 依赖检查没有表达当前会话是否生效、覆盖面和失败阶段 | 先建立可信能力/状态合同，再投影设置与 Composer 的简明状态 |
| D09 | Bridge 伪版本尚未发布，普通工作区可掩盖依赖分发问题 | 验证干净机器可取得的 SDK/Bridge/Nexus 组合；记录 checksum 与包内实际 binary |
| D10 | Windows 安装/修复/升级/卸载与受保护机器状态仍是设计 | 分离资源设置权限与命令执行权限，保持普通 App 安装方式 |
| D11 | 旧 README 混合 1,300 多行计划与逐次实验，新 SDK 进展没有同步 | 历史冻结、当前规范唯一、开发计划统一维护状态 |

其中 D01–D07、D09–D10 是默认启用/发布的门槛。D08 不能先把依赖可用包装成安全完成；D11 的文档整理也不能代替这些工程验收。

## 相比 Codex 还缺什么

| 主题 | 已核实的 Codex 参考 | Nexus 差距或应保留的差异 |
| --- | --- | --- |
| 默认产品体验 | 默认权限自动应用沙箱；进程和开发工具继承限制。[官方 Sandbox](https://learn.chatgpt.com/docs/sandboxing) | 当前仍是宿主环境开关，没有经过安装包验收的默认受限体验 |
| 策略表达 | beta permission profiles 将文件和网络规则组合，可选只读、工作区写入与完全访问；保留受保护路径。[官方 Permissions](https://learn.chatgpt.com/docs/permissions) | 审批模式已具备，但缺统一的资源 profile、只读执行模式、精确目录授权与已生效状态；不能把 /plan 当只读隔离 |
| 自动审核 | 独立 reviewer 处理越界请求，保留沙箱；支持原因、拒绝与超时区分及拒绝循环中断。[官方 Auto-review](https://learn.chatgpt.com/docs/sandboxing/auto-review) | 已有独立审核；仍需核对拒绝恢复、重复越界抑制、人工覆盖的 exact action 边界与所有入口一致性 |
| 网络 | 是否准许联网、是否启动代理、代理的目的地规则是不同配置；域名规则本身不能阻止直连。[官方审批与安全](https://learn.chatgpt.com/docs/agent-approvals-security) | 已有 macOS 直连拒绝和连接审批；还缺完整 IPv4/IPv6、UDP/DNS、loopback/private 地址与平台组合验收 |
| 原生 Windows | 较强 dedicated-user 实现与较弱 same-user fallback 明确区分，并有 setup、private desktop、管理策略与诊断。[官方 Windows sandbox](https://learn.chatgpt.com/docs/windows/windows-sandbox) | 仍停留在组件与失败实验；完整 backend、设置、修复及发布缺失。Nexus 暂不承诺弱 fallback，更不能静默降级 |
| 服务职责 | 固定源码的 service 请求为 RegisterInstallation / ProvisionSandbox；执行经专用账号 runner。[源码审计](codex-source-audit.md) | 旧“每条命令都走高权限 broker”不是可直接照搬的 Codex 架构，已退回候选 |
| 文件覆盖 | 官方说明命令与内置文件操作都参与沙箱边界；具体 platform/profile 限制仍须核对。[官方 Sandbox](https://learn.chatgpt.com/docs/sandboxing) | SDK 文件 helper 是重要进展；剩余宿主读取、能力协商、长文件/错误语义和打包需收口 |
| 故障与授权 | 审批和技术隔离分离，Full Access 也不等于管理员 | 保留 Nexus 更明确的 exact owner/session/round 与 unknown 不重放要求；无需复制 Codex CLI 名称和所有重试分支 |

这个对照不表示所有 Codex 特性都应复制。Nexus 的 Room、后台任务、多人 owner、领域审批和 runtime 替换需要自己的验收，不能用单用户 CLI 的成功代替。

## Windows 当前证据的正确读法

| 对照 | 历史原生结果 | 尚未证明 |
| --- | --- | --- |
| 完整 restricting-SID token | runner thread/token 拒绝通过；PowerShell/系统 DLL 与正常后代失败 | 可用 Windows 开发环境 |
| 参考 token（当前 Nexus fixture） | PowerShell、Go 后代、事件成功；runner thread 权限拒绝失败 | runner 控制面隔离，更不是已证实的 Codex App 漏洞 |
| 显式 LogonW bootstrap | 兼容用例通过，runner_thread_boundary 仍失败 | 完整复现官方安装、desktop、IPC 与资源配置组合 |
| 不同账号 + 参考 token | host process/thread 拒绝阶段通过；PowerShell 初始化失败；未到后代步骤 | 可直接上线的 broker 路径 |

详细提交与原生运行链接见 [后续测试记录](codex-source-audit.md#后续测试实现记录)。不得删除原失败断言，或仅把拒绝改成“允许也算通过”来推进平台状态。

## 本次验证记录

本次新执行的命令、退出码、能力范围与剩余门禁记录在 [验收矩阵](../../testing/desktop-sandbox-acceptance.md)。复测使用独立 SDK 提交导出、明确的本地 helper 和 GOWORK=off 的 Nexus 依赖。未执行 Windows 系统设置，未做签名安装包验收。

新代码加入后，应更新开发计划与当前规范；本文保留为本次重新规划的基线，不追加逐次实施流水。
