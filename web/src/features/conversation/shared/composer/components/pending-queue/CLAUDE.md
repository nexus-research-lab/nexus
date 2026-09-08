# pending-queue/

L6 | 父级: web/src/features/conversation/shared/composer/components

## 职责

- `composer-pending-queue.tsx`: 使用共享 Disclosure 装配原生折叠头和有序消息列表
- `pending-queue-item.tsx`: 展示单条待发送消息、内容描述与独立引导/删除动作；排序手柄组合公共 IconButton/Action Menu
- `pending-queue-model.ts`: 处理重排、拖动身份、正文/附件和引导状态的纯投影；密度 padding 由上层 `composer-styles.ts` 唯一持有
- `use-pending-queue-controller.ts`: 管理拖拽/相邻移动、有限边缘滚动和串行派发；折叠由 Disclosure 持有

拖拽中的 DOM/动画帧状态只存在于 controller，消息行不直接操作共享引用。重排函数保持纯函数，视图只提交排序后的 ID。
队列头使用 Disclosure caption，正文使用 Typography supporting；微型动作复用共享 Button。消息正文保留完整 title，并通过 aria-describedby 关联行内动作；空内容显示本地化占位。密度和层级的当前合同见根 design.md。

- 拖动只从排序手柄开始，正文可选择；同一手柄也能打开上移/下移菜单，键盘与鼠标共用一个命令路径。边缘项禁用无效方向，无相邻项或派发中时清空菜单并禁用排序，恢复后不自动重开。
- 控制器只对当前 IDs 执行非空变更；同位/失效拖放不发命令。引导、删除、重排在当前 Promise 派发阶段共享同步 ref 保护，开始命令先终止拖动；Promise 成功不等同服务端已受理，错误仍由 Conversation transport 投影，不能新增一份猜测的结果或自动重放。
- 仅本队列真实拖动可以启动边缘滚动；到边界或中间停止动画帧，条目消失、折叠、drag end 与卸载清理运行时。队列顺序从 props 的服务端快照读取，不做私有乐观重排。
- ComposerPanel 以包含 Session 的 draftScopeKey 为队列实例 key；切换会话不继承折叠、菜单、拖动或派发忙碌。空队列不保留 Disclosure，下一次新队列重新展开。
