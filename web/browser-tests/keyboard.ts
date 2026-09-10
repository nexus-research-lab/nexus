// INPUT: 浏览器项目、运行平台和前后遍历方向。
// OUTPUT: 宿主支持的真实全控件键盘遍历，不改系统偏好或 DOM 焦点顺序。
// POS: 浏览器测试键盘适配；macOS WebKit 默认用 Option-Tab 遍历非文本控件。

import type { Page, TestInfo } from "@playwright/test";

export async function moveKeyboardFocus(page: Page, info: TestInfo, reverse = false) {
  // https://support.apple.com/guide/safari/cpsh003/mac
  const option = process.platform === "darwin" && info.project.use.browserName === "webkit";
  await page.keyboard.press(`${option ? "Alt+" : ""}${reverse ? "Shift+" : ""}Tab`);
}
