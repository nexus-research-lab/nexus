"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent/agent";

export interface MentionTargetItem {
    id: string;
    label: string;
    subtitle?: string | null;
    kind: "agent" | "room";
}

interface MentionTargetPopoverProps {
    items: MentionTargetItem[];
    filter: string;
    anchor_rect: DOMRect | null;
    on_select: (item: MentionTargetItem) => void;
    on_close: () => void;
    placement?: "above" | "below" | "auto";
}

interface MentionPopoverProps {
    members: Agent[];
    filter: string;
    anchor_rect: DOMRect | null;
    on_select: (agent: Agent) => void;
    on_close: () => void;
}

/**
 * 通用 mention 目标选择面板
 *
 * 渲染到 document.body，避免被父级的 overflow 和层叠上下文裁切。
 */
export const MentionTargetPopover = memo(({
    items,
    filter,
    anchor_rect,
    on_select,
    on_close,
    placement = "auto",
}: MentionTargetPopoverProps) => {
    const [active_index, set_active_index] = useState(0);
    const list_ref = useRef<HTMLDivElement>(null);

    const normalized_filter = filter.trim().toLowerCase();
    const filtered_items = useMemo(() => items.filter((item) =>
        item.label.toLowerCase().includes(normalized_filter)
        || item.subtitle?.toLowerCase().includes(normalized_filter),
    ), [items, normalized_filter]);

    useEffect(() => {
        set_active_index(0);
    }, [filter]);

    useEffect(() => {
        if (filtered_items.length === 0) {
            on_close();
        }
    }, [filtered_items.length, on_close]);

    const handle_key_down = useCallback((event: KeyboardEvent) => {
        if (filtered_items.length === 0) {
            return;
        }

        switch (event.key) {
            case "ArrowDown":
                event.preventDefault();
                event.stopPropagation();
                set_active_index((prev) => (prev + 1) % filtered_items.length);
                break;
            case "ArrowUp":
                event.preventDefault();
                event.stopPropagation();
                set_active_index((prev) => (prev - 1 + filtered_items.length) % filtered_items.length);
                break;
            case "Enter":
            case "Tab":
                event.preventDefault();
                event.stopPropagation();
                on_select(filtered_items[active_index]);
                break;
            case "Escape":
                event.preventDefault();
                event.stopPropagation();
                on_close();
                break;
        }
    }, [active_index, filtered_items, on_close, on_select]);

    useEffect(() => {
        document.addEventListener("keydown", handle_key_down, true);
        return () => document.removeEventListener("keydown", handle_key_down, true);
    }, [handle_key_down]);

    useEffect(() => {
        const active_element = list_ref.current?.children[active_index] as HTMLElement | undefined;
        active_element?.scrollIntoView({ block: "nearest" });
    }, [active_index]);

    if (!anchor_rect || filtered_items.length === 0) {
        return null;
    }

    const MAX_HEIGHT = 192;
    const GAP = 6;
    const estimated_height = Math.min(filtered_items.length * 52 + 8, MAX_HEIGHT);
    const can_place_above = anchor_rect.top - GAP - estimated_height >= 12;
    const should_place_below = placement === "below" || (placement === "auto" && !can_place_above);
    const top = should_place_below
        ? anchor_rect.bottom + GAP
        : anchor_rect.top - GAP - estimated_height;
    const left = anchor_rect.left;
    const min_width = Math.max(anchor_rect.width, 200);

    const popover = (
        <div
            className="fixed z-[9999] max-h-48 overflow-y-auto rounded-2xl"
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
                {filtered_items.map((item, index) => (
                    <button
                        key={item.id}
                        className={cn(
                            "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors duration-(--motion-duration-fast)",
                            index === active_index ? "text-(--text-strong)" : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                        )}
                        style={index === active_index ? { background: "var(--surface-interactive-active-background)" } : undefined}
                        onMouseDown={(e) => {
                            e.preventDefault();
                            on_select(item);
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
                            {item.kind === "room" ? "#" : item.label.charAt(0).toUpperCase()}
                        </span>
                        <span className="min-w-0 flex-1">
                            <span className="block truncate font-medium">{item.label}</span>
                            {item.subtitle ? (
                                <span className="block truncate text-[11px] text-(--text-soft)">
                                    {item.subtitle}
                                </span>
                            ) : null}
                        </span>
                    </button>
                ))}
            </div>
        </div>
    );

    return createPortal(popover, document.body);
});

MentionTargetPopover.displayName = "MentionTargetPopover";

/**
 * Room composer 仍然只需要 @agent 选择，这里保留兼容包装层。
 */
export const MentionPopover = memo(({
    members,
    filter,
    anchor_rect,
    on_select,
    on_close,
}: MentionPopoverProps) => {
    const items = useMemo<MentionTargetItem[]>(() => members.map((member) => ({
        id: member.agent_id,
        label: member.name,
        subtitle: null,
        kind: "agent",
    })), [members]);

    return (
        <MentionTargetPopover
            anchor_rect={anchor_rect}
            filter={filter}
            items={items}
            on_close={on_close}
            on_select={(item) => {
                const selected_member = members.find((member) => member.agent_id === item.id);
                if (selected_member) {
                    on_select(selected_member);
                }
            }}
            placement="above"
        />
    );
});

MentionPopover.displayName = "MentionPopover";
