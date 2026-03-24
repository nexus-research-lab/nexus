/**
 * 兼容入口页面
 *
 * 当前主路由已经迁移到 pages/launcher、pages/room、pages/nexus、pages/contacts。
 * 这里暂时保留 HomePage，避免旧引用立即失效。
 */

import { LauncherPage } from "@/pages/launcher/launcher-page";

export function HomePage() {
  return <LauncherPage />;
}
