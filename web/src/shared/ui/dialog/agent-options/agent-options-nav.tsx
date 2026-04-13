/**
 * AgentOptions 左侧图标导航栏
 *
 * 垂直标签导航，强调配置分组与当前上下文
 */

"use client";

import { UserPen, Brain, ToolCase, Album, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

/** Tab 键值类型 */
export type TabKey = "identity" | "persona" | "skills" | "advanced";

/** 单个导航项配置 */
interface NavItem {
  key: TabKey;
  label: string;
  icon: LucideIcon;
}

/** 导航栏 Tab 配置列表 */
const NAV_ITEMS: NavItem[] = [
  { key: "identity", label: "Identity", icon: UserPen },
  { key: "persona", label: "Persona", icon: Brain },
  { key: "advanced", label: "Tools", icon: ToolCase },
  { key: "skills", label: "Skills", icon: Album },
];

interface AgentOptionsNavProps {
  activeTab: TabKey;
  onTabChange: (tab: TabKey) => void;
}

/** 左侧图标导航栏组件 */
export function AgentOptionsNav({ activeTab, onTabChange }: AgentOptionsNavProps) {
  return (
    <div className="flex w-36 flex-col border-r dialog-divider bg-transparent px-2.5 py-3">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = activeTab === item.key;
        return (
          <button
            key={item.key}
            onClick={() => onTabChange(item.key)}
            title={item.label}
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
            <span className="relative z-[1] text-[13px] font-semibold">{item.label}</span>
          </button>
        );
      })}
    </div>
  );
}
