# macOS 根进程内核退出观察

2026-09-28，Bridge `882df63ef230ccd96cfe6df3a7bce16c1fd2075e`，macOS 27.0 arm64。`productionIntegrated=false`、`releaseAccepted=false`。

[原生日志](native.jsonl)覆盖：

- `TestMacOSBootstrapExec`：helper 放行前按连接身份与登记集合前后核验，并注册 kqueue `NOTE_EXIT | NOTE_EXITSTATUS`；原地 exec 后从内核取得退出码 7，不再轮询 launchd 的最后退出码。提前取消一个等待者不影响后续结果；另一观察者重复 Close 后只返回 `ErrObservationStopped`，不终止任务或伪造退出。
- `TestMacOSScopeReapsDetached`：内核已确认主进程正常退出，但脱离后代仍可被观察；之后必须撤销 job 并单独 Reap 原集合。主进程退出不是 scratch 可回收的依据。
- 原有真实 helper 的错误宿主与放行前断连子用例继续通过。

[竞态结果](race.txt)、[无 cgo 结果](no-cgo.txt)及目标包 vet 通过。普通单元测试额外覆盖正常退出、信号、停止/继续等非终态与非法 status；不把模拟 status 当作真实信号验收。kqueue 句柄在 Go ForkLock 内创建并设置 CLOEXEC，避免并发启动继承；本次没有独立的并发 fd 泄漏原生实验。

重放（Bridge 目录）：

```sh
GOWORK=off go vet ./internal/processscope ./internal/processbootstrap
GOWORK=off go test -race -count=1 ./internal/processscope ./internal/processbootstrap
CGO_ENABLED=0 GOWORK=off go test -count=1 ./internal/processscope ./internal/processbootstrap
NEXUS_NATIVE_SCOPE_TEST=1 GOWORK=off go test -json -count=1 -timeout=60s ./internal/processscope ./internal/processbootstrap -run '^TestMacOS(ScopeReapsDetached|BootstrapExec)$'
```

观察者只保存当前进程内事实，不能恢复丢失的历史退出码，也不拥有 job 撤销、持久 owner/session/generation 回执或 scratch 生命周期。宿主生产 launcher、持久放行、默认 transport、unknown 恢复及发布包仍未接通；Nexus Bridge 固定依赖未改。没有 Windows、Intel、macOS 14.0、签名或 clean-host 验收。
