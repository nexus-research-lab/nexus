/**
 * INPUT: 上层投影的固定会话、活动态与导航/排序/取消固定动作。
 * OUTPUT: 能力入口下方带分割线、拖放落点、边缘滚动与独立取消位的固定会话 Dock。
 * POS: 主侧栏导航轨的固定会话纯视图，不读取 Store、路由或业务 API。
 */
import { MessageSquareText, X } from "lucide-react";
import { useRef, useState, type DragEvent } from "react";

import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

import { resolveSidebarPinnedConversationDropPlacement } from "./sidebar-pinned-conversations-model";
import type {
  SidebarPinnedConversationItem,
  SidebarPinnedConversationPlacement,
} from "./sidebar-wide-panel-types";

interface PinnedConversationDropTarget {
  key: string;
  placement: SidebarPinnedConversationPlacement;
}

interface SidebarPinnedConversationsProps {
  items: SidebarPinnedConversationItem[];
  label: string;
  onReorder: (
    source: SidebarPinnedConversationItem,
    target: SidebarPinnedConversationItem,
    placement: SidebarPinnedConversationPlacement,
  ) => void;
  onSelect: (item: SidebarPinnedConversationItem) => void;
  onUnpin: (item: SidebarPinnedConversationItem) => void;
  reorderLabel: string;
  unpinLabel: string;
}

const DRAG_EDGE_SCROLL_THRESHOLD = 28;
const DRAG_EDGE_SCROLL_STEP = 12;

export function SidebarPinnedConversations({
  items,
  label,
  onReorder,
  onSelect,
  onUnpin,
  reorderLabel,
  unpinLabel,
}: SidebarPinnedConversationsProps) {
  const draggingKeyRef = useRef<string | null>(null);
  const [draggingKey, setDraggingKey] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<PinnedConversationDropTarget | null>(null);

  if (items.length === 0) {
    return null;
  }

  const resetDragState = () => {
    draggingKeyRef.current = null;
    setDraggingKey(null);
    setDropTarget(null);
  };

  const handleDragStart = (
    event: DragEvent<HTMLButtonElement>,
    item: SidebarPinnedConversationItem,
  ) => {
    draggingKeyRef.current = item.key;
    setDraggingKey(item.key);
    setDropTarget(null);
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", item.key);
  };

  const handleItemDragOver = (
    event: DragEvent<HTMLDivElement>,
    item: SidebarPinnedConversationItem,
  ) => {
    const sourceKey = draggingKeyRef.current;
    if (!sourceKey || sourceKey === item.key) {
      setDropTarget(null);
      return;
    }
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    const bounds = event.currentTarget.getBoundingClientRect();
    const placement = resolveSidebarPinnedConversationDropPlacement(
      event.clientY,
      bounds.top,
      bounds.height,
    );
    setDropTarget((current) => (
      current?.key === item.key && current.placement === placement
        ? current
        : {key: item.key, placement}
    ));
  };

  const handleItemDrop = (
    event: DragEvent<HTMLDivElement>,
    target: SidebarPinnedConversationItem,
  ) => {
    event.preventDefault();
    event.stopPropagation();
    const source = items.find((item) => item.key === draggingKeyRef.current);
    if (source && source.key !== target.key) {
      const bounds = event.currentTarget.getBoundingClientRect();
      onReorder(
        source,
        target,
        resolveSidebarPinnedConversationDropPlacement(
          event.clientY,
          bounds.top,
          bounds.height,
        ),
      );
    }
    resetDragState();
  };

  const handleScrollAreaDragOver = (event: DragEvent<HTMLElement>) => {
    if (!draggingKeyRef.current) {
      return;
    }
    event.preventDefault();
    const bounds = event.currentTarget.getBoundingClientRect();
    if (event.clientY < bounds.top + DRAG_EDGE_SCROLL_THRESHOLD) {
      event.currentTarget.scrollTop -= DRAG_EDGE_SCROLL_STEP;
    } else if (event.clientY > bounds.bottom - DRAG_EDGE_SCROLL_THRESHOLD) {
      event.currentTarget.scrollTop += DRAG_EDGE_SCROLL_STEP;
    }
  };

  return (
    <section
      aria-label={label}
      className="soft-scrollbar min-h-0 w-full flex-1 overflow-y-auto overscroll-contain px-1 pb-2"
      data-pinned-conversation-scroll-region="true"
      onDragOver={handleScrollAreaDragOver}
      onDrop={resetDragState}
    >
      <div className="mx-auto flex w-14 flex-col items-center gap-2 pt-3">
        <div
          aria-hidden="true"
          className="h-px w-10 shrink-0 bg-(--divider-subtle-color)"
          data-sidebar-rail-divider
        />
        {items.map((item) => (
          <div
            className={cn(
              "group/pinned relative h-14 w-14 shrink-0 transition-colors duration-(--motion-duration-fast)",
              item.active
                ? "text-(--text-strong)"
                : "text-(--text-muted)",
              draggingKey === item.key && "opacity-45",
            )}
            data-pinned-conversation-dragging={
              draggingKey === item.key ? "true" : undefined
            }
            data-pinned-conversation-drop-position={
              dropTarget?.key === item.key ? dropTarget.placement : undefined
            }
            data-pinned-conversation-id={item.conversationId}
            key={item.key}
            onDragOver={(event) => handleItemDragOver(event, item)}
            onDrop={(event) => handleItemDrop(event, item)}
          >
            {dropTarget?.key === item.key ? (
              <span
                aria-hidden="true"
                className={cn(
                  "pointer-events-none absolute left-2 right-2 z-20 h-0.5 rounded-full bg-(--primary)",
                  dropTarget.placement === "before" ? "-top-1" : "-bottom-1",
                )}
              />
            ) : null}
            <button
              aria-current={item.active ? "page" : undefined}
              className="absolute inset-0 min-w-0 cursor-grab rounded-[12px] text-2xs font-medium transition-colors duration-(--motion-duration-fast) active:cursor-grabbing hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_34%,transparent)]"
              draggable
              onDragEnd={resetDragState}
              onDragStart={(event) => handleDragStart(event, item)}
              onClick={() => onSelect(item)}
              title={`${item.title} · ${reorderLabel}`}
              type="button"
            >
              <span className={cn(
                "absolute left-1/2 top-0 flex h-8 w-8 -translate-x-1/2 items-center justify-center rounded-[10px] transition-[background,color] duration-(--motion-duration-fast)",
                item.active
                  ? SIDEBAR_SELECTION_CLASS_NAME
                  : "group-hover/pinned:bg-(--surface-interactive-hover-background)",
              )}>
                <MessageSquareText className="h-[17px] w-[17px]" />
              </span>
              <span className="absolute inset-x-0 bottom-2 block truncate px-1 text-center leading-tight">
                {item.title}
              </span>
              <span className="sr-only">{reorderLabel}</span>
            </button>
            <UiIconButton
              aria-label={`${unpinLabel}：${item.title}`}
              className="absolute -right-1 -top-1 z-10 opacity-0 focus-visible:opacity-100 group-focus-within/pinned:opacity-100 group-hover/pinned:opacity-100"
              data-pinned-conversation-unpin="true"
              onClick={() => onUnpin(item)}
              shape="round"
              size="xs"
              title={unpinLabel}
              variant="ghost"
            >
              <X className="h-3.5 w-3.5" />
            </UiIconButton>
          </div>
        ))}
      </div>
    </section>
  );
}
