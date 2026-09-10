/**
 * INPUT: Room 候选成员、可选完整姓名目录、受控选择与业务外观语境。
 * OUTPUT: 共用头像与可区分名称的紧凑切换器；缺项显示不可用，候选或受控选择变化关闭旧菜单。
 * POS: Workspace、Subagent 与 Room 进程共用的成员切换视图。
 */
"use client";

import { useCallback, useMemo, useRef } from "react";
import { Check, ChevronDown } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { cn } from "@/shared/ui/class-name";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { getAgentDisplayName } from "@/lib/agent-display-name";
import {
  buildAgentSelectionOptions,
  includeUnavailableAgentSelection,
} from "@/lib/agent-selection-options";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type { Agent } from "@/types/agent/agent";

interface RoomAgentSwitcherProps {
  ariaLabel?: string;
  variant?: "panel" | "task";
  members: Agent[];
  directory?: readonly Agent[];
  selectedId: string;
  onSelect: (id: string) => void;
  className?: string;
}

export function RoomAgentSwitcher({
  ariaLabel,
  members,
  directory = members,
  selectedId,
  onSelect,
  className,
  variant = "panel",
}: RoomAgentSwitcherProps) {
  const { t } = useI18n();
  // 名称与顺序刷新不撤销正在使用的菜单；候选资格或受控身份变化才终止旧选择。
  const menuScope = JSON.stringify([selectedId, members.map((member) => member.agent_id).sort()]);
  const [isOpen, setIsOpen] = useResettableState(false, menuScope);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const closeMenu = useCallback(() => setIsOpen(false), [setIsOpen]);
  const options = useMemo(
    () => includeUnavailableAgentSelection(buildAgentSelectionOptions(members, t, directory), selectedId, t),
    [directory, members, selectedId, t],
  );
  const membersById = new Map(members.map((member) => [member.agent_id, member]));
  const selectedMember = membersById.get(selectedId);
  const accessibleLabel = ariaLabel ?? t("room.switch_agent");
  const selectedLabel = options.find((option) => option.value === selectedId)?.label ?? accessibleLabel;

  if (!selectedId && members.length === 0) {
    return null;
  }

  const menuItems: UiActionMenuItem[] = options.map((option) => {
    const member = membersById.get(option.value);
    const isActive = option.value === selectedId;
    return {
      active: isActive,
      disabled: option.disabled,
      icon: <UiAgentAvatar aria-hidden="true" avatar={member?.avatar} name={getAgentDisplayName(member?.name, t)} size="xxs" />,
      label: option.label,
      trailing: (
        <Check className={cn(
          "h-3.5 w-3.5 text-(--icon-default) transition-opacity duration-(--motion-duration-fast)",
          isActive ? "opacity-100" : "opacity-0",
        )} />
      ),
      value: option.value,
    };
  });

  return (
    <div
      className={cn(
        "relative min-w-0",
        variant === "panel" ? "w-28 shrink-0" : "w-full max-w-36",
        className,
      )}
      data-room-agent-switcher-variant={variant}
    >
      <UiButton
        ref={triggerRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={selectedId ? t("room.switch_agent_current", {
          label: accessibleLabel,
          name: selectedLabel,
        }) : accessibleLabel}
        className="w-full min-w-0 justify-start"
        disabled={members.length === 0}
        onClick={() => setIsOpen((prev) => !prev)}
        size="xs"
        title={selectedLabel}
        variant="ghost"
      >
        <UiAgentAvatar
          aria-hidden="true"
          avatar={selectedMember?.avatar}
          name={getAgentDisplayName(selectedMember?.name, t)}
          size="xxs"
        />
        <span className="min-w-0 flex-1 truncate text-left leading-normal">
          {selectedLabel}
        </span>
        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
          <ChevronDown className={cn(
            "h-3 w-3 text-(--icon-muted) transition-transform duration-(--motion-duration-fast)",
            isOpen && "rotate-180 text-(--icon-default)",
          )} />
        </span>
      </UiButton>
      <UiActionMenu
        anchorRef={triggerRef}
        ariaLabel={accessibleLabel}
        isOpen={isOpen}
        items={menuItems}
        minWidth={220}
        onClose={closeMenu}
        onSelect={(id) => {
          if (membersById.has(id)) onSelect(id);
        }}
      />
    </div>
  );
}
