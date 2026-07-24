import { X } from "lucide-react";

import { resolveWorkspaceConversationTabPresentation } from "./workspace-conversation-tab-model";

interface WorkspaceConversationTabProps {
  canClose: boolean;
  closeLabel: string;
  conversationId: string;
  externalSessionLabel: string | null;
  isActive: boolean;
  onClose: () => void;
  onSelect: () => void;
  tabWidth?: number;
  title: string;
}

export function WorkspaceConversationTab({
  canClose,
  closeLabel,
  conversationId,
  externalSessionLabel,
  isActive,
  onClose,
  onSelect,
  tabWidth,
  title,
}: WorkspaceConversationTabProps) {
  const presentation = resolveWorkspaceConversationTabPresentation({
    canClose,
    externalSessionLabel,
    isActive,
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
        className="flex h-full w-full min-w-0 items-center justify-start pl-[22px] pr-7 text-left"
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
    </div>
  );
}
