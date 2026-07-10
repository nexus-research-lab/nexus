"use client";

import { memo, useCallback, useEffect, useMemo, useRef } from "react";
import { createPortal } from "react-dom";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/lib/utils";

export interface MentionTargetItem {
    id: string;
    label: string;
    subtitle?: string | null;
    kind: "agent" | "room";
}

interface MentionTargetPopoverProps {
    items: MentionTargetItem[];
    filter: string;
    anchorRect: DOMRect | null;
    onSelect: (item: MentionTargetItem) => void;
    onClose: () => void;
    placement?: "above" | "below" | "auto";
}

/**
 * 通用 mention 目标选择面板
 *
 * 渲染到 document.body，避免被父级的 overflow 和层叠上下文裁切。
 */
export const MentionTargetPopover = memo(({
    items,
    filter,
    anchorRect: anchorRect,
    onSelect: onSelect,
    onClose: onClose,
    placement = "auto",
}: MentionTargetPopoverProps) => {
    const [activeIndex, setActiveIndex] = useResettableState(0, filter);
    const listRef = useRef<HTMLDivElement>(null);

    const normalizedFilter = filter.trim().toLowerCase();
    const filteredItems = useMemo(() => items.filter((item) =>
        item.label.toLowerCase().includes(normalizedFilter)
        || item.subtitle?.toLowerCase().includes(normalizedFilter),
    ), [items, normalizedFilter]);

    useEffect(() => {
        if (filteredItems.length === 0) {
            onClose();
        }
    }, [filteredItems.length, onClose]);

    const handleKeyDown = useCallback((event: KeyboardEvent) => {
        if (filteredItems.length === 0) {
            return;
        }

        switch (event.key) {
            case "ArrowDown":
                event.preventDefault();
                event.stopPropagation();
                setActiveIndex((prev) => (prev + 1) % filteredItems.length);
                break;
            case "ArrowUp":
                event.preventDefault();
                event.stopPropagation();
                setActiveIndex((prev) => (prev - 1 + filteredItems.length) % filteredItems.length);
                break;
            case "Enter":
            case "Tab":
                event.preventDefault();
                event.stopPropagation();
                onSelect(filteredItems[activeIndex]);
                break;
            case "Escape":
                event.preventDefault();
                event.stopPropagation();
                onClose();
                break;
        }
    }, [activeIndex, filteredItems, onClose, onSelect, setActiveIndex]);

    useEffect(() => {
        document.addEventListener("keydown", handleKeyDown, true);
        return () => document.removeEventListener("keydown", handleKeyDown, true);
    }, [handleKeyDown]);

    useEffect(() => {
        const activeElement = listRef.current?.children[activeIndex] as HTMLElement | undefined;
        activeElement?.scrollIntoView({ block: "nearest" });
    }, [activeIndex]);

    if (!anchorRect || filteredItems.length === 0) {
        return null;
    }

    const MAX_HEIGHT = 192;
    const GAP = 6;
    const estimatedHeight = Math.min(filteredItems.length * 52 + 8, MAX_HEIGHT);
    const canPlaceAbove = anchorRect.top - GAP - estimatedHeight >= 12;
    const shouldPlaceBelow = placement === "below" || (placement === "auto" && !canPlaceAbove);
    const top = shouldPlaceBelow
        ? anchorRect.bottom + GAP
        : anchorRect.top - GAP - estimatedHeight;
    const left = anchorRect.left;
    const minWidth = Math.max(anchorRect.width, 200);

    const popover = (
        <div
            className="fixed z-[9999] max-h-48 overflow-y-auto rounded-2xl"
            style={{
                top,
                left,
                minWidth: minWidth,
                background: "var(--surface-popover-background)",
                border: "1px solid var(--surface-popover-border)",
                boxShadow: "var(--surface-popover-shadow)",
            }}
        >
            <div ref={listRef} className="py-1">
                {filteredItems.map((item, index) => (
                    <button
                        key={item.id}
                        className={cn(
                            "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors duration-(--motion-duration-fast)",
                            index === activeIndex ? "text-(--text-strong)" : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                        )}
                        style={index === activeIndex ? { background: "var(--surface-interactive-active-background)" } : undefined}
                        onMouseDown={(e) => {
                            e.preventDefault();
                            onSelect(item);
                        }}
                        onMouseEnter={() => setActiveIndex(index)}
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
