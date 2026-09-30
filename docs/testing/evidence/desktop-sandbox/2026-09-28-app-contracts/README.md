# macOS App 审批、切换与取消重连（2026-09-28）

## 固定来源

- 最新基线：Nexus `259ccda2de7c93777628377f70f05229157ef81f`、SDK `5a937a18355c94d8766458b705d4b9eef7bd8a23`、Bridge `v0.1.34-0.20260927182153-4b2972f09143`。开始时源码干净，[48 项检查、689 个必测名称](baseline-report.json)全部通过，无必测缺失或跳过；[96 份原始输出](baseline-logs.tar.gz)保留。
- 第一批 App 场景使用 Nexus `5c51baa09` 与同一 SDK/Bridge；对应[47 项/686 个必测名称基线](switch-baseline-report.json)和[原始输出](switch-baseline-logs.tar.gz)单独保留。
- macOS arm64 / Darwin 27.0.0；两次均为独立 bundle ID `com.nexus.sandbox-switch-qa` 的开发签名 App，完整构建、签名完整性及三种资源模式的包内自检通过：[第一批](switch-bundled-runtime-check.json)、[修复重连后](bundled-runtime-check.json)。
- 两个 App 使用同一份隔离测试数据库，继续已有 Agent、DM、Room、配置与 workspace。没有删除旧失败会话的未收口回执，没有访问主数据库。Provider 仅使用已授权的第三方模型字段，实际模型为 `kimi-for-coding`；Claude CLI 使用该网关，无官方 Claude 登录。
- 新 App 复用同一 SDK 固定提交的已验证 nxs 输入；本轮基线重新构建的 nxs 有独立摘要。App 内二进制、输入、构建来源和归档摘要见 [build-and-evidence.json](build-and-evidence.json)。

## 发现并修复的问题

**工作目录的 macOS 路径别名。** Claude 返回 nxs 后，模型使用配置根的 `/private/var/...`
真实路径，旧审批预检却只认识 `/var/...`，导致同一 workspace 的普通 Read 被额外拦住。
SDK `5a937a18` 将配置根与其真实拼写纳入同一审批检查，工具输入保持原样；显式 ask/deny
覆盖两种拼写，子目录链接、外部路径及隐藏写入仍不能借此免审批。原生文件与规则回归进入必测基线。

**取消审批后重连收不到事件。** 第一批 App 在等待网络审批时取消，终态 interrupted、
批准失效及文件未创建均正确；紧接着新连接发起 Read，服务端实际完成了 round，客户端却
一个事件也未收到，90 秒后连接超时。旧请求结束时用取消上下文向新 sender 广播，旧
`SendJSON` 会把调用取消也当作连接失效；定向回归复现取消写入和健康连接失效问题。
Nexus `259ccda2d` 在接触连接前拒绝已经取消的事件，已经接纳的帧以连接独立的 10 秒
超时完成，真实传输失败仍使连接失效。三个回归均进入基线，20 次 race、目标包、增量
Go/vet 和架构检查通过。

旧失败事件及后续只读探测保留在归档中。后续探测已恢复连接，但 Read 返回合法的
`file_unchanged` 去重结果，旧夹具要求再次返回全文而失败；最终复验改读每轮唯一的新文件，
没有把模型回复或历史内容当作本次实际读取证据。

## 实际 App 验证

本批全部通过原生 App 启动的 sidecar 的 HTTP/WebSocket 入口执行，**不属于 UI 点击验收**。

| 场景 | 实际结果与证据 |
| --- | --- |
| nxs → Claude → nxs | 同一宿主 DM 下，指定 `/private/var/...` 的原生 Read 全部返回精确内容，无多余审批；runtime kind、所需能力和 generation 分别核对。[切换记录](fixed-switch-evidence.json) |
| nxs 默认 → Full Access → 默认 | 真实外部目录写入依次拒绝、允许、拒绝。Full Access 的允许场景显式设置 `dangerouslyDisableSandbox=true` 并另行本次批准 `sandbox_escape`；普通 Bash 仍受沙箱限制，不能把 Full Access 描述成所有命令自动裸执行。[边界记录](boundary-switch-evidence.json) |
| Room 审批拒绝及本次批准 | 指定唯一测试 Agent，`mcp__nexus__deliver_files` 第一次拒绝产生工具错误；新的独立请求本次批准后产生成功交付回执。同一个 permission ID 的重复 transport 事件只回复一次。[Room 记录](room-approval-evidence.json) |
| 网络拒绝 → 本次允许 → 再次拒绝 | 三个独立 round 的 `curl https://example.com` 分别失败、返回 HTTP 200、再次请求审批并失败。目标固定 host/443，本次批准没有持久授权，也没有重放命令。[网络记录](network-approval-evidence.json) |
| 独立后台任务 | 两个始终 disabled、手动触发且不外投的任务调用文件交付：拒绝后同一次运行结束；allow_once 在同一个 run ID 中恢复并成功，attempts 1→2。[后台审批](background-approval-evidence.json)与[最终停用状态](background-final-state.json) |
| 取消待网络审批后立即重连 | 修复版 App 连续两轮：interrupt 精确 round，pending 权限变 cancelled，round 为 interrupted，迟到批准被拒绝，写入 sentinel 始终不存在。未等 interrupt ACK 或旧连接关闭即重连；下一条 Read 都返回每轮新生成的唯一内容，无审批。[取消重连记录](network-cancel-reconnect-evidence.json) |
| Claude 默认 → Full Access → 默认 | 修复版 App 的 Ruby 真正发起外部测试目录文件 IO：默认沙箱拒绝，Full Access 写入精确内容，返回默认再拒绝；没有用 shell 重定向预检替代实际文件 IO。generation 31→32→33，限制能力仅在两次默认模式声明。[Claude 记录](claude-access-evidence.json) |
| 最后返回 nxs | 同一 DM 再切回受限 nxs，Read 新文件成功，资源能力恢复，generation 为 34。[返回记录](nxs-final-return-evidence.json) |
| 最终 App 正常退出 | 两次均用 SIGTERM 进入 `NSApp.terminate`。DM、Room、两个后台任务的最新 receipt 全为 retired，对应 scratch 不存在，精确 App/sidecar/runtime/log 后代全部退出：[第一批](app-switch-shutdown-evidence.json)、[修复版](app-final-shutdown-evidence.json) |

切换保留的是宿主会话，不能据此承诺不同后端使用相同 SDK Session ID。审批测试逐个核对本轮
request/round/tool-use/result 身份；没有把 replay 的旧事件算作当前结果。

原始事件、App 日志、构建日志与客户端脚本见 [app-acceptance-logs.tar.gz](app-acceptance-logs.tar.gz)，
按两次 App 的源码提交分目录。归档前扫描了本次 desktop 与 Provider 凭据的实际值；没有归档
连接凭据、数据库或用户 preferences。最初缺少相邻 rg 的构建失败日志保留，完整成功构建另有记录。

## 交付边界

这一批闭合的是实际 App 的上述审批、切换、取消重连和正常退出场景。旧失败会话
`agent:94a0a7739fb9:ws:dm:e6a86831da0e` 的未收口记录仍保留，不能声称一般崩溃 unknown 已自动恢复。

仍待完成：任意 setsid/脱离后代的可部署监督与安全恢复；其余 SDK 辅助/后台 IO、秘密文件/
句柄和 Provider 网络边界；真实外部 MCP 服务；完整窗口操作、历史正式 App 数据库升级/回退；
Developer ID/公证、Intel 和干净机器安装。Mac 锁屏期间未用后端入口代替 UI 结论；签名 CI
仍待私有 SDK 的只读授权。Windows 由另一台机器核验，官方 Claude 账号/OAuth 暂缓。
`releaseAccepted=false`，Goal 保持 active。
