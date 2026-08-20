# Nexus WebBridge

Nexus 自有的 Chromium 扩展，通过 WebSocket 把浏览器 computer-use 能力交给 Nexus Agent。

## 桌面端安装

在 Nexus 设置中打开「电脑操控」，点击「开始安装」，再按页面提示选择「加载未打包的扩展程序」并打开 Nexus 已定位的扩展文件夹。连接成功后，设置页会自动显示扩展版本。

## 源码安装

1. 打开 `chrome://extensions` 或 `edge://extensions`。
2. 开启「开发者模式」。
3. 选择「加载已解压的扩展程序」，指向本目录 `desktop/browser-extension`。
4. 启动 Nexus Desktop。扩展角标显示 `ON` 即已连接。

源码开发时，先设置 `NEXUS_WEBBRIDGE_ENABLED=true` 再启动端口 8010 的 Nexus Server。扩展默认依次尝试桌面固定端口 `34343` 和开发端口 `8010`；也可以在扩展弹窗中填写任意 `ws://` 或 `wss://` 地址并测试、连接或断开。

## 能力

- 每个 Nexus Session 独立管理和分组多个标签页，支持导航、新建、查找、借用、列出和关闭。
- 支持可访问性树快照以及 CSS selector / `@e` ref 的 DOM 点击、真实鼠标点击和输入。
- 支持文本插入、组合键、重复按键、任意 JavaScript，以及用户明确开启后的完整 Chrome DevTools Protocol 命令。
- 支持网络记录、请求过滤、响应详情与正文读取。
- 支持本机文件上传、PNG/JPEG 整页或元素截图，以及常用纸张规格的 PDF 导出。

扩展使用 `debugger`、`tabs`、`tabGroups`、`storage`、`windows` 与 `<all_urls>` 权限完成上述操作。
