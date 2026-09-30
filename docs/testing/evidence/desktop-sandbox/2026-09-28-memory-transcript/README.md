# macOS 后台会话记录读取验收

2026-09-28，macOS arm64 本机验证。**61 项检查、888 个必测名称全部通过**，无必测缺失或跳过；`releaseAccepted=false`。

## 固定来源

- Nexus：`fc888db41daae0b56a763c438af7e39a0ee57c51`，门禁启动时工作树干净。
- SDK：`2d1fd0d6b2d0d7d1a6dedbad071533132dde61d9`，从固定 Git 归档构建，门禁启动时工作树干净。
- Bridge：`v0.1.34-0.20260927182153-4b2972f09143`，checksum `h1:61JYZ5QUkuhaEb1Y0Bxe17ETd6CYolEQVQgZdb+bNRc=`；`GOWORK=off`，没有 replace。
- nxs SHA-256：`6622c107941409c480a8fbb0a517edfde41f542cb450a365f0e37d047ebdd6aa`。

复核入口：

```sh
node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /absolute/nexus-agent-sdk-go \
  --sdk-ref 2d1fd0d6b2d0d7d1a6dedbad071533132dde61d9
```

## 行为与结果

| 范围 | 结果 |
| --- | --- |
| 修复前反例 | SDK `aa140386` 下直接拒绝与链接目标拒绝两种场景仍读出了受限内容；允许对照通过。原始失败、夹具与 overlay 映射保留 |
| 原生 runtime | 18 个父/子名称通过：直接/链接拒绝、允许读取，Summary/AutoMemory/AutoDream 三入口各自拒绝/允许/取消，以及当前 transcript 不存在 |
| 原生流式传输 | 4 个父/子名称在 race 下通过：允许完整读取，以及已返回数据后 helper 非零退出或最终响应截断；失败时不发布部分记录 |
| 流式会话语义 | 最终 18 个父/子名称在 race 下通过：小日志、大日志、保留段、禁用 precompact 跳过、取消、迟到错误、缺失端口和协议长度/限额 |
| 相关包 | session、worker、sandboxfs、executor、runtime 和 cmd/nxs 的 race 运行共 633 个测试名称通过；移除未使用 wrapper 后 session 最终 race 24 个名称通过；vet 通过 |
| 固定集成 | 61 项/888 个必测名称通过，既有宿主、配置、文件、命令、媒体、Skill、MCP 与生命周期检查保留 |

Summary、AutoMemory 与 AutoDream 只沿当前 recorder transcript 路径，经当前文件执行器读取内容替换记录。准备和读取共享 30 秒截止；显式端口缺失、拒绝、取消或不完整读取均停止该后台操作，不调用模型。只有确认当前文件不存在时按空记录处理，不再回退宿主 catalog、历史目录或 Git worktree 扫描。

文件 transport 按帧传给消费者，完整终态、EOF 和辅助进程成功退出后才允许发布结果。会话层保留原有 5 MiB 阈值、最后一个未保留 compact 后缀、metadata 和显式禁用跳过语义；有效后缀没有新增硬大小上限，不能据此宣称任意日志都有固定内存上限。内部流式端口未改变 wire 格式，也没有扩张 Bridge 能力；Nexus/nxs 继续配套发布。

## 证据边界

修复前夹具对应旧函数签名，overlay 文件保留原开发机路径；在旧提交复现时需映射为当地路径，不能原样套到当前函数。初始单元运行和最终运行都归档，最终版本还覆盖删除未使用 wrapper 后的实际共享加载路径。普通包 race 的 opt-in 原生 skip 不作为原生验收；原生 runtime 采用普通构建，流式 helper 单独运行 race。

本项关闭后台内容替换记录的宿主读取旁路；普通会话录制/恢复、其余 SDK 辅助 IO、任意脱离 session 后代监督、一般 `cleanup_unknown` 恢复、Provider 网络和秘密/句柄隔离继续独立收口。没有运行 Windows 验证。

实际 App 仍使用[此前固定来源的证据](../2026-09-28-app-contracts/README.md)，不重标为本 SDK。完整窗口复验与签名 CI 私有 SDK 读取授权仍待原有外部条件；本次没有启动新的 QA App、发送真实模型请求或完成签名/公证/干净机器安装。

## 归档

- [固定报告](baseline-report.json)
- [固定基线原始日志](baseline-logs.tar.gz)
- [修复前后与聚焦日志](focused-logs.tar.gz)
- [来源与归档/条目 SHA-256](manifest.json)

归档不包含数据库、二进制、完整源码导出或凭据；保留独立复现夹具，每个归档及条目哈希已重新核验。
