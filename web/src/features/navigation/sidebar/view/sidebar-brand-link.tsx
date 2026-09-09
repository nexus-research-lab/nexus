/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 保留主题石墨渐变、固定比例且无投影的几何 Nexus 矢量字标。
 * POS: 宽侧栏顶部唯一品牌入口，不承载 Agent 会话动作。
 */
import { useId } from "react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { cn } from "@/shared/ui/class-name";

export function SidebarBrandLink({
  collapsed,
  label,
}: {
  collapsed: boolean;
  label: string;
}) {
  const gradientId = useId();
  return (
    <Link
      aria-label={label}
      aria-hidden={collapsed || undefined}
      className={cn(
        "group/brand relative isolate flex h-10 min-w-0 flex-1 cursor-default items-center rounded-sm transition-opacity duration-(--motion-duration-fast)",
        collapsed
          ? "min-w-0 flex-1 pointer-events-none opacity-0"
          : "",
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
      tabIndex={collapsed ? -1 : undefined}
      to={AppRouteBuilders.launcher()}
    >
      <svg
        aria-hidden="true"
        className="sidebar-brand-wordmark shrink-0 cursor-pointer transition-opacity duration-(--motion-duration-fast) group-hover/brand:opacity-80"
        fill="none"
        focusable="false"
        height="28"
        viewBox="0 0 108 28"
        width="108"
      >
        <defs>
          <linearGradient
            gradientUnits="userSpaceOnUse"
            id={gradientId}
            x1="0"
            x2="0"
            y1="3"
            y2="23"
          >
            <stop offset="4%" stopColor="color-mix(in srgb, var(--text-strong) 94%, white 6%)" />
            <stop offset="48%" stopColor="var(--text-default)" />
            <stop offset="100%" stopColor="color-mix(in srgb, var(--text-muted) 72%, var(--text-strong) 28%)" />
          </linearGradient>
        </defs>
        {/* Open counters and shared rounded strokes echo the surrounding navigation icons. */}
        <g stroke={`url(#${gradientId})`} strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.2">
          <path d="M3 23V3L18 23V3" />
          <path d="M26 16H41C41 11.5 38.4 9 33.5 9S26 11.8 26 16C26 20.5 28.8 23 33.5 23C36.6 23 38.9 22.1 40.3 20.3" />
          <path d="M47 9L60 23M60 9L47 23" />
          <path d="M67 9V16.5C67 20.8 69.4 23 73.5 23C78.2 23 81 20.2 81 16M81 9V23" />
          <path d="M103 10.4C101.2 9.4 98.8 9 96 9C91.5 9 89 10.5 89 13C89 18 103 13.5 103 19C103 21.5 100.4 23 96 23C92.8 23 90.2 22.3 88.5 21" />
        </g>
      </svg>
    </Link>
  );
}
