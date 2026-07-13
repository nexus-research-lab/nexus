# 启动恢复

- chunk 错误识别与全局监听归 `chunk-error-recovery.ts`，不得混入 React 渲染命令。
- 所有自动刷新必须经过 `reload-guard.ts` 的 Session 哨兵，哨兵不可用时拒绝刷新循环。
- `render-watchdog.ts` 只采样桌面渲染快照并驱动健康状态，不拥有恢复页 JSX。
- watchdog 状态和用户可见原因使用数据表映射，新增状态时必须显式补齐。
