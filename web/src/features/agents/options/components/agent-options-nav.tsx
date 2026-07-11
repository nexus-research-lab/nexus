/**
 * AgentOptions 左侧图标导航栏
 *
 * 垂直标签导航，强调配置分组与当前上下文
 */

"use client";

import { type ReactNode } from "react";
import { UserPen, ToolCase, Album, type LucideIcon } from "lucide-react";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiUnderlineTabs } from "@/shared/ui/navigation/tabs";

/** Tab 键值类型 */
export type TabKey = "identity" | "skills" | "advanced";

/** 单个导航项配置 */
interface NavItem {
  key: TabKey;
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
  activeTab: TabKey;
  onTabChange: (tab: TabKey) => void;
  variant?: "sidebar" | "inline";
  trailing?: ReactNode;
}

/** 左侧图标导航栏组件 */
export function AgentOptionsNav({
  activeTab: activeTab,
  onTabChange: onTabChange,
  variant = "sidebar",
  trailing,
}: AgentOptionsNavProps) {
  const { t } = useI18n();

  if (variant === "inline") {
    return (
      <div className="flex h-[41px] min-w-0 items-center justify-between gap-4 border-b dialog-divider px-6">
        <UiUnderlineTabs
          activeValue={activeTab}
          ariaLabel="Agent 配置切换"
          className="-mx-0.5 flex-1 px-0.5"
          itemClassName="h-full"
          onChange={onTabChange}
          options={NAV_ITEMS.map((item) => {
            const label = t(item.labelKey);
            return {
              icon: item.icon,
              label,
              title: label,
              value: item.key,
            };
          })}
        />
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
        const isActive = activeTab === item.key;
        const label = t(item.labelKey);
        return (
          <UiChoiceButton
            active={isActive}
            className="relative w-full justify-start gap-2.5 rounded-[16px] px-2.5 py-2.5 text-left"
            choiceSize="lg"
            key={item.key}
            onClick={() => onTabChange(item.key)}
            title={label}
          >
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
          </UiChoiceButton>
        );
      })}
    </div>
  );
}
