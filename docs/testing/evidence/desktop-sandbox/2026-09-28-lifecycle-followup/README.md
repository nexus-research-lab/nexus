# macOS 正常退出和终态后续恢复

2026-09-28，本机 macOS 27 arm64。Nexus 基线 6265aa187 加本批代码；Bridge 固定 c994b197e010，nxs 固定 2d1fd0d6。

## 已验证

- [contracts.log](contracts.log)：正常最终 Release 在启动未收口时拒绝删除；complete 提交后丢失响应保留句柄，重试读取完成事实。SQLite 终态资源和策略独立扫描与稳定分页通过。
- [batch.log](batch.log)：失败项保留且后续项推进；分页结束仍报告失败；取消与缺失所有权拒绝；失败解除后按原记录完成。
- [native-crash.log](native-crash.log)：独立宿主真实退出后，持新锁的宿主先回收原生集合，再经独立生命周期扫描删除原 scratch 并收口两份绑定策略，重复扫描为空。任务为 shell/sleep，策略回执为注入值。
- [autodream.log](autodream.log)：固定真实 nxs 经 App 管理的 AutoDream 服务正常关闭，保存 scratch complete，生命周期扫描与重复扫描通过。没有真实模型请求或 App UI 交互。
- [architecture.log](architecture.log)、[vet.log](vet.log)：架构检查、runtime 和 storage/sandbox vet 通过。

所有 Go 调用 GOWORK=off，race 测试 count=1。原生用例显式提供 NEXUS_SUPERVISION_TEST_HELPER；AutoDream 另提供 NEXUS_SANDBOX_TEST_BINARY。测试名见日志，未把环境缺失 skip 作为成功。

nxs SHA256: 6622c107941409c480a8fbb0a517edfde41f542cb450a365f0e37d047ebdd6aa

helper SHA256: 7e21e16effd3c00c0d2805d8c09cc850470cdf6b11d37eb5677c5f1dc31735a8

## 包级回归

[packages-initial.log](packages-initial.log)：runtime 全包 race 312.669s、confinedfs 和 protocol 通过；storage/sandbox 的旧降级测试硬编码期待 schema 145，但迁移 149 已先拒绝有记录的降级，因此该次整体失败。更新断言为保留当前 149，未更改生产回退规则。[storage-recheck.log](storage-recheck.log) 的 storage/sandbox 全包无缓存 race 重跑通过。新增批次故障测试单独见 batch.log。

命令：`GOWORK=off go test -race -count=1 ./internal/runtime ./internal/storage/sandbox ./internal/infra/confinedfs ./internal/protocol`；重跑为 `GOWORK=off go test -race -count=1 ./internal/storage/sandbox`。没有执行全仓 Go 测试或 Windows 验证。

## 未完成边界

App 默认启动尚未配置监督器、保护宿主根并调用两阶段恢复；本批不证明默认 App 隔离、macOS 14.0、Intel、签名安装包或干净机器验收。PostgreSQL 迁移只有 SQL 审查，未运行数据库实例。未知业务副作用和历史无关联策略不会被清除或重放。
