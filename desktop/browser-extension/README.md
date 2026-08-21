# Nexus Browser

Nexus 自有的 Chromium 扩展，通过 WebSocket 把完整 Browser 能力交给 Nexus Agent。

## 桌面端安装

在 Nexus 设置中打开「浏览器」并点击「开始安装」。Nexus 会自动打开已安装的 Google Chrome 或 Microsoft Edge 的扩展程序页面，并在系统文件管理器中定位随应用打包的扩展目录。按页面提示选择「加载未打包的扩展程序」；连接成功后，设置页会显示浏览器和扩展版本，旧协议扩展则会直接提示更新。

## 源码安装

1. 打开 `chrome://extensions` 或 `edge://extensions`。
2. 开启「开发者模式」。
3. 选择「加载已解压的扩展程序」，指向本目录 `desktop/browser-extension`。
4. 启动 Nexus Desktop。扩展角标显示 `ON` 即已连接。

源码开发时，先设置 `NEXUS_BROWSER_ENABLED=true` 再启动端口 8010 的 Nexus Server。扩展默认依次尝试桌面固定端口 `34343` 和开发端口 `8010`；也可以在扩展弹窗中填写任意 `ws://` 或 `wss://` 地址并测试、连接或断开。

## 能力

- 每个 Nexus Session 通过绑定扩展代次和标签页实例的不透明引用独立管理多个标签页，支持导航、新建、查找、借用、前进后退、刷新、列出全部标签页和读取历史。
- 受控页面创建的 OAuth、登录或 `target=_blank` 子标签页会自动继承来源 Session、标签组与活动页控制。
- 每个 runtime round 结束时自动关闭 Agent 新建的临时页并解除用户页面控制；Agent 可用 `mark_tab` 把结果页交付给用户，或留给下一轮继续操作。
- 支持有上限且默认增量返回的可访问性树快照；同页 `@e` ref 跨快照保持稳定，导航后立即失效，也可用 `full` 强制刷新完整基线。
- 支持页面正文/HTML、元素等待，以及 CSS selector / 最新快照 `@e` ref 或坐标驱动的点击、双击、悬停、拖拽、滚动和表单操作。
- 鼠标操作只在当前可见标签页显示带 Nexus 蓝紫光晕的指针；指针抵达目标后才执行点击，受限页面无法显示时自动退回原有 CDP 操作。
- 支持文本插入、组合键、重复按键、剪贴板文本读写、会等待 Promise 的任意 JavaScript，以及用户明确开启后的完整 Chrome DevTools Protocol 命令。
- 支持网络记录、请求过滤、响应详情、控制台日志与 JavaScript 对话框处理。
- 支持本机文件上传、浏览器下载管理、PNG/JPEG 整页或元素截图，以及常用纸张规格的 PDF 导出。

扩展使用 `debugger`、`tabs`、`tabGroups`、`webNavigation`、`history`、`downloads`、`clipboardRead`、`clipboardWrite`、`offscreen`、`scripting`、`storage`、`windows` 与 `<all_urls>` 权限完成上述操作。

## 图标

`icons/browser-icon-master.png` 是图标母版。修改后导出 `16`、`32`、`48` 和 `128` 像素 PNG；Manifest 与扩展弹窗共用这些文件。
