# 启动恢复

- chunk 错误识别与全局监听归 `chunk-error-recovery.ts`，不得混入 React 渲染命令。
- 浏览器 `ResizeObserver` 循环告警不是崩溃，不得上报为 desktop web fatal；真实未处理异常仍保留诊断。
- 所有自动刷新必须经过 `reload-guard.ts` 的 Session 哨兵，哨兵不可用时拒绝刷新循环。
- `render-watchdog.ts` 只在桌面窗口重新聚焦或可见时采样渲染快照并驱动健康状态，不拥有恢复页 JSX，不启动周期探测。
- 渲染健康状态和用户可见原因使用数据表映射，新增状态时必须显式补齐。
