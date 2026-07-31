/**
 * INPUT: Room 成员目录、当前选择与业务外观语境。
 * OUTPUT: 复用共享菜单生命周期的成员身份切换器及默认/Task 紧凑触发器。
 * POS: Workspace、Subagent 与 Room 进程共用的成员切换视图。
 */
"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";

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
  variant?: "default" | "task";
  members: Agent[];
  selectedId: string;
  onSelect: (id: string) => void;
  className?: string;
}

export function RoomAgentSwitcher({
  ariaLabel = "切换 Agent",
  members,
  selectedId,
  onSelect,
  className,
  variant = "default",
}: RoomAgentSwitcherProps) {
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
      className={cn("relative min-w-0", className)}
      data-room-agent-switcher-variant={variant}
    >
      <button
        ref={triggerRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={`${ariaLabel}：${selectedMember.name}`}
        className={cn(
          "flex min-w-0 items-center gap-1 text-compact transition-[background,border-color,color] duration-(--motion-duration-fast) focus-visible:outline-none",
          variant === "task"
            ? "h-7 w-full max-w-[9rem] rounded-[7px] px-1.5 font-semibold leading-none text-(--text-strong) hover:bg-(--surface-interactive-hover-background) focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"
            : "max-w-[168px] border-b px-0 pb-0.5 text-(--text-default)",
          variant === "task"
            && isOpen
            && "bg-(--surface-interactive-active-background)",
        )}
        style={variant === "default"
          ? isOpen
            ? { borderBottom: "1px solid var(--surface-popover-border)" }
            : { borderBottom: "1px solid color-mix(in srgb, var(--divider-subtle-color) 82%, transparent)" }
          : undefined}
        onClick={() => setIsOpen((prev) => !prev)}
        type="button"
      >
        <RoomAgentAvatar
          className={variant === "task" ? "h-4 w-4" : "h-4.5 w-4.5"}
          member={selectedMember}
        />
        <span className={cn(
          "truncate",
          variant === "task"
            ? "min-w-0 max-w-[104px] text-compact font-semibold leading-none"
            : "max-w-[120px] font-medium",
        )}>
          {selectedMember.name}
        </span>
        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
          <ChevronDown className={cn(
            "h-3 w-3 text-(--icon-muted) transition-transform duration-(--motion-duration-fast)",
            isOpen && "rotate-180 text-(--icon-default)",
          )} />
        </span>
      </button>
      <UiActionMenu
        anchorRef={triggerRef}
        ariaLabel={ariaLabel}
        isOpen={isOpen}
        items={menuItems}
        minWidth={variant === "task" ? 220 : 296}
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
