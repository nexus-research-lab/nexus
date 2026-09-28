# macOS 进程集合组件验收

2026-09-28，Bridge `27f2a4f3ccd491d0613783bb42ad03f0817b6948`，macOS 27.0 arm64。`productionIntegrated=false`，`releaseAccepted=false`。

- [原生测试](native.jsonl)：独立 launchd job 执行前 Capture；登记写入测试目录后才放行；后代另建 session，忽略 SIGTERM。错误 PID version 被拒绝，bootout 成功后后代仍存活，恢复登记后 Reap 等待内核回收，独立对照身份保持。
- [竞态测试](race.log)：错误身份、缺能力、空枚举、信号/观察失败、旧 token 重扫、boot 变化及并发取消等待者。
- [无 cgo](no-cgo.log)：Capture/Restore 明确拒绝原生能力缺失，没有 PID 扫描回退。只在本机运行，没有 Windows 编译或验证。
- `GOWORK=off go vet ./internal/processscope` 与 `git diff --check` 通过。
- [成功 bootout 的反例](bootout-persistent.json)：持久 job 卸载返回成功，后代仍活着，只有精确终止后原集合才被回收。对应 [驱动](probe.py) 和 [原生程序](probe.c) 保留；记录中的 PID 均为已结束夹具，不得据此向当前进程发信号。

Bridge 目录重放组件测试：

```sh
GOWORK=off go test -race -count=1 ./internal/processscope
CGO_ENABLED=0 GOWORK=off go test -count=1 ./internal/processscope
NEXUS_NATIVE_SCOPE_TEST=1 GOWORK=off go test -json -count=1 -timeout=45s ./internal/processscope -run '^TestMacOSScopeReapsDetached$'
```

这只验收独立内部组件。测试登记文件不是生产认证持久层；尚无正式 helper/IPC、执行前宿主登记、transport 接线、崩溃恢复或 scratch 回收。初始 macOS 14.0 缺少精确信号 API，Intel/签名包/干净机器也未验证。Nexus 固定 Bridge 依赖未改，因为该组件尚未进入运行路径。
