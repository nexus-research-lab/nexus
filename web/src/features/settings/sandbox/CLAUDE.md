# sandbox/

沙箱设置分区，仅桌面端可见（侧栏与 URL 门禁同 Browser）。集中承载所有沙箱相关设置，其他分区不再重复展示。

- `use-sandbox-settings-controller.ts` 持有桌面沙箱开关（偏好 `desktop_sandbox_enabled`，复用 General 的版本化 Preferences 事务）、显式沙箱诊断与残留资源回收。
- 显式“检查沙箱支持”单独调用有界诊断，不复用偏好保存锁、不修改 Preferences。依赖齐全只表示可以开启，不展示为任务已启用沙箱；旧接口和读取失败保持未知。
- 沙箱残留资源恢复先调用 owner-scoped inspect，再调用 dry-run reconcile；只有用户点击明确的回收动作才提交 `apply=true`。页面只投影资源数量、可回收数量和 `cleanup_unknown` 待核对数量，禁止显示 marker 路径或把 dead PID 解释为已安全收口。
- 文案沿用 `settings.general.desktop_sandbox_*` 与 `settings.runtime.sandbox_*` 键，分区标题使用 `settings.sandbox.section_title`。
- `settings-sandbox-section.test.tsx` 覆盖开关、诊断与回收交互；`use-sandbox-settings-controller.test.tsx` 覆盖 API 调用与偏好更新边界。
