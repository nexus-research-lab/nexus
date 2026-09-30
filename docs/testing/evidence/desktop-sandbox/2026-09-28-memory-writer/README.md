# macOS 记忆写入租约验收

2026-09-28，macOS arm64 本机验证。**58 项检查、848 个必测名称全部通过**，无必测缺失或跳过；`releaseAccepted=false`。

## 固定来源

- Nexus：`5cbdec84a9e7c91b3b84f08521e9c80c458ca190`，门禁启动时工作树干净。
- SDK：`aa140386aa4a2fef0627026a10681e95164002ec`，从固定 Git 归档构建，门禁启动时工作树干净。
- Bridge：`v0.1.34-0.20260927182153-4b2972f09143`，checksum `h1:61JYZ5QUkuhaEb1Y0Bxe17ETd6CYolEQVQgZdb+bNRc=`；`GOWORK=off`，没有 replace。
- nxs SHA-256：`030815e71c4a7747328ca8a99c882f0f7fa28ef5e8685cea41e9f23a86b7addd`。

复核入口：

```sh
node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /absolute/nexus-agent-sdk-go \
  --sdk-ref aa140386aa4a2fef0627026a10681e95164002ec
```

## 行为与结果

| 范围 | 结果 |
| --- | --- |
| 修复前反例 | SDK `608f1b3b` 下拒绝取锁/旧记录读取/完成标记，以及锁/目录替换共 5 个场景失败，正常对照通过；原夹具保留 |
| 原生 runtime | 原 6 个场景全部通过，另补抽取允许、拒绝和取消 3 个场景；11 个父/子必测名称通过 |
| 原生 worker | 竞争、完成、释放、取消、父端 EOF、丢失回复、非零退出、多余响应、提前退出、取锁超时及旧记录存活/死亡兼容；14 个父/子名称在 race 下通过 |
| 相关包 | 相关包 race 共 595 个测试名称通过；普通运行中的原生 opt-in skip 不作为原生证据；vet 通过 |
| 固定集成 | 58 项/848 个必测名称通过，既有宿主、配置、文件、命令、媒体、Skill、MCP 与生命周期检查保留 |

macOS 写入者使用同文件策略的内部 worker，持有目录 fd 与稳定 flock。完成只尝试一次并核对终态及退出；释放只关闭本次句柄，不删路径。目录或锁换代拒绝完成，替换文件保持；取消或释放失败不报保存成功或推进抽取游标。旧记忆布局、完成标记语义及 `.consolidation-active` 记录保留，旧持有者明确死亡后才继续。无 Bridge 协议扩张；Nexus/nxs 配套发布。

## 保留的失败与边界

初版 worker 在实际启动前过早清理沙箱 profile，原生拒绝退出；修复为等待 worker 退出后清理，失败与随后通过日志均保留。另一次原生 runtime race 运行的 9 个场景均在初始化阶段触发既有 30 秒截止，未到达租约断言；每个 race helper 的退出等待累积是该构建的限制。普通构建原生回归、独立 worker race 和相关包 race 通过，未放宽生产截止，不把原生 runtime race 记为通过。

本证据只覆盖记忆控制文件和已知内部 worker；不证明任意脱离 session 后代监督、一般 `cleanup_unknown` 恢复、未受限同 UID 篡改防护、其余 transcript/SDK IO、Provider 网络或秘密/句柄隔离已闭合。未知临时文件保留，不按路径补偿；busy 可能已创建稳定 guard，不表示零文件副作用。其他平台保持既有路径，未运行 Windows 验证。

实际 App 仍使用 [此前固定版本的证据](../2026-09-28-app-contracts/README.md)，未重标为本 SDK。Mac 锁屏与签名 CI 缺少私有 SDK 只读凭据仍阻塞完整 UI/分发验证；本次没有启动新的 QA App、发送真实模型请求或完成签名/公证/干净机器安装。

## 归档

- [固定报告](baseline-report.json)
- [固定基线原始日志](baseline-logs.tar.gz)
- [修复前后与失败日志](focused-logs.tar.gz)
- [来源与归档/条目 SHA-256](manifest.json)

归档不包含数据库、二进制、源码导出或凭据；每个归档及条目哈希已重新核验。
