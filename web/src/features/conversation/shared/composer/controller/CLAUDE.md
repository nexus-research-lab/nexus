# controller/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `use-composer-controller.ts`: 组合草稿、附件、提及、Slash 目录、历史和各动作协议
- `use-composer-session-settings.ts`: 读取、缓存并更新 Session 覆盖，投影 Agent/全局继承结果；Room 模型入口打开时补齐成员摘要，保存期间阻止新一轮提交
- `use-composer-draft.ts`: 将正文、附件、输入模式、Goal 负责人和 Mention 目标绑定到包含 Session ID 的 Room/DM 内存草稿胶囊，并独立管理瞬时弹层状态
- `use-composer-message-submit.ts`: 按资格判断、附件准备、投递和收尾阶段提交消息
- `use-composer-goal-actions.ts`: 管理 Goal 与 Loop 动作；宿主请求发出后立即从原 Session 原子认领草稿，ACK 跨 Session 切换按 client_request_id 收口，明确失败只在该作用域仍为空时恢复，受理未知、切换 Session 或继续输入不会被迟到结果覆盖
- `use-composer-keyboard.ts`: 依次执行输入法、Safari、Slash 和 Mention 守卫，再分派键盘命令
- `composer-view-projections.ts`: 分别投影输入、运行时、模式和动作状态
- `composer-controller-model.ts`: 组装各状态投影为视图消费契约

控制器只编排窄接口，不自行复制子领域状态。视图可见状态必须由模型纯函数派生，异步动作不得塞回视图组件；状态投影之间只传递明确结果，不读取彼此的实现条件。
运行状态投影把 `compacting` 作为独立活动传给 Footer，同时继续独立计算停止按钮资格。
`hasStopAction` 是表面能力而非运行态推断：DM 传入 Composer 时才允许停止按钮和 Escape 停止，Room 始终由 Agent slot 持有停止入口。
`awaiting_permission` 阶段保留输入草稿但禁止消息提交；Enter 和发送按钮必须共享同一提交资格，不能越过权限交互。
Composer 挂载或 Session 草稿作用域变化后，控制器只执行一次聚焦并把 selection 移到当前草稿末尾；上下键召回历史后同样在下一帧把 selection 放到召回正文末尾；普通输入更新不得重置用户主动选择的光标位置。
Slash 选择只负责改写 textarea 草稿与光标；host/runtime command 都由现有消息提交器作为普通文本发送，不能在键盘控制器中旁路发送资格、附件或队列策略。
消息投递只有在后端 ACK 后才进入收尾阶段；完整草稿修订号仍等于提交时修订号才原子清空，迟到 ACK 不得删除更新后的正文、附件、模式、Goal 负责人或 Mention 目标；传输失败或 ACK 超时必须保留整个草稿胶囊。
Goal 控制命令与普通消息的确认语义不同：发送函数同步建立宿主请求后即认领原 Session 草稿，避免用户切换 Session 时仍看到已派发的 Goal；普通切换不取消旧请求，ACK/拒绝按 client_request_id 回到原 Promise，明确失败仅恢复原作用域且不得覆盖其后产生的新草稿，受理未知不得诱导重复提交。
