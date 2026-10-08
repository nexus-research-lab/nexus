# 原进程与 warm 策略回执关联

本批在 Nexus `b0b8dd89e` 基础上实现；精确源码版本为包含本文件的提交。Bridge 固定 `c994b197e010`，nxs 为 SDK `2d1fd0d6` 构建。

## 已验证范围

- Manager client 创建时冻结原进程代次；三次 warm startup 的策略代次分别为 1/2/3，始终关联同一原 runtime launch。新 client 替换另建原进程代次。
- 数据库保存原进程 generation + launch ID，不使用 latest 查询推断关联。跨 owner/session、lease/runtime 不符、未来进程代次、探测或未放行记录拒绝。
- 已绑定策略拒绝改绑或撤销关联；历史无关联记录拒绝事后补写推测身份。缺少原 runtime 时 Connect 不确认策略。
- SQLite 迁移保留历史 unknown 原因与资源身份；有绑定数据时拒绝丢失身份事实的降级。PostgreSQL SQL 仅审查，未进行 PostgreSQL 运行验收。
- macOS arm64 真实 helper + 固定 nxs 的 AutoDream 连续两次独立生命周期：policy exact key 等于实际回收进程 key，策略 retired，scratch 删除。AutoDream consolidation 关闭，没有真实模型请求。

## 复验

所有 Go 命令显式 `GOWORK=off`。

```sh
GOWORK=off go test -race -count=1 ./internal/storage/sandbox ./internal/runtime
GOWORK=off go test -race -count=1 -v ./internal/runtime -run '^TestSupervisedReceipt'
NEXUS_SANDBOX_TEST_BINARY=<fixed-nxs> NEXUS_SUPERVISION_TEST_HELPER=<signed-helper> GOWORK=off go test -race -count=1 -v ./internal/app -run '^TestAppManagedAutoDreamSupervisedNative$'
GOWORK=off make check-architecture
```

固定二进制 SHA256：

- nxs: `6622c107941409c480a8fbb0a517edfde41f542cb450a365f0e37d047ebdd6aa`
- helper: `7e21e16effd3c00c0d2805d8c09cc850470cdf6b11d37eb5677c5f1dc31735a8`

## 边界

这是恢复所需身份关联，尚未自动清除 policy/scratch unknown；旧无监督证据不被视为已退出。App 默认监督/启动恢复、真实模型 UI 全链路、macOS 14.0/Intel 与正式发布验收仍未完成。不能用本批结果代替整体交付。
