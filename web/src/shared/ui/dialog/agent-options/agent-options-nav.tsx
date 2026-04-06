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
    <div className="flex w-44 flex-col border-r dialog-divider bg-transparent px-3 py-4">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = activeTab === item.key;
        return (
          <button
            key={item.key}
            onClick={() => onTabChange(item.key)}
            title={item.label}
            className={cn(
              "relative flex w-full items-center gap-3 rounded-[18px] px-3 py-3 text-left transition-all duration-200",
              isActive
                ? "surface-card border-white/46 text-primary shadow-none"
                : "text-slate-500 hover:bg-white/38 hover:text-slate-900"
            )}
          >
            {isActive && (
              <span className="absolute left-0 top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-r-full bg-primary" />
            )}
            <span
              className={cn(
                "flex h-9 w-9 items-center justify-center rounded-[12px]",
                isActive
                  ? "bg-primary/10 text-primary"
                  : "bg-transparent text-slate-600"
              )}
            >
              <Icon className="h-4 w-4" />
            </span>
            <span className="text-sm font-semibold">{item.label}</span>
          </button>
        );
      })}
    </div>
  );
}
