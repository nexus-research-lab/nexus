# 消息领域

- `message-content-model.ts` 负责跨消息项、时间线和会话导航共享的文本协议清理与内容提取；`nexus_room_no_reply`、`nexus_room_fanout` 等 Room 编排标记必须在这里一次剥离，正文、result、复制与历史投影不得各自维护名单。
- `message-tool-names.ts` 只保存跨活动、过程和视图共同依赖的稳定工具标识，避免兄弟领域相互借用常量。
- `message-time.ts` 只负责消息时间的稳定格式化，不读取视图状态。
- `tool-activity.ts` 统一工具名称与输入摘要，供工具块和消息过程摘要直接消费，不依赖具体块视图。
- `item/message-item-projection.ts` 定义消息项内部的有序条目、轮次和内容投影，不承载 DOM 或视觉规则。
- `item/activity/` 统一活动状态契约，以及轮次级和内容块级的纯状态推导。
- `item/process/` 负责过程摘要、问答超时识别与 DM live 连续工具段纯投影；工具段只接收人工交互工具 ID 集合作为边界，不取得权限动作所有权。
- `ui/` 只保留跨消息表面的头像、动作、轨道和统计；消息项私有视图不得上提到共享聚合文件。
- `markdown-renderer.tsx` 只把消息文件产物协议适配到共享 Markdown；通用渲染能力归 `shared/ui/markdown/`。
- `agent-handoff-status-context.tsx` 只桥接按 `handoff_id` 投影的 mention 阶段；状态真相仍属于 Room 面板，mention chip 只原位展示，不建立第二张 Agent 卡。
- DM pending interaction 的操作 owner 是 Composer：消息内容只保留等待确认的只读过程证据；Room 继续由对应 Agent 消息轨道持有完整权限、问答与计划确认组件。
- 单消费者逻辑留在拥有它的 controller/view；禁止重新建立聚合 helper 或通过根 barrel 暴露内部模型。
- `MessageItem` 由 `item/message-item.tsx` 直接公开，消费者不得绕回消息目录聚合出口。
- 消息项控制器只返回按 User/Assistant 和视觉职责分组的具体状态；各视图在消费侧声明所需结构，不共享宽状态接口。
