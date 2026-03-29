/**
 * 双面板侧边栏容器
 *
 * 组合 SidebarNarrowRail（窄栏 Tab 选择器）+ SidebarWidePanel（宽面板内容）。
 * 根据 collapse_mode 控制四种显示模式：
 *   - full：窄栏 88px + 宽面板（可拖拽宽度）
 *   - narrow-only：仅窄栏 88px
 *   - icon-only：窄栏缩小到 56px
 *   - collapsed：完全隐藏
 */

import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";

import { SidebarNarrowRail } from "./sidebar-narrow-rail";
import { SidebarWidePanel } from "./sidebar-wide-panel";

export function AppSidebar() {
  const collapse_mode = useSidebarStore((s) => s.collapse_mode);

  // 全收起时不渲染任何内容
  if (collapse_mode === "collapsed") {
    return null;
  }

  return (
    <div
      className={cn(
        "flex h-full shrink-0",
        // icon-only 模式下窄栏缩小（通过 CSS 变量或 class 控制）
        collapse_mode === "icon-only" && "[&>aside]:w-[56px]",
      )}
    >
      {/* 窄栏 Tab 选择器 */}
      <SidebarNarrowRail />

      {/* 宽面板（仅 full 模式显示，内部自行判断） */}
      <SidebarWidePanel />
    </div>
  );
}
