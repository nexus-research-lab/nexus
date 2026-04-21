/**
 * AgentOptions 左侧图标导航栏
 *
 * 垂直标签导航，强调配置分组与当前上下文
 */

"use client";

import { ReactNode } from "react";
import { UserPen, ToolCase, Album, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";

/** Tab 键值类型 */
export type TabKey = "identity" | "skills" | "advanced";

/** 单个导航项配置 */
interface NavItem {
  key: TabKey;
  label_key:
    | "agent_options.nav.identity"
    | "agent_options.nav.tools"
    | "agent_options.nav.skills";
  icon: LucideIcon;
}

/** 导航栏 Tab 配置列表 */
const NAV_ITEMS: NavItem[] = [
  { key: "identity", label_key: "agent_options.nav.identity", icon: UserPen },
  { key: "advanced", label_key: "agent_options.nav.tools", icon: ToolCase },
  { key: "skills", label_key: "agent_options.nav.skills", icon: Album },
];

interface AgentOptionsNavProps {
  active_tab: TabKey;
  on_tab_change: (tab: TabKey) => void;
  variant?: "sidebar" | "inline";
  trailing?: ReactNode;
}

/** 左侧图标导航栏组件 */
export function AgentOptionsNav({
  active_tab,
  on_tab_change,
  variant = "sidebar",
  trailing,
}: AgentOptionsNavProps) {
  const { t } = useI18n();

  if (variant === "inline") {
    return (
      <div className="flex items-center justify-between gap-4 border-b dialog-divider px-6 py-3">
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            const isActive = active_tab === item.key;
            const label = t(item.label_key);
            return (
              <button
                key={item.key}
                onClick={() => on_tab_change(item.key)}
                title={label}
                className={cn(
                  "inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-[color,background,border-color] duration-(--motion-duration-normal)",
                  isActive
                    ? "border border-primary/18 bg-primary/8 text-primary"
                    : "border border-transparent text-(--text-muted) hover:border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                )}
                type="button"
              >
                <Icon className="h-3.5 w-3.5" />
                <span>{label}</span>
              </button>
            );
          })}
        </div>
        {trailing ? (
          <div className="ml-4 flex shrink-0 items-center gap-2">
            {trailing}
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="flex w-36 flex-col border-r dialog-divider bg-transparent px-2.5 py-3">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = active_tab === item.key;
        const label = t(item.label_key);
        return (
          <button
            key={item.key}
            onClick={() => on_tab_change(item.key)}
            title={label}
            className={cn(
              "relative flex w-full items-center gap-2.5 rounded-[16px] px-2.5 py-2.5 text-left transition-[color,background] duration-(--motion-duration-normal)",
              isActive
                ? "text-primary shadow-none"
                : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
            )}
          >
            {isActive ? (
              <span
                aria-hidden="true"
                className="absolute inset-0 rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)]"
              />
            ) : null}
            {isActive && (
              <span className="absolute left-0 top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-r-full bg-primary" />
            )}
            <span
              className={cn(
                "relative z-[1] flex h-8 w-8 items-center justify-center rounded-[10px]",
                isActive
                  ? "bg-primary/10 text-primary"
                  : "bg-transparent text-(--icon-default)"
              )}
            >
              <Icon className="h-4 w-4" />
            </span>
            <span className="relative z-[1] text-[13px] font-semibold">{label}</span>
          </button>
        );
      })}
    </div>
  );
}
