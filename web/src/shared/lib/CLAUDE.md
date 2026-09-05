# Shared browser and React adapters

本目录只持有与 Nexus 业务身份无关的浏览器能力和 React 生命周期；组件 DOM、样式和业务决策由调用者持有。

- `browser/ime-keyboard-event.ts`：唯一原生输入法事件识别，覆盖 composition、Process 和兼容键码；不持有应用快捷键、提交命令或 composition 结束后的计时状态。
- `browser/clipboard.ts`：唯一剪贴板能力适配，处理异步 API、原生回退、临时元素释放与焦点恢复。
- `react/use-copy-to-clipboard.ts`：仅绑定本地复制成功反馈；卸载清理计时器并忽略迟到反馈，不取消已提交的原生复制。
- `react/use-resettable-state.ts`：按调用者提供的 key 重置本地状态，不解释 Session、Room 或资源 revision。
- `react/use-media-query.ts`、`use-prefers-reduced-motion.ts`：媒体条件和系统动效偏好的订阅；不定义业务断点或动画。
- `react/use-textarea-height.ts`：依据真实 textarea 排版、输入和宽度变化测量高度；不通过 React 状态替代浏览器几何。
- `react/page-header-actions-context.ts`：跨层共享无 DOM 的页头动作插槽；App 持有挂载点和响应式生命周期，页面只读取目标。

直接导入职责文件；不恢复 `hooks/ui` 转发入口或聚合导出。
