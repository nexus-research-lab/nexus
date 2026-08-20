# controller/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `use-composer-controller.ts`: 组合草稿、附件、提及、Slash 目录、历史和各动作协议
- `use-composer-session-settings.ts`: 读取、缓存并更新 Session 的模型、权限与 Connector 覆盖，投影 Agent/全局继承结果；Room 模型入口打开时补齐成员摘要，保存期间阻止新一轮提交
- `use-composer-local-directories.ts`: 为桌面 DM/Room 读取、选择、移除并保存 Session 级本机工作文件夹；Room 将同一目录快照投影给全部成员 runtime
- `use-composer-draft.ts`: 将正文、附件、输入模式、Goal 负责人和 Mention 目标绑定到包含 Session ID 的 Room/DM 内存草稿胶囊，并独立管理瞬时弹层状态
- `use-composer-message-submit.ts`: 按资格判断、附件准备、投递和收尾阶段提交消息
- `use-composer-goal-actions.ts`: 管理 Goal 与 Loop 动作；宿主请求发出后立即从原 Session 原子认领草稿，ACK 跨 Session 切换按 client_request_id 收口，明确失败只在该作用域仍为空时恢复；带 transport identity 的 post-send 失败同时保留 recovery receipt，由更新过的 owner-scoped Goal fence 或 exact durable Goal 控制记录撤回同一自动恢复修订，受理未知继续进入原 scope 的“确认中”互斥态
- `use-composer-keyboard.ts`: 依次执行输入法、Safari、Slash 和 Mention 守卫，再分派键盘命令
- `composer-view-projections.ts`: 分别投影输入、运行时、模式和动作状态
- `composer-controller-model.ts`: 组装各状态投影为视图消费契约

控制器只编排窄接口，不自行复制子领域状态。视图可见状态必须由模型纯函数派生，异步动作不得塞回视图组件；状态投影之间只传递明确结果，不读取彼此的实现条件。
运行状态投影把 `compacting` 作为独立活动传给 Footer，同时继续独立计算停止按钮资格。
`hasStopAction` 是表面能力而非运行态推断：DM 传入 Composer 时才允许停止按钮和 Escape 停止，Room 始终由 Agent slot 持有停止入口。
`awaiting_permission` 阶段保留输入草稿但禁止消息提交；Enter 和发送按钮必须共享同一提交资格，不能越过权限交互。
Composer 挂载或 Session 草稿作用域变化后，控制器只执行一次聚焦并把 selection 移到当前草稿末尾；上下键召回历史后同样在下一帧把 selection 放到召回正文末尾；普通输入更新不得重置用户主动选择的光标位置。
Slash 选择只负责改写 textarea 草稿与光标；host/runtime command 都由现有消息提交器作为普通文本发送，不能在键盘控制器中旁路发送资格、附件或队列策略。
普通消息在本地协议派发成功后立即按提交修订号原子认领草稿，迟到 ACK 不得删除更新后的正文、附件、模式、Goal 负责人或 Mention 目标；发送前失败或后端明确拒绝只在原作用域仍未产生新草稿时恢复，受理状态未知不得恢复，避免把可能已落盘的内容再次发送。
Goal、普通消息、编辑重跑与队列输入共用发送前建立的 exact transport owner，切换和新建 Session 都保留原 owner，ACK/拒绝按 client_request_id 回到原 Promise且只收口原 Session。Goal 另有 confirming/recovery 状态：明确失败仅恢复原作用域且不得覆盖其后产生的新草稿；已越过发送边界的明确失败必须把 exact request identity 与恢复修订号保留到 recovery receipt，迟到 acceptance 只删除未被编辑的自动恢复内容并清除旧错误，新提交原子废止旧 receipt。受理未知显示“确认中”且禁止同 scope 重复提交；旧的同 objective Goal 不能当证据，只有越过提交前 Goal ID/version baseline 的 owner-scoped 快照，或原 Session 中 message_id 已由服务端签发且 client_message_id 精确匹配的 durable `/goal` 控制记录才能解除。
