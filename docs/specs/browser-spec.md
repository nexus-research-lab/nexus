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

## 连接协议

- WebSocket 路由固定为 `/nexus/v1/internal/browser/ws`，子协议为 `nexus.browser.v1`。
- 扩展依次发送和接收 `browser.ready`、`browser.accepted`、`browser.command`、`browser.result`、`browser.ping` 与 `browser.pong`。
- Handler 只接受 manifest 固定 ID 对应的 `chrome-extension://` Origin。
- `browser.ready` 携带稳定浏览器实例 ID 与当前扩展进程代次；代次变化时宿主立即废弃旧标签页引用。
- 同一时刻只有一个扩展连接；新连接替换旧连接并结束旧连接上的等待请求。

## Session 与标签页

- 每个 runtime Session 保存自己的活动标签页与已归属标签页集合；宿主只复用扩展签发的 `tab_ref`，整数 `tab_id` 仅作结果展示。
- `navigate`、`find_tab`、`attach_active` 和 `attach_tab` 建立 Session 归属；后续页面动作只能使用该 Session 的活动标签页。
- `list_tabs scope=session` 只列出本 Session 标签页，`scope=all` 只用于发现浏览器标签页；模型必须把结果中的 `tab_ref` 交给 `attach_tab` 后才能操作发现的标签页。
- `tab_ref` 同时绑定浏览器实例、扩展进程代次、标签页 ID 和标签页实例 token；关闭后复用的整数 ID、扩展重启前的引用或模型自行构造的引用都会失败关闭。
- 新建标签页按 Session 放入独立 Chrome 标签组；`close_session` 只关闭该 Session 已归属的标签页。

## Browser action

| 能力 | action |
| --- | --- |
| 状态与标签页 | `status`、`navigate`、`find_tab`、`list_tabs`、`attach_active`、`attach_tab`、`back`、`forward`、`reload`、`close_tab`、`close_session`、`close` |
| 页面读取与等待 | `history`、`evaluate`、`page_content`、`wait_for`、`wait_for_url`、`snapshot` |
| 页面调试 | `network`、`console`、`dialog`、`cdp` |
| DOM 与表单 | `click`、`fill`、`check`、`uncheck`、`select_option`、`upload` |
| 键盘与鼠标 | `clipboard`、`key_type`、`send_keys`、`press_key`、`mouse_click`、`double_click`、`hover`、`mouse_move`、`drag`、`scroll` |
| 文件与图像 | `download`、`downloads`、`screenshot`、`save_as_pdf` |

`click` 使用 DOM click；`mouse_click` 与其余鼠标 action 使用真实 CDP 输入事件。DOM action 接受 CSS selector 或最近一次 `snapshot` 返回的 `@e` ref，鼠标 action 也可直接使用视口坐标。

扩展按需向网页顶层文档注入封闭 Shadow DOM，只在当前可见标签页显示不可交互的 Nexus 指针，因此安装前已打开的标签页无需刷新。标签页进入后台时立即隐藏指针，后台收到动作也不显示。普通移动与点击先等待指针抵达再发送 CDP 输入；拖拽只在起点和释放前同步，指针脚本不可用或 1.5 秒内未响应时继续执行原始 CDP 操作。

`snapshot` 返回按页面顺序排列的紧凑可访问性文本，并优先保留可交互节点与页面结构。单次结果最多包含 300 个有效节点和 24 KB UTF-8 文本；超限时通过 `nodes`、`total_nodes` 与 `truncated` 明示裁剪。`@e` ref 只对最近一次快照中实际返回的节点有效。`evaluate` 会等待返回的 Promise 完成，并在 `timeout_ms` 或默认 80 秒后终止执行。

## 完整 CDP

普通 Browser action 使用扩展内部固定的 CDP 方法。模型只有在用户于 Browser 设置中显式开启完整 CDP 后，才能通过 `cdp` action 调用任意方法；该偏好默认关闭并按用户持久化。

[PROTOCOL]: 变更 action、路由、消息类型、Session 归属或 CDP 开关时，必须同步更新本文、Browser 包 L2 头、扩展 README 与相关测试。
