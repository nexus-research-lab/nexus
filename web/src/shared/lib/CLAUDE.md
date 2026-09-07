# Shared selection, browser and React adapters

本目录只持有与 Nexus 业务身份无关的纯选择投影、浏览器能力和 React 生命周期；组件 DOM、样式和业务决策由调用者持有。

- `selection-options.ts` 是同名/缺名候选序号与缺项当前值显示的中立算法；候选和显示目录分离，目录不能增加候选。调用方传入普通 value、名称、可选创建时间与文案，不读取 Agent/Room 类型或 Store；原顺序/value 保留，序号不持久化，禁用显示项不成为资源资格。

- `browser/ime-keyboard-event.ts`：唯一原生输入法事件识别，覆盖 composition、Process 和兼容键码；不持有应用快捷键、提交命令或 composition 结束后的计时状态。
- `browser/clipboard.ts`：唯一剪贴板能力适配，处理异步 API、原生回退、临时元素释放与焦点恢复。
- `react/use-copy-to-clipboard.ts`：仅绑定本地复制成功反馈；卸载清理计时器并忽略迟到反馈，不取消已提交的原生复制。
- `react/use-resettable-state.ts`：按调用者提供的 key 重置本地状态，不解释 Session、Room 或资源 revision。
- `react/use-mouse-drag.ts`：首页辅助面板与文件目录分栏的共用鼠标拖动生命周期；主键松开、窗口失焦、文档隐藏和调用方停用均结束，卸载释放监听。只传递仍按住主键的移动，尺寸、方向和边界留在布局 owner；侧栏的指针捕获/折叠热区保留独立职责。
- `react/use-media-query.ts` 是媒体条件首次读取、变更订阅和清理的唯一所有者；没有 matchMedia 时返回 false。`use-prefers-reduced-motion.ts` 只绑定固定系统动效查询，不重复实现监听，不定义业务断点或动画。
- `react/use-textarea-height.ts`：依据真实 textarea 排版、输入和宽度变化测量高度；不通过 React 状态替代浏览器几何。
- `react/page-header-actions-context.ts`：跨层共享无 DOM 的页头动作插槽；App 持有挂载点和响应式生命周期，页面只读取目标。

直接导入职责文件；不恢复 `hooks/ui` 转发入口或聚合导出。
