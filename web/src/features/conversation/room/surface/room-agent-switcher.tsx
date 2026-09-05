/**
 * INPUT: Room 成员目录、当前选择与业务外观语境。
 * OUTPUT: 复用共享菜单生命周期的成员身份切换器及行高自适应的 Panel/Task 紧凑触发器。
 * POS: Workspace、Subagent 与 Room 进程共用的成员切换视图。
 */
"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  getIconAvatarSrc,
  getInitials,
} from "@/lib/avatar";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type { Agent } from "@/types/agent/agent";

interface RoomAgentSwitcherProps {
  ariaLabel?: string;
  variant?: "panel" | "task";
  members: Agent[];
  selectedId: string;
  onSelect: (id: string) => void;
  className?: string;
}

export function RoomAgentSwitcher({
  ariaLabel,
  members,
  selectedId,
  onSelect,
  className,
  variant = "panel",
}: RoomAgentSwitcherProps) {
  const { t } = useI18n();
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const closeMenu = useCallback(() => setIsOpen(false), []);
  const selectedMember = useMemo(
    () => members.find((member) => member.agent_id === selectedId) ?? members[0] ?? null,
    [members, selectedId],
  );

  if (!selectedMember) {
    return null;
  }
  const accessibleLabel = ariaLabel ?? t("room.switch_agent");

  const menuItems: UiActionMenuItem[] = members.map((member) => {
    const isActive = member.agent_id === selectedId;
    return {
      active: isActive,
      icon: <RoomAgentAvatar member={member} />,
      label: member.name,
      trailing: (
        <Check className={cn(
          "h-3.5 w-3.5 text-(--success) transition-opacity duration-(--motion-duration-fast)",
          isActive ? "opacity-100" : "opacity-0",
        )} />
      ),
      value: member.agent_id,
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
        aria-label={t("room.switch_agent_current", {
          label: accessibleLabel,
          name: selectedMember.name,
        })}
        className="w-full min-w-0 justify-start"
        onClick={() => setIsOpen((prev) => !prev)}
        size="xs"
        variant="ghost"
      >
        <RoomAgentAvatar
          className="h-4 w-4"
          member={selectedMember}
        />
        <span className="min-w-0 flex-1 truncate text-left leading-normal">
          {selectedMember.name}
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
        onSelect={onSelect}
      />
    </div>
  );
}

function RoomAgentAvatar({
  className,
  member,
}: {
  className?: string;
  member: Agent;
}) {
  const avatarSrc = getIconAvatarSrc(member.avatar);
  return (
    <span className={cn(
      "flex h-4 w-4 shrink-0 items-center justify-center overflow-hidden rounded-[5px] border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
      className,
    )}>
      {avatarSrc ? (
        <img
          alt={member.name}
          className="h-full w-full object-cover"
          src={avatarSrc}
        />
      ) : (
        <span className="text-[8px] font-semibold text-(--text-strong)">
          {getInitials(member.name)}
        </span>
      )}
    </span>
  );
}
