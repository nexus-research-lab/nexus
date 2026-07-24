"use client";

import { UserPen, ToolCase, Album, type LucideIcon } from "lucide-react";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiChoiceButton } from "@/shared/ui/form/choice";

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
    <div className="soft-scrollbar flex w-36 shrink-0 flex-col gap-2 overflow-y-auto border-r dialog-divider bg-transparent px-2.5 py-3 max-xl:w-full max-xl:flex-row max-xl:overflow-x-auto max-xl:overflow-y-hidden max-xl:border-r-0 max-xl:border-b max-xl:px-3 max-xl:py-2">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = activeTab === item.key;
        const label = t(item.labelKey);
        return (
          <UiChoiceButton
            active={isActive}
            className="relative w-full justify-start gap-2.5 surface-radius-md px-2.5 py-2.5 text-left max-xl:min-w-[84px] max-xl:flex-1 max-xl:justify-center max-xl:gap-1.5 max-xl:px-2 max-xl:py-1.5"
            choiceSize="lg"
            key={item.key}
            onClick={() => onTabChange(item.key)}
            title={label}
          >
            {isActive && (
              <span className="absolute left-0 top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-r-full bg-primary max-xl:bottom-0 max-xl:left-1/2 max-xl:top-auto max-xl:h-[3px] max-xl:w-6 max-xl:-translate-x-1/2 max-xl:translate-y-0 max-xl:rounded-t-full max-xl:rounded-r-none" />
            )}
            <span
              className={cn(
                "relative z-[1] flex h-8 w-8 items-center justify-center rounded-[10px] max-xl:h-7 max-xl:w-7",
                isActive
                  ? "bg-primary/10 text-primary"
                  : "bg-transparent text-(--icon-default)"
              )}
            >
              <Icon className="h-4 w-4" />
            </span>
            <span className="relative z-[1] text-sm font-semibold">{label}</span>
          </UiChoiceButton>
        );
      })}
    </div>
  );
}
