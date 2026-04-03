/**
 * 应用侧栏容器
 *
 * 保留信息面板本体，把原来独立的窄导航栏并入面板头部，
 * 避免左侧出现只占空间的第一列。
 */

import { SidebarWidePanel } from "@/shared/ui/sidebar/sidebar-wide-panel";

export function AppSidebar() {
  return <SidebarWidePanel />;
}
