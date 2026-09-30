# macOS 原 scratch 目录身份登记

本批在 Nexus `0fc80ac20` 基础上实现；精确源码为包含本证据的提交。Bridge 固定 `c994b197e010`；nxs 为 SDK `2d1fd0d6` 构建。平台仅本机 macOS arm64。

## 覆盖

- 原 host lease 提供 parent/leaf 目录身份（device、inode、generation、birth time）。修改任务可写 marker 不影响这个来源。
- 替换 leaf 或 parent 后无法生成身份；探测结束后替换 scratch，后续 runtime 工厂拒绝启动。
- released lease 拒绝。合法证明持久保存，缺字段、路径穿越、无 lease、错误格式拒绝，历史空证明仍可读取。
- 注入的精确进程恢复流程保持原 scratch 身份；该测试只验证适配，不是 OS 回收验收。
- 真实 helper 的 nxs/Claude 各启动用途通过，nxs 探测和 runtime 均保存同一原资源身份；测试命令为受控 shell，不是 Claude 模型调用。
- 真实 helper + nxs 的两次独立 AutoDream 生命周期包含有效 scratch 证明，关联策略终态正确，目录删除。Consolidation 关闭，未发起模型请求。

## 复验

```sh
GOWORK=off go test -race -count=1 -v ./internal/runtime ./internal/storage/sandbox -run '^TestSupervisedScratch|^TestProcessScratch'
GOWORK=off go test -race -count=1 ./internal/runtime ./internal/storage/sandbox
NEXUS_SUPERVISION_TEST_HELPER=<signed-helper> GOWORK=off go test -race -count=1 -v ./internal/runtime -run '^TestSandboxProcessSupervisorNativeStartup$'
NEXUS_SANDBOX_TEST_BINARY=<fixed-nxs> NEXUS_SUPERVISION_TEST_HELPER=<signed-helper> GOWORK=off go test -race -count=1 -v ./internal/app -run '^TestAppManagedAutoDreamSupervisedNative$'
GOWORK=off make check-architecture
```

固定 SHA256：nxs `6622c107941409c480a8fbb0a517edfde41f542cb450a365f0e37d047ebdd6aa`；helper `7e21e16effd3c00c0d2805d8c09cc850470cdf6b11d37eb5677c5f1dc31735a8`。

## 尚未证明

这是可信身份登记，不是自动资源恢复完成。持锁自动清理、抗目录替换的清理提交与重试、policy/scratch unknown 收口、App 默认装配及完整发布验收仍未完成。非 macOS 不构造此证明，未运行 Windows 验证。
