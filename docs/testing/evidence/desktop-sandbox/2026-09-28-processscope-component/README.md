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

## 控制连接身份登记

Bridge `af3031b5850f96dd3433ed690fbb0427884739e7` 补充 `CapturePeer`：由 Unix 连接的 `LOCAL_PEERTOKEN` 获取内核 audit identity，与可信 launcher 指定 PID 的当前完整 audit token 比较，拒绝 PID 复用、身份变化、错误预期 PID 和已关闭连接。[原生测试](peer-native.jsonl)把文件放行改为 Unix 控制连接：登记前引导进程等待，登记写入后才放行测试后代，随后继续通过精确清理、恢复及对照存活断言。[竞态结果](peer-race.log)与[无 cgo 结果](peer-no-cgo.log)通过，目标包 vet 通过。

`expectedPID` 的可信来源仍由调用方负责；测试使用独立夹具自报 PID 作断言，不等于生产 job/可执行文件认证。该方法只验证控制连接与观察进程身份一致；正式 launcher、双向认证、持久执行登记、任务/凭据及文件描述符传输和产品接入仍未完成。
