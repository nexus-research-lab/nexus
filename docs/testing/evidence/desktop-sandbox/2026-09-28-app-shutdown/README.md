# macOS App 正常退出与同会话重启（2026-09-28）

## 固定来源

- 修复提交 Nexus `132b5ad69826ecde11cab769fdeda03b7436c257`；SDK `b84b7b6c7c3970d4a948501eaed2874563730cc2`，规范 Bridge `v0.1.34-0.20260927182153-4b2972f09143`。
- 完整基线开始时 Nexus/SDK 均干净：[46 项检查、661 个必测名称全部通过](baseline-report.json)，无必测缺失或跳过。原始 92 份输出见 [baseline-logs.tar.gz](baseline-logs.tar.gz)。
- macOS arm64 / Darwin 27.0.0；独立 bundle ID `com.nexus.sandbox-shutdown-qa`，开发签名 App；完整构建、签名完整性和[随包 runtime 自检](bundled-runtime-check.json)通过。
- 二进制摘要、构建路径、日志摘要及证据级别见 [build-and-evidence.json](build-and-evidence.json)。本次没有生成新的正式 DMG，没有 Developer ID/公证结论。
- Provider 仅来自已授权 `.env` 的相关字段，测试 App 保存独立配置；实际模型为服务端目录中的 `kimi-for-coding`（UI 名称 K2.8 Preview），不是 `.env` 的旧别名 K2.6-code-preview。无需官方 Claude 登录。

## 发现并修复的问题

先在 Nexus `278ce0bc2` 的实际 App 界面正常退出并重开，复现同一会话启动失败：
`previous sandbox runtime cleanup is unresolved`。App、sidecar 和 nxs 虽然退出了，
宿主关闭服务却没有调用 Manager 的 Session 清理，最新回执仍为 `confirmed`，见
[修复前退出记录](ui-quit-evidence.json)。新加入的真实 SQLite AppServices 回归在旧实现也失败，
错误为 `phase=confirmed ... disconnected=false`。

修复将 Manager 退出纳入 AppServices 生命周期：先拒绝新启动、round 与后台任务，取消现有任务，
等待在途启动和回执写入，再并行关闭 Session，最后关闭数据库。调用者超时只停止等待，
数据库继续供原清理流程使用；重复调用等待同一次结果。已有未知状态不会因此自动解锁。

## 验证结果与实际入口

| 场景 | 入口与结果 |
| --- | --- |
| DM 原生 Write、Bash 读取 | 修复前独立 App 的实际界面操作；文件内容为 `MACOS_UI_QA_20260928_A`，实际文件核对通过 |
| 拒绝交付与本次批准 | 实际 App 的审批卡；第一次拒绝显示 User denied permission，第二次本次批准产生文件卡片；实际 runtime 权限是 Agent 继承的 `auto`，不能把菜单焦点标记当成 `default` 生效证据 |
| UI 停止命令 | App 停止按钮中断 `sleep 120 && printf ...`；精确测试 shell/sleep PID 均消失，后续 sentinel 文件未创建，见[停止记录](ui-stop-evidence.json) |
| 真实 nxs 连续关闭与数据库重开 | 新的必测用例通过产品 options builder、真实 Bridge/nxs 与 host scratch lease，连续两次连接、AppServices.Close、数据库重开；代次递增、回执 retired、scratch 删除 |
| 修复版 App 的真实 DM 重启 | 实际原生 App 加 WebSocket 客户端，在同一独立测试数据库中创建新的验收会话；Read 成功后用 SIGTERM 进入 App 的 `NSApp.terminate` 正常退出链，旧 App/sidecar/nxs/log 进程退出，回执 retired；重新启动 App 后同一会话第二次 Read 成功，SDK Session ID 保留、generation 1→2，见[重启记录](app-restart-evidence.json)及[重启后回执](app-restart-after-receipts.json) |
| 修复版 App 的 Room 基础工具 | 本地独立测试 Room，WebSocket 显式指定唯一 Sandbox QA Agent；原生 Write 创建 `sandbox-group-qa.txt`，Bash 读取得到 `MACOS_GROUP_QA_20260928`，round 成功 |
| DM 与 Room 一起退出 | 最后正常退出同一 App，两种 Session 的最新回执均 retired，两个 scratch 路径均不存在，测试进程全部退出，见[最终记录](app-final-shutdown-evidence.json) |
| 并发与数据库顺序 | runtime/AppServices 定向 race、目标包测试、增量 Go/vet 与架构门禁通过；覆盖迟到 factory、写入中的 confirmed 回执、调用者超时、错误保留和新任务拒绝 |

测试 App 使用独立状态根，修复版沿用这份测试数据库，以保留前一版生成的 Agent、Provider、
文件与会话；没有清除修复前失败会话的旧回执，没有把新建会话冒充该失败历史的自动恢复。
该记录不等于已发布历史 App 数据库的完整升级/回退验收。

源日志、前后 WebSocket 事件、测试客户端与退出循环脚本见 [app-acceptance-logs.tar.gz](app-acceptance-logs.tar.gz)。
归档前核对日志不包含本次 Provider token 或测试 desktop token；连接凭据文件和数据库未归档。

## 测试隔离说明与未完成项

本次 UI 测试准备阶段曾使用默认 bundle ID：界面工具在退出后自动重启该 App，因启动环境丢失而短暂
打开默认状态根；没有发送模型任务，随后停止。此后改用独立 bundle ID、固定状态目录指针和独立
preferences suite。这里的 UI 工具结果来自后续独立实例；不把准备阶段描述为完全没有访问默认数据。

执行修复版 UI 复验时 Mac 锁屏，界面工具无法继续。后续 DM 重启和 Room 场景通过真实 App 的
HTTP/WebSocket 入口完成，不计为菜单点击或审批卡的完整 UI 复验。新测试 App 最后已正常退出。

待完成：完整 App 的 Room/后台审批和权限/后端切换、任意脱离后代的监督与安全恢复、其余 SDK IO/
秘密/Provider 网络边界、真实外部 MCP、完整历史 App 数据迁移，以及正式签名/公证、Intel 与干净机器。
签名 CI 仍待私有 SDK 的只读授权。未运行 Windows；官方 Claude 账号/OAuth 暂缓。
`releaseAccepted=false`，Goal 保持 active。
