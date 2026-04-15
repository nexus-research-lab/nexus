/**
 * =====================================================
 * @File   : room-agent-switcher.tsx
 * @Date   : 2026-04-15 18:48
 * @Author : leemysw
 * 2026-04-15 18:48   Create
 * =====================================================
 */

"use client";

import { useMemo, useState } from "react";
import { Check, ChevronDown } from "lucide-react";

import { get_icon_avatar_src, get_initials } from "@/lib/utils";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";

interface RoomAgentSwitcherProps {
  members: Agent[];
  selected_id: string;
  on_select: (id: string) => void;
  class_name?: string;
}

/**
 * 房间成员切换器
 *
 * 中文注释：这里直接复用会话切换器的交互形态，统一 header 左侧的切换体验。
 */
export function RoomAgentSwitcher({
  members,
  selected_id,
  on_select,
  class_name,
}: RoomAgentSwitcherProps) {
  const [is_open, set_is_open] = useState(false);
  const selected_member = useMemo(
    () => members.find((member) => member.agent_id === selected_id) ?? members[0] ?? null,
    [members, selected_id],
  );

  if (!selected_member) {
    return null;
  }

  const selected_avatar_src = get_icon_avatar_src(selected_member.avatar);

  return (
    <div className={cn("relative", class_name)}>
      <button
        aria-expanded={is_open}
        className="flex max-w-[168px] items-center gap-1 border-b px-0 pb-0.5 text-[12px] text-(--text-default) transition-[border-color,color] duration-(--motion-duration-fast)"
        style={is_open
          ? { borderBottom: "1px solid var(--surface-popover-border)" }
          : { borderBottom: "1px solid color-mix(in srgb, var(--divider-subtle-color) 82%, transparent)" }}
        onClick={() => set_is_open((prev) => !prev)}
        type="button"
      >
        <span className="flex h-4.5 w-4.5 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)">
          {selected_avatar_src ? (
            <img
              alt={selected_member.name}
              className="h-full w-full object-cover"
              src={selected_avatar_src}
            />
          ) : (
            <span className="text-[8px] font-bold text-(--text-strong)">
              {get_initials(selected_member.name)}
            </span>
          )}
        </span>
        <span className="max-w-[120px] truncate font-medium">
          {selected_member.name}
        </span>
        <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
          <ChevronDown className={cn(
            "h-3 w-3 text-(--icon-muted) transition-transform duration-(--motion-duration-fast)",
            is_open && "rotate-180 text-(--icon-default)",
          )} />
        </span>
      </button>

      {is_open ? (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => set_is_open(false)}
          />
          <div className="surface-panel radius-shell-lg absolute left-0 top-[calc(100%+8px)] z-50 w-[min(18.5rem,calc(100vw-24px))] overflow-hidden">
            <div className="p-1.5">
              {members.map((member) => {
                const is_active = member.agent_id === selected_id;
                const avatar_src = get_icon_avatar_src(member.avatar);

                return (
                  <button
                    aria-pressed={is_active}
                    key={member.agent_id}
                    className={cn(
                      "group flex w-full items-center gap-2.5 rounded-[14px] border px-3.5 py-2.5 text-left text-[11.5px] font-medium transition-[background-color,border-color,color,opacity] duration-(--motion-duration-fast) ease-out",
                      is_active
                        ? "bg-(--surface-interactive-active-background) text-(--text-strong) hover:brightness-[0.985]"
                        : "border-transparent text-(--text-default) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                    )}
                    onClick={() => {
                      on_select(member.agent_id);
                      set_is_open(false);
                    }}
                    type="button"
                  >
                    <span className="flex h-5 w-5 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)">
                      {avatar_src ? (
                        <img
                          alt={member.name}
                          className="h-full w-full object-cover"
                          src={avatar_src}
                        />
                      ) : (
                        <span className="text-[8px] font-bold text-(--text-strong)">
                          {get_initials(member.name)}
                        </span>
                      )}
                    </span>
                    <span className="min-w-0 flex-1 truncate">
                      {member.name}
                    </span>
                    <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center">
                      <Check className={cn(
                        "h-3.5 w-3.5 text-(--success) transition-opacity duration-(--motion-duration-fast)",
                        is_active ? "opacity-100" : "opacity-0",
                      )} />
                    </span>
                  </button>
                );
              })}
            </div>
          </div>
        </>
      ) : null}
    </div>
  );
}
