# Scheduled Task Dialog

- 根目录只保留弹窗入口、组合控制器和跨层共享类型。
- `form/` 负责目标、执行会话、结果回传、初始化和提交载荷。
- `schedule/` 负责时间规则、时区、日期选择器和计划视图。
- `resources/` 负责按当前目标依赖加载 Agent、Room 与会话资源。
- 控制器只维护表单、资源与提交事务；创建、更新和错误投影按独立阶段执行，Escape、焦点循环和滚动锁复用共享 Dialog 协议。
- 表单和计划分别使用单一草稿对象，不增加相互镜像的字段状态。
- Room 任务始终绑定共享 Room Session，并把执行 Agent 与结果回复 Agent 分别作为该 Session 的成员选择；留空表示房主，提交层必须解析为具体 Agent 后再写入。
- 结果投递只列真实存在的 Nexus、IM 或 Room 会话；“智能体会话”只列结构化 `chat_type=dm` 的 Nexus/active-paired IM Session（Room-backed DM 即使带 `room_id` 仍属于 DM），“Room 会话”只列 `room_type=room` 并把 `chat_type=group` 的成员 Session 折叠为共享 Room Session。两类候选不得交叉，页面不得合成“智能体收件箱”一类没有用户来源的内部 Session。
- 编辑历史任务时，会话选择器只列当前可用会话；旧裸路由、合成收件箱、已解绑、已删除或无法证明 active pairing 的 IM 会话不得作为候选项。待重绑任务清空受影响选择并展示失效状态；用户改选目标 Agent 必须原子更新任务 Agent 与对应执行/回传路由，但不得改写创建来源。
- 建议入口只注入创建初值，进入弹窗后仍由同一草稿、校验和提交事务接管。
- 创建与编辑共用固定响应式高度；计划类型、错误和高级字段只改变可滚动正文，不得驱动弹窗外框跳动。
