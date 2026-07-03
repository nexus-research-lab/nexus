/**
 * =====================================================
 * @File   : room-agent-switcher.tsx
 * @Date   : 2026-04-15 18:48
 * @Author : leemysw
 * 2026-04-15 18:48   Create
 * =====================================================
 */

"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";

import { getIconAvatarSrc, getInitials } from "@/lib/utils";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent/agent";

interface RoomAgentSwitcherProps {
  members: Agent[];
  selectedId: string;
  onSelect: (id: string) => void;
  className?: string;
}

/**
 * 会话成员切换器
 *
 * 中文注释：这里直接复用会话切换器的交互形态，统一 header 左侧的切换体验。
 */
export function RoomAgentSwitcher({
  members,
  selectedId: selectedId,
  onSelect: onSelect,
  className: className,
}: RoomAgentSwitcherProps) {
  const [isOpen, setIsOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const selectedMember = useMemo(
    () => members.find((member) => member.agent_id === selectedId) ?? members[0] ?? null,
    [members, selectedId],
  );

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && rootRef.current?.contains(target)) {
        return;
      }
      setIsOpen(false);
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    return () => document.removeEventListener("pointerdown", handlePointerDown, true);
  }, [isOpen]);

  if (!selectedMember) {
    return null;
  }

  const selectedAvatarSrc = getIconAvatarSrc(selectedMember.avatar);

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <button
        aria-expanded={isOpen}
        className="flex max-w-[168px] items-center gap-1 border-b px-0 pb-0.5 text-[12px] text-(--text-default) transition-[border-color,color] duration-(--motion-duration-fast)"
        style={isOpen
          ? { borderBottom: "1px solid var(--surface-popover-border)" }
          : { borderBottom: "1px solid color-mix(in srgb, var(--divider-subtle-color) 82%, transparent)" }}
        onClick={() => setIsOpen((prev) => !prev)}
        type="button"
      >
        <span className="flex h-4.5 w-4.5 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)">
          {selectedAvatarSrc ? (
            <img
              alt={selectedMember.name}
              className="h-full w-full object-cover"
              src={selectedAvatarSrc}
            />
          ) : (
            <span className="text-[8px] font-bold text-(--text-strong)">
              {getInitials(selectedMember.name)}
            </span>
          )}
        </span>
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

      {isOpen ? (
        <div className="surface-panel surface-radius-lg absolute left-0 top-[calc(100%+8px)] z-50 w-[min(18.5rem,calc(100vw-24px))] overflow-hidden">
          <div className="p-1.5">
            {members.map((member) => {
              const isActive = member.agent_id === selectedId;
              const avatarSrc = getIconAvatarSrc(member.avatar);

              return (
                <button
                  aria-pressed={isActive}
                  key={member.agent_id}
                  className={cn(
                    "group flex w-full items-center gap-2.5 rounded-[14px] border px-3.5 py-2.5 text-left text-[11.5px] font-medium transition-[background-color,border-color,color,opacity] duration-(--motion-duration-fast) ease-out",
                    isActive
                      ? "bg-(--surface-interactive-active-background) text-(--text-strong) hover:brightness-[0.985]"
                      : "border-transparent text-(--text-default) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  )}
                  onClick={() => {
                    onSelect(member.agent_id);
                    setIsOpen(false);
                  }}
                  type="button"
                >
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)">
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
                  <span className="min-w-0 flex-1 truncate">
                    {member.name}
                  </span>
                  <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
                    <Check className={cn(
                      "h-3.5 w-3.5 text-(--success) transition-opacity duration-(--motion-duration-fast)",
                      isActive ? "opacity-100" : "opacity-0",
                    )} />
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      ) : null}
    </div>
  );
}
