// INPUT: Agent Options 当前栏目、可用栏目定义与切换命令。
// OUTPUT: 桌面纵向、窄窗横向且由共享 Button 持 active 状态的导航。
// POS: Agent Options 弹窗导航 Pattern；不拥有栏目内容或保存事务。

"use client";

import { UserPen, ToolCase, Album, type LucideIcon } from "lucide-react";
import { UiButton } from "@/shared/ui/button/button";
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
          <UiButton
            aria-current={isActive ? "page" : undefined}
            className="h-9 w-full justify-start gap-2 px-2.5 text-left max-xl:min-w-[84px] max-xl:flex-1 max-xl:justify-center"
            key={item.key}
            onClick={() => onTabChange(item.key)}
            size="md"
            title={label}
            variant="ghost"
          >
            <span className="flex h-6 w-6 items-center justify-center">
              <Icon className="h-4 w-4" />
            </span>
            <span>{label}</span>
          </UiButton>
        );
      })}
    </div>
  );
}
