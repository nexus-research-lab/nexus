# Browser 能力规范

本文记录 Nexus Browser 的当前跨进程合同。Browser 是该能力在设置、MCP、后端与 Chromium 扩展中的唯一名称。

## 架构边界

```text
runtime MCP browser
  -> internal/mcp/browser
  -> internal/service/browser
  -> internal/handler/browser
  -> desktop/browser-extension
```

- `internal/mcp/browser` 只暴露一个 `browser` 工具，并把截图与 PDF 转成 MCP content。
- `internal/service/browser` 是 action、输入校验、扩展连接和 Session 标签页归属的业务真相源。
- `internal/handler/browser` 只处理状态查询、扩展身份校验和 WebSocket 消息。
- `desktop/browser-extension` 是 Chromium Manifest V3 扩展，负责调用 Chrome API 与 Chrome DevTools Protocol。
- `web/src/features/settings/browser` 只负责安装引导、连接状态与完整 CDP 开关，不承载浏览器数据或动作。

## 安装与状态

- 桌面构建把扩展放在应用资源目录；设置页不下载或复制第二份扩展。
- 安装入口优先打开已安装的 Google Chrome，其次打开 Microsoft Edge 的扩展程序页面，并在 Finder 或 Explorer 中定位同一扩展目录。
- 状态接口区分未连接、已连接和协议不兼容；不兼容握手会保留扩展版本与协议版本，成功连接后立即清除。
- 扩展弹窗显示当前活动页与控制状态；弹窗按钮和网页右键菜单通过 `nexus://launcher?initial=...` 打开 Launcher 草稿，用户仍需自行选择 Agent 或发送消息。

## 连接协议

- WebSocket 路由固定为 `/nexus/v1/internal/browser/ws`，子协议为 `nexus.browser.v1`。
- 扩展依次发送和接收 `browser.ready`、`browser.accepted`、`browser.command`、`browser.result`、`browser.event`、`browser.ping` 与 `browser.pong`。
- Handler 只接受 manifest 固定 ID 对应的 `chrome-extension://` Origin。
- `browser.ready` 携带浏览器名称、稳定浏览器实例 ID 与当前扩展进程代次；代次变化时宿主立即废弃旧标签页引用。
- 同一时刻只有一个扩展连接；新连接替换旧连接并结束旧连接上的等待请求。

## Session 与标签页

- 每个 runtime Session 保存自己的活动标签页与已归属标签页集合；宿主只复用扩展签发的 `tab_ref`，整数 `tab_id` 仅作结果展示。
- `navigate`、`find_tab`、`attach_active` 和 `attach_tab` 建立 Session 归属；后续页面动作只能使用该 Session 的活动标签页。
- `list_tabs scope=session` 只列出本 Session 标签页，`scope=all` 只用于发现浏览器标签页；模型必须把结果中的 `tab_ref` 交给 `attach_tab` 后才能操作发现的标签页。
- `tab_ref` 同时绑定浏览器实例、扩展进程代次、标签页 ID 和标签页实例 token；关闭后复用的整数 ID、扩展重启前的引用或模型自行构造的引用都会失败关闭。
- 扩展监听 `webNavigation.onCreatedNavigationTarget`；由已归属标签页创建的新标签页继承来源 Session 和标签组，并以 `browser.event/tab_created` 主动更新宿主活动页。事件丢失时，扩展侧 Session 租约会在下一次 `list_tabs` 中补回。
- 扩展通过 `tab_updated`、`tab_activated` 与 `tab_removed` 事件同步受控页的导航、激活和关闭状态；宿主只接受当前 Session 已持有的不透明引用。
- 新建标签页按 Session 放入独立 Chrome 标签组；`close_session` 只关闭该 Session 已归属的标签页。
- 标签页租约绑定当前 runtime round。round 结束时，扩展关闭本轮 Agent 新建且未标记的临时页，并对借用的用户页或 `deliverable` 结果页解除调试连接；Agent 新建的 `deliverable` 结果页同时移出 Nexus 临时标签组，借用页保留用户原有分组；`handoff` 页保留 Session 归属，供下一轮继续控制。

## Browser action

| 能力 | action |
| --- | --- |
| 批处理 | `batch` |
| 状态与标签页 | `status`、`navigate`、`find_tab`、`list_tabs`、`attach_active`、`attach_tab`、`mark_tab`、`back`、`forward`、`reload`、`close_tab`、`close_session`、`close` |
| 页面读取与等待 | `history`、`evaluate`、`page_content`、`wait_for`、`wait_for_url`、`snapshot` |
| 页面调试 | `network`、`console`、`dialog`、`cdp` |
| DOM 与表单 | `click`、`fill`、`check`、`uncheck`、`select_option`、`upload` |
| 键盘与鼠标 | `clipboard`、`key_type`、`send_keys`、`press_key`、`mouse_click`、`double_click`、`hover`、`mouse_move`、`drag`、`scroll` |
| 文件与图像 | `download`、`downloads`、`screenshot`、`save_as_pdf` |

`click` 与其余鼠标 action 使用真实 CDP 输入事件；`click` 固定为左键单击，`mouse_click` 额外接受坐标、按钮与点击次数。DOM action 接受 CSS selector 或最近一次 `snapshot` 返回的 `@e` ref，鼠标 action 也可直接使用视口坐标。页面点击不得用 `evaluate` 调用 `element.click()` 绕过真实输入与可见指针。

`batch` 在宿主内顺序复用现有 action 和输入校验，不增加扩展线协议。它最多接受 20 个不需要消费中间结果的导航、等待、表单、键鼠或上传动作，默认在全部动作结束后只返回一次最终 AX 快照；失败会报告精确动作序号和已完成数量，禁止嵌套 `batch`、读取结果、调试、截图、下载或原始 CDP。

`mark_tab` 的 `mark` 接受 `deliverable`、`handoff` 或 `none`。`deliverable` 在本轮结束后保留页面并交还用户，`handoff` 保留控制权供下一轮继续，`none` 恢复默认收尾策略。

需要 `cmd` 的 action 使用判别式输入约束：`network` 只接受 `start|stop|list|detail`，`console` 只接受 `start|stop|list`，`dialog` 只接受 `get|accept|dismiss`，`downloads` 只接受 `list|wait|show`，`clipboard` 只接受 `read|write`。MCP schema 在模型生成参数时约束组合，Browser service 在执行前再次校验。

扩展按需向网页顶层文档注入封闭 Shadow DOM，只在当前可见标签页显示不可交互的 Nexus 指针，因此安装前已打开的标签页无需刷新。标签页进入后台时立即隐藏指针，后台收到动作也不显示。首次操作从视口中部起步，普通移动与点击沿平滑弧线等待指针抵达后再发送 CDP 输入，抵达后轻微左右摆动并保持到本轮结束；拖拽只在起点和释放前同步，指针脚本不可用或 1.5 秒内未响应时继续执行原始 CDP 操作。

`snapshot` 返回按页面顺序排列的紧凑可访问性文本，并优先保留可交互节点与页面结构。单次结果最多包含 300 个有效节点和 24 KB UTF-8 文本；超限时通过 `nodes`、`total_nodes` 与 `truncated` 明示裁剪。同一文档的 `@e` ref 跨快照保持稳定，导航后立即失效。首个快照和 `full=true` 返回 `snapshot_type=full`；后续在更紧凑时返回相对 `base_snapshot_id` 的 `diff`，无变化时返回 `unchanged`。`evaluate` 会等待返回的 Promise 完成，并在 `timeout_ms` 或默认 80 秒后终止执行。

## 完整 CDP

普通 Browser action 使用扩展内部固定的 CDP 方法。模型只有在用户于 Browser 设置中显式开启完整 CDP 后，才能通过 `cdp` action 调用任意方法；该偏好默认关闭并按用户持久化。

[PROTOCOL]: 变更 action、路由、消息类型、Session 归属或 CDP 开关时，必须同步更新本文、Browser 包 L2 头、扩展 README 与相关测试。
