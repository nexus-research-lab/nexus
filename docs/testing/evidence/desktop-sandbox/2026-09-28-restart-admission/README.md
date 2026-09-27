# macOS 重启准入与固定基线（2026-09-28）

## 固定来源

- Nexus `278ce0bc286b2080c35e836ea24e167721c37297`，SDK `b84b7b6c7c3970d4a948501eaed2874563730cc2`；门禁开始时两个工作树均干净。
- Bridge `v0.1.34-0.20260927182153-4b2972f09143`，校验和 `h1:61JYZ5QUkuhaEb1Y0Bxe17ETd6CYolEQVQgZdb+bNRc=`；未使用本地模块替换。
- macOS arm64，Darwin `27.0.0`，Go `1.27.1`；SDK 固定归档构建 nxs，SHA-256 `39fc47dcdb551eb1b8c22e56490c24cf0e7cde1af7f939795cbeb099d87e0a4b`。
- [原始报告](baseline-report.json)：45 项检查、655 个必测名称全部通过，必测无跳过或缺失。
- [90 份 stdout/stderr 日志](baseline-logs.tar.gz)，SHA-256 `31c6207e1fa60b1757877dc0d1f2f84ede9e702bc2e0a4b4ba4892beeb0cb007`。

```sh
GOWORK=off node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox \
  --sdk-ref b84b7b6c
```

## 本批新增证明

1. 宿主重启后，新 runtime 在创建进程前读取当前 owner/session 的持久回执。未收口、读取失败、损坏或代次耗尽均拒绝启动；切换到 Claude 或 Full Access 不能绕过。
2. 正常结束的历史回执提供下一代次下界。真实 SQLite 关闭重开与 Manager 重建保留 `1 → 2 → 3` 及不可变历史，避免重启后重复使用 generation 1。
3. 真实子进程崩溃夹具留下的 `cleanup_unknown` marker 阻止相同 scope 重新 Acquire；只读/可写 scope 和历史 `-stale-` 路径都不能换目录绕过。其他 Session 可正常启动。
4. 并发 Acquire 串行发布 marker，16 个调用复用同一资源，避免把尚未写完的 marker 误判为崩溃。

上述回归在修复前复现失败、修复后通过；另运行 runtime 目标测试、定向 race、增量 Go/vet 与架构检查。

## 边界

本批保留已知清理失败的持久栅栏，不提供任意脱离后代进程的终态证明或通用安全解锁。
同一启动周期中的 active marker sweep 行为未在本批重写。正式签名、公证、Intel、干净机器、
完整 App 数据迁移及全部 UI 场景继续独立验收；`releaseAccepted=false`。未运行 Windows 验证。
