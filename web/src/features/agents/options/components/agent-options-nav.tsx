"use client";

import { UserPen, ToolCase, Album, type LucideIcon } from "lucide-react";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { AgentOptionsTabKey } from "../agent-options-editor-model";

interface NavItem {
  key: AgentOptionsTabKey;
  labelKey:
    | "agent_options.nav.identity"
    | "agent_options.nav.tools"
    | "agent_options.nav.skills";
  icon: LucideIcon;
}

/** 导航栏 Tab 配置列表 */
const NAV_ITEMS: NavItem[] = [
  { key: "identity", labelKey: "agent_options.nav.identity", icon: UserPen },
  { key: "advanced", labelKey: "agent_options.nav.tools", icon: ToolCase },
  { key: "skills", labelKey: "agent_options.nav.skills", icon: Album },
];

interface AgentOptionsNavProps {
  activeTab: AgentOptionsTabKey;
  onTabChange: (tab: AgentOptionsTabKey) => void;
}

export function AgentOptionsNav({
  activeTab,
  onTabChange,
}: AgentOptionsNavProps) {
  const { t } = useI18n();

  return (
    <div className="soft-scrollbar flex w-32 shrink-0 flex-col gap-1 overflow-y-auto border-r dialog-divider bg-transparent p-2 max-xl:w-full max-xl:flex-row max-xl:overflow-x-auto max-xl:overflow-y-hidden max-xl:border-r-0 max-xl:border-b">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = activeTab === item.key;
        const label = t(item.labelKey);
        return (
          <button
            aria-current={isActive ? "page" : undefined}
            className={cn(
              "flex h-9 w-full items-center justify-start gap-2 radius-control-md border border-transparent px-2.5 text-left text-sm font-medium transition-[background,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] max-xl:min-w-[84px] max-xl:flex-1 max-xl:justify-center",
              isActive
                ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
            )}
            key={item.key}
            onClick={() => onTabChange(item.key)}
            title={label}
            type="button"
          >
            <span className="flex h-6 w-6 items-center justify-center">
              <Icon className="h-4 w-4" />
            </span>
            <span>{label}</span>
          </button>
        );
      })}
    </div>
  );
}
