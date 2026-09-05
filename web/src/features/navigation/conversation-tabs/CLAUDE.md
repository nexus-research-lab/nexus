# Room 会话标签导航

- `room-conversation-tabs.tsx` 是 DM Header、Group Header 与 Contacts 联络 Header 的业务入口：读取当前 owner 的固定偏好，投影标题、外部 Session 标签和关闭/固定资格，再把受控事实与独立命令交给共享 `WorkspaceConversationTabs`。此处不复制标签 DOM、样式或滚动。
- `use-room-conversation-tabs.ts` 拥有打开集合、乐观活动项、创建与最终替换单飞、关闭回退和按 Room 持久化。`store/room-navigation.ts` 继续拥有持久化介质及主侧栏共用的固定集合；关闭标签不取消固定，删除会话才清理固定项。
- `room-conversation-tabs-model.ts` 定义创建时间顺序、存活集合归约、初始恢复、活动项、关闭邻居、持久化资格和当前内部 draft 判断。首次进入只打开恢复目标；之后只因显式选择、新建或外部导航追加，消息活动及服务端新增历史不得重排或补入标签。归约按保留存活项、追加外部选中项、保证非空执行，活动项始终属于打开集合。
- `final-conversation-replacement.ts` 及其 `FinalConversationReplacementHandler` 拥有最后标签替换事务合同。已开始会话先确保唯一 draft 并提交，再后台停止旧 runtime；可能复用同 ID 的 draft 先等 runtime 关闭，再确保并提交。失败或较新导航保留原正文，外部 Session 跳过 runtime 关闭。页面绑定精确 Room/Conversation/导航版本守卫；共享视图不得解释此事务。
- 创建是否复用只取当前选中内部 Session 的 `is_draft === true`；不扫描历史、按消息数猜测或复用外部 Session。标题不能绕过服务端每 Room 唯一 draft 约束；页面创建命令和标签入口均保持单飞。
- 共享宽度、测量、DOM、滚动和拖拽仍归 `shared/ui/workspace/controls/conversation-tabs/`。Gallery 直接提供受控 UI fixture，不调用本 Feature 或 Room Store。
- 共置 Hook 测试覆盖恢复、显式选择、旧路由关闭收口、固定保留和单飞替换；事务测试覆盖 draft/已开始/外部会话顺序、失败与过期提交；入口测试覆盖精确 Room 固定身份和禁用能力。
