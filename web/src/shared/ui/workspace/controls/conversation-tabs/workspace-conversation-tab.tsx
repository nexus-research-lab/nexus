/**
 * INPUT: 单个 Conversation 的活动、固定、关闭状态与独立动作。
 * OUTPUT: 标题、状态点、图钉和关闭按钮组成的单一会话标签。
 * POS: Workspace 会话标签纯视图，不推导集合或持久化状态。
 */
import { Pin, X } from "lucide-react";

import { resolveWorkspaceConversationTabPresentation } from "./workspace-conversation-tab-model";

interface WorkspaceConversationTabProps {
  canClose: boolean;
  canPin: boolean;
  closeLabel: string;
  conversationId: string;
  externalSessionLabel: string | null;
  isActive: boolean;
  isPinned: boolean;
  onClose: () => void;
  onSelect: () => void;
  onTogglePin: () => void;
  pinLabel: string;
  tabWidth?: number;
  title: string;
}

export function WorkspaceConversationTab({
  canClose,
  canPin,
  closeLabel,
  conversationId,
  externalSessionLabel,
  isActive,
  isPinned,
  onClose,
  onSelect,
  onTogglePin,
  pinLabel,
  tabWidth,
  title,
}: WorkspaceConversationTabProps) {
  const presentation = resolveWorkspaceConversationTabPresentation({
    canClose,
    canPin,
    externalSessionLabel,
    isActive,
    isPinned,
    tabWidth,
    title,
  });
  return (
    <div
      className={presentation.rootClassName}
      data-conversation-tab-id={conversationId}
      style={presentation.style}
      title={presentation.title}
    >
      <button
        aria-current={presentation.ariaCurrent}
        aria-pressed={isActive}
        className={presentation.contentClassName}
        onClick={onSelect}
        type="button"
      >
        <span
          aria-hidden="true"
          className={presentation.indicatorClassName}
        />
        <span className="min-w-0 truncate">{title}</span>
        {presentation.showExternalSessionLabel ? (
          <span className="ml-1 inline-flex shrink-0 items-center radius-control-xs border border-[color:color-mix(in_srgb,var(--primary)_20%,transparent)] px-1 py-px text-[8.5px] font-semibold leading-none text-(--primary)">
            IM
          </span>
        ) : null}
      </button>
      {presentation.showPin || presentation.showClose ? (
        <span className={presentation.actionsClassName}>
          {presentation.showPin ? (
            <button
              aria-label={pinLabel}
              aria-pressed={isPinned}
              className={presentation.pinClassName}
              onClick={(event) => {
                event.stopPropagation();
                onTogglePin();
              }}
              title={pinLabel}
              type="button"
            >
              <Pin className={isPinned ? "h-3 w-3 fill-current" : "h-3 w-3"} />
            </button>
          ) : null}
          {presentation.showClose ? (
            <button
              aria-label={closeLabel}
              className={presentation.closeClassName}
              onClick={(event) => {
                event.stopPropagation();
                onClose();
              }}
              title={closeLabel}
              type="button"
            >
              <X className="h-3 w-3" />
            </button>
          ) : null}
        </span>
      ) : null}
    </div>
  );
}
