"use client";

import { memo, useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";

interface MentionPopoverProps {
    members: Agent[];
    filter: string;
    anchor_rect: DOMRect | null;
    on_select: (agent: Agent) => void;
    on_close: () => void;
}

/**
 * @mention 下拉选择面板
 *
 * 渲染到 document.body (portal)，避免被父级 overflow:hidden 或 stacking context 截断。
 * 位置用 anchor_rect (fixed 坐标) 计算，始终显示在输入框上方。
 */
export const MentionPopover = memo(({
    members,
    filter,
    anchor_rect,
    on_select,
    on_close,
}: MentionPopoverProps) => {
    const [active_index, set_active_index] = useState(0);
    const list_ref = useRef<HTMLDivElement>(null);

    const filtered_members = members.filter((member) =>
        member.name.toLowerCase().includes(filter.toLowerCase()),
    );

    useEffect(() => {
        set_active_index(0);
    }, [filter]);

    useEffect(() => {
        if (filtered_members.length === 0) {
            on_close();
        }
    }, [filtered_members.length, on_close]);

    const handle_key_down = useCallback((event: KeyboardEvent) => {
        if (filtered_members.length === 0) {
            return;
        }

        switch (event.key) {
            case "ArrowDown":
                event.preventDefault();
                event.stopPropagation();
                set_active_index((prev) => (prev + 1) % filtered_members.length);
                break;
            case "ArrowUp":
                event.preventDefault();
                event.stopPropagation();
                set_active_index((prev) => (prev - 1 + filtered_members.length) % filtered_members.length);
                break;
            case "Enter":
            case "Tab":
                event.preventDefault();
                event.stopPropagation();
                on_select(filtered_members[active_index]);
                break;
            case "Escape":
                event.preventDefault();
                event.stopPropagation();
                on_close();
                break;
        }
    }, [active_index, filtered_members, on_close, on_select]);

    useEffect(() => {
        document.addEventListener("keydown", handle_key_down, true);
        return () => document.removeEventListener("keydown", handle_key_down, true);
    }, [handle_key_down]);

    useEffect(() => {
        const active_element = list_ref.current?.children[active_index] as HTMLElement | undefined;
        active_element?.scrollIntoView({ block: "nearest" });
    }, [active_index]);

    if (!anchor_rect || filtered_members.length === 0) {
        return null;
    }

    // Position the popover above the textarea using fixed coordinates.
    // Max height is capped at 192px (max-h-48); we prefer top-anchored-above.
    const MAX_HEIGHT = 192;
    const GAP = 6;
    const top = anchor_rect.top - GAP - Math.min(filtered_members.length * 40 + 8, MAX_HEIGHT);
    const left = anchor_rect.left;
    const min_width = Math.max(anchor_rect.width, 200);

    const popover = (
        <div
            className="fixed z-[9999] max-h-48 overflow-y-auto rounded-2xl backdrop-blur-[18px]"
            style={{
                top,
                left,
                minWidth: min_width,
                background: "var(--surface-popover-background)",
                border: "1px solid var(--surface-popover-border)",
                boxShadow: "var(--surface-popover-shadow)",
            }}
        >
            <div ref={list_ref} className="py-1">
                {filtered_members.map((member, index) => (
                    <button
                        key={member.agent_id}
                        className={cn(
                            "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors duration-150",
                            index === active_index ? "text-slate-900/94" : "text-slate-600/90 hover:bg-[var(--surface-interactive-hover-background)]",
                        )}
                        style={index === active_index ? { background: "var(--surface-interactive-active-background)" } : undefined}
                        onMouseDown={(e) => {
                            e.preventDefault();
                            on_select(member);
                        }}
                        onMouseEnter={() => set_active_index(index)}
                        type="button"
                    >
                        <span
                            className="flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
                            style={{
                                background: "var(--surface-avatar-background)",
                                color: "var(--surface-avatar-foreground)",
                                boxShadow: "var(--surface-avatar-shadow)",
                            }}
                        >
                            {member.name.charAt(0).toUpperCase()}
                        </span>
                        <span className="truncate font-medium">{member.name}</span>
                    </button>
                ))}
            </div>
        </div>
    );

    return createPortal(popover, document.body);
});

MentionPopover.displayName = "MentionPopover";
