"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";

import { cn, getIconAvatarSrc, getInitials } from "@/lib/utils";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type { Agent } from "@/types/agent/agent";

interface RoomAgentSwitcherProps {
  members: Agent[];
  selectedId: string;
  onSelect: (id: string) => void;
  className?: string;
}

export function RoomAgentSwitcher({
  members,
  selectedId,
  onSelect,
  className,
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
    <div className={cn("relative", className)}>
      <button
        ref={triggerRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        className="flex max-w-[168px] items-center gap-1 border-b px-0 pb-0.5 text-[12px] text-(--text-default) transition-[border-color,color] duration-(--motion-duration-fast)"
        style={isOpen
          ? { borderBottom: "1px solid var(--surface-popover-border)" }
          : { borderBottom: "1px solid color-mix(in srgb, var(--divider-subtle-color) 82%, transparent)" }}
        onClick={() => setIsOpen((prev) => !prev)}
        type="button"
      >
        <RoomAgentAvatar className="h-4.5 w-4.5" member={selectedMember} />
        <span className="max-w-[120px] truncate font-medium">
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
        ariaLabel="切换 Agent"
        isOpen={isOpen}
        items={menuItems}
        minWidth={296}
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
      "flex h-4 w-4 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
      className,
    )}>
      {avatarSrc ? (
        <img
          alt={member.name}
          className="h-full w-full object-cover"
          src={avatarSrc}
        />
      ) : (
        <span className="text-[8px] font-bold text-(--text-strong)">
          {getInitials(member.name)}
        </span>
      )}
    </span>
  );
}
