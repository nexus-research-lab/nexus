# Room 页面协调

- 浏览器 URL 是当前 Room、Conversation 和外部 Session 路由的唯一真相源。
- 每个 Room 最后一次显式激活的 Conversation 持久化为入口恢复偏好；普通目录入口先进入 Room 根路由并校验该偏好，明确 Conversation URL 和未读目标仍优先。
- 会话专注模式的返回动作进入 `/app` 聊天目录并展开宽侧栏，不返回启动台。
- `initial` 查询参数只消费一次，进入页面状态后立即从地址栏移除。
- 页面事件只处理路由失效和资源重同步；消息、Goal 等领域事件由各自控制器消费。
- 新建 Session 只在当前选中项被服务端明确标记为 `is_draft` 时留在当前页；否则调用服务端创建/确保命令，同一页面作用域内保持单飞。历史 Conversation 不参与草稿推断，创建响应也不作为跨渲染的本地草稿真相；标题只作为 draft 的可选初始元数据，不能绕过每 Room 唯一未开始 Session。
- `use-room-page-navigation.test.tsx` 组合真实路由、标签控制器与 owner 绑定 Store，验证 Room 根入口和精确 Conversation 入口的新建、历史选择都保留先前打开的标签。
- `room-session-navigation.test.tsx` 进一步组合真实目录加载、页面写命令、会话投影、历史菜单与标签 DOM，只替换 HTTP 结果；DM/Group 均覆盖新建后的延迟目录刷新、当前草稿复用、历史追加、标签切换、关闭后重新打开、固定保留和 owner 重新绑定后的根路由恢复。jsdom 不作为实际窗口布局或点击命中验收。
