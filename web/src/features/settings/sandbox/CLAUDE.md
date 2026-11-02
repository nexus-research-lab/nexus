# sandbox/

沙箱设置分区，仅桌面端可见（侧栏与 URL 门禁同 Browser）。集中承载所有沙箱相关设置，其他分区不再重复展示。

- `use-sandbox-settings-controller.ts` 持有桌面沙箱开关（偏好 `desktop_sandbox_enabled`，复用 General 的版本化 Preferences 事务）与显式沙箱诊断。
- 显式“检查沙箱支持”单独调用有界诊断，不复用偏好保存锁、不修改 Preferences。依赖齐全只表示可以开启，不展示为任务已启用沙箱；旧接口和读取失败保持未知。
- 沙箱按命令无状态执行，没有需要用户回收或放弃的残留资源，本分区不提供回收入口。
- 文案沿用 `settings.general.desktop_sandbox_*` 与 `settings.runtime.sandbox_*` 键，分区标题使用 `settings.sandbox.section_title`。
- `settings-sandbox-section.test.tsx` 覆盖开关与诊断交互；`use-sandbox-settings-controller.test.tsx` 覆盖 API 调用与偏好更新边界。
- 两行与 `SettingsToggleRow` 同一几何：文字区占满、控件贴右并垂直居中；动作按钮统一 `UiButton size="xs" variant="surface"`，等待态用共享 `xs` Spinner 替换图标。行标题已说明用途，按钮只写动作（“检查”），不重复标题。诊断结果按 `SANDBOX_STATE_TONES` 着色：依赖齐全 success、未知 muted、其余 warning。
