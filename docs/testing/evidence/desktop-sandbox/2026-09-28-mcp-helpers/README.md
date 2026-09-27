# macOS MCP 认证 helper 与辅助输出限额验收（2026-09-28）

Nexus/nxs 按整包发布；本批恢复已有认证 helper 配置在 macOS 受限会话中的使用，
不增加用户单独升级内核的步骤。当前合同见[沙箱规范](../../../../specs/desktop-sandbox-spec.md)。

## 固定来源

| 项目 | 验收来源 |
| --- | --- |
| Nexus | `2a0fdb895fa99c8e4c3233f8809cfa679043ad53`（产品接入 `9b2d06add`，门禁整组时限 `2a0fdb895`） |
| SDK | `5dc1eb8bee34741843a39f327a229adbcc3b99e8`（helper `121514c2`，文件辅助输出修复 `5dc1eb8b`） |
| Bridge | 远程规范模块 `v0.1.34-0.20260927173548-32d41b77af4a`，无本地 replace |
| Bridge checksum | `h1:2mhVPBsFus9PBTKUOEY/pS0/Jo+xzp4X31JcnOnCq6o=` |
| nxs SHA-256 | `115dc72e60036fc041ef4ea98eb5fa4af3d9efcba3dcc9498f89012f4d1f4f32` |
| 平台 | 本机 macOS arm64，详情见报告 |

验收启动时 Nexus 与 SDK 源码工作树均干净。nxs 由固定 SDK Git 归档构建；
归档证据作为后续文档提交，不改变已验证的生产代码。

## 结果

- [最终基线](baseline-report.json)：**44 项检查、595 个必测名称全部通过**，无必测 skip 或缺失。
  [完整输出](baseline-logs.tar.gz)包括 native helper、MCP 网络、指令、配置、写入恢复等全部门禁日志。
- 持久化 HTTP、Connector SSE 及各自的认证 helper 共四条路径，经 Nexus options builder →
  固定 Bridge → 真实 nxs → 本机 MCP 服务 → 后续模型协议请求完整往返。
  helper 的动态认证覆盖夹具中的过期静态 header；普通工具网络授权未扩大。
- 原生 helper 覆盖工作区读取、禁止读取、只读拒写、scratch 写入、Provider 环境过滤、
  MCP 端点授权不外溢至 helper、输出超限、取消、权限变化、会话关闭及普通同组后代清理。
  错误认证输出不发出 HTTP 请求；认证每次请求重新获取。原生重连继续经过相同装配。
- 文件 worker/Git/PDF 等辅助进程的输出上限回归覆盖 `io.Copy` 快速路径。
  修复前，限额 16 字节却保留 1024 字节且未标记超限；修复后定向竞态检查通过。
- [配套自检](package-runtime-check.json)：以源代码构建的 sidecar 和上述 nxs 完成
  workspace-write、read-only、full-access 三种权限配置握手与关闭。
- SDK `client`、MCP、executor、nxs 目标包，SDK MCP/helper 定向竞态，Bridge
  `client`/`protocol` 竞态，Nexus runtime/clientopts 目标包与定向竞态、架构依赖检查通过。
  不据此宣称整个 SDK runtime 包竞态或全产品测试通过。

## 保留的失败与修正

首次基线见[失败报告](initial-baseline-report.json)与[超时日志](initial-context-timeout.jsonl.gz)。
`TestDarwinSandboxStartupInstructions` 的前 10 个子用例依次通过，每个耗时约 8–14 秒；
进入第 11 个用例约 6 秒时触发整组 2 分钟上限，没有得到后续用例证据。

同一时段在上一轮固定 SDK `b487ef24` 归档中运行 `project/allowed=false` 与
`include/allowed=false` 对照，两例通过，合计 20.605 秒。门禁因此按必测名称数量
分配整组预算（至少 120 秒，每个必测名称 15 秒，外层额外 60 秒用于收尾），保持
生产操作截止时间及所有断言、必测 skip/缺失拒绝规则。最终同组 16 个子用例全部通过，
总耗时 51.58 秒。首次失败没有改写成通过。

## 复现

```sh
GOWORK=off node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /absolute/path/to/nexus-agent-sdk-go \
  --sdk-ref 5dc1eb8bee34741843a39f327a229adbcc3b99e8
```

原始最终报告目录为 `/var/folders/jk/9xhnwgrx6cj4fj76wffmxy0w0000gn/T/nexus-sandbox-baseline-Czc6GC`。
日志归档 SHA-256：`db5059c177f74e4323c68a9a95a82c897b89fc4a37f4c9725e0f90ecdda63bb8`。

## 证据边界

模型与 MCP 使用本机确定性协议夹具，不是实际外部 Provider/MCP 或 App UI。
配套自检使用源码 sidecar，没有为本批重新构建或签名 App/DMG。需要联网获取凭据的
helper 仍须有相应命令网络授权；MCP 端点授权不能代替它。同组清理不证明独立 `setsid`
后代终态。stdio MCP、完整后台/秘密/Provider 边界、DM/Room/后台 App 场景、正式签名/
公证及安装升级仍见统一计划；未运行 Windows 验证，`releaseAccepted=false`，Goal 保持进行中。
