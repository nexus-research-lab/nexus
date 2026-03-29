/**
 * AgentOptions 左侧图标导航栏
 *
 * 垂直图标列表，选中态左边框高亮 + 图标变色
 * 使用 Lucide 图标：Palette（Identity）、Brain（Persona）、Wrench（Skills）、Settings（Advanced）
 */

"use client";

import { Palette, Brain, Wrench, Settings, type LucideIcon } from "lucide-react";
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
  { key: "identity", label: "Identity", icon: Palette },
  { key: "persona", label: "Persona", icon: Brain },
  { key: "skills", label: "Skills", icon: Wrench },
  { key: "advanced", label: "Advanced", icon: Settings },
];

interface AgentOptionsNavProps {
  activeTab: TabKey;
  onTabChange: (tab: TabKey) => void;
}

/** 左侧图标导航栏组件 */
export function AgentOptionsNav({ activeTab, onTabChange }: AgentOptionsNavProps) {
  return (
    <div className="flex w-16 flex-col items-center gap-1 border-r border-white/22 bg-white/8 py-4">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = activeTab === item.key;
        return (
          <button
            key={item.key}
            onClick={() => onTabChange(item.key)}
            title={item.label}
            className={cn(
              "relative flex h-10 w-10 items-center justify-center rounded-lg transition-all duration-200",
              isActive
                ? "text-primary bg-primary/10"
                : "text-muted-foreground hover:bg-white/25 hover:text-foreground"
            )}
          >
            {/* 左边框高亮指示器 */}
            {isActive && (
              <div className="absolute left-0 top-1/2 h-5 w-0.5 -translate-y-1/2 rounded-r-full bg-primary" />
            )}
            <Icon className="h-4 w-4" />
          </button>
        );
      })}
    </div>
  );
}
