# 显式 macOS 监督启动器验收

2026-09-28，Bridge `95b9616f8770ad2f31d1701b31aa80e17306dead`，macOS 27.0 arm64。`productionTransportIntegrated=false`、`releaseAccepted=false`。Nexus 仍未升级 Bridge 固定依赖或默认接入此入口。

## 本批次

Bridge 公共 `supervision` 入口已串联固定 helper 摘要核验、宿主 Reserve/Publish、launchd 启动、连接内核身份、主进程退出观察、Register、一次性 ClaimRelease 和单次任务/标准管道发送。主进程退出主动触发共享清理，不依赖消费者先读完管道；清理必须撤销 job、回收原集合，再把 exact evidence 交回 Host.Finish。回调必须由可信宿主实现且持久提交后返回，不能直接暴露给模型。

[真实原生竞态日志](native-race.jsonl)必测主用例及 8 个子用例全部通过：

- success：真实 helper 及 job 完成标准流、显式环境和退出码 7；回调顺序为 reserve/publish/register/release/finish。
- detached_output：任务后代另建 session、忽略 TERM 并持有输出管道，主进程先退出；消费者在调用 Wait 之前即取得 EOF，随后确认集合回收及终态。
- cancel_close_waiter：取消一个 Close 等待者后，另一等待者仍取得共享清理完成及回收事实。
- reserve/publish/register/release：在各阶段注入失败；register 和 release 包含先记录阶段再返回错误。任务标记未产生，无重复放行，收尾按原身份保留或终结记录。
- finish：进程已经回收但终态写入失败，返回错误并保留 released；重复 Close 保留同一错误。

[普通竞态检查](unit-race.txt)与[无 cgo 检查](no-cgo.txt)通过；目标包 vet 通过。额外验证已取消的 Start 不调用 Reserve，以及 io.Copy 优化不会绕过系统命令输出上限。未运行 Windows 验证或交叉编译。

```sh
GOWORK=off go vet ./supervision ./internal/processscope ./internal/processbootstrap
GOWORK=off go test -race -count=1 ./supervision ./internal/processscope ./internal/processbootstrap
CGO_ENABLED=0 GOWORK=off go test -count=1 ./supervision ./internal/processscope ./internal/processbootstrap
NEXUS_NATIVE_SCOPE_TEST=1 GOWORK=off go test -race -json -count=1 -timeout=90s ./supervision -run '^TestMacOSSupervisedStart$'
```

## 仍需产品接入

原生用例使用独立 Host 文件夹具，不是 Nexus 数据库适配，也不是 App 或模型端到端验收。生产 Host 必须把 launch ID 固定到现有 owner/session/generation，使用任务不可写的宿主路径及 confinedfs 发布 job/收尾文件；本包不拥有用户 scratch。固定 helper 摘要不能代替签名分发或抵御任意未受限同 UID 程序。

默认 client transport、Nexus 持久写入适配、重启恢复、单轮中断边界、Full Access/后端切换与正式打包尚未接通。Unix socket 当前限 103 字节，超长状态根的路径分配仍待处理，不能退到不受保护路径；macOS 14.0 原生 API、Intel、签名与 clean-host 也未完成。全部临时 job 和本次进程已按用例收尾，不操作已有 App、历史 PID 或用户 unknown 记录。
