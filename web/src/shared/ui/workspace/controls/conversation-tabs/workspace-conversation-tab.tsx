import { X } from "lucide-react";

import {
  ACTIVE_TAB_MIN_WIDTH,
  INACTIVE_TAB_MIN_WIDTH,
} from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-model";
import { cn } from "@/shared/ui/class-name";

const TAB_BASE_CLASS_NAME =
  "group relative inline-flex h-6.5 flex-none items-center overflow-hidden rounded-[13px] border text-[11px] font-semibold transition-[width,background-color,border-color,color,box-shadow] duration-[145ms] ease-[cubic-bezier(0.25,0.1,0.25,1)]";

interface WorkspaceConversationTabProps {
  canClose: boolean;
  closeLabel: string;
  externalSessionLabel: string | null;
  isActive: boolean;
  onClose: () => void;
  onHoverChange: (hovered: boolean) => void;
  onPreview: () => void;
  onSelect: () => void;
  showSeparator: boolean;
  tabWidth?: number;
  title: string;
}

export function WorkspaceConversationTab({
  canClose,
  closeLabel,
  externalSessionLabel,
  isActive,
  onClose,
  onHoverChange,
  onPreview,
  onSelect,
  showSeparator,
  tabWidth,
  title,
}: WorkspaceConversationTabProps) {
  return (
    <div
      className={cn(
        TAB_BASE_CLASS_NAME,
        isActive
          ? "z-10 border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color)_82%)] bg-(--surface-interactive-active-background) text-(--text-strong) shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_6%,transparent)] hover:border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color)_78%)] hover:bg-(--surface-interactive-hover-background) hover:shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)]"
          : "border-transparent bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) hover:shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)]",
        showSeparator &&
          "before:pointer-events-none before:absolute before:left-0 before:top-1.5 before:bottom-1.5 before:w-px before:bg-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] before:content-['']",
      )}
      onMouseEnter={() => onHoverChange(true)}
      onMouseLeave={() => onHoverChange(false)}
      style={{
        minWidth: isActive ? ACTIVE_TAB_MIN_WIDTH : INACTIVE_TAB_MIN_WIDTH,
        width: tabWidth ?? (isActive ? ACTIVE_TAB_MIN_WIDTH : INACTIVE_TAB_MIN_WIDTH),
      }}
      title={externalSessionLabel ? `${title} · IM ${externalSessionLabel}` : title}
    >
      <button
        aria-current={isActive ? "page" : undefined}
        aria-pressed={isActive}
        className="flex h-full w-full min-w-0 items-center justify-start pl-[22px] pr-7 text-left"
        onClick={onSelect}
        onPointerDown={(event) => {
          if (event.button === 0) {
            onPreview();
          }
        }}
        type="button"
      >
        <span
          aria-hidden="true"
          className={cn(
            "absolute left-2.5 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast)",
            isActive
              ? "bg-(--primary) shadow-[0_0_0_2px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
              : "border border-[color:color-mix(in_srgb,var(--icon-muted)_72%,transparent)] bg-transparent group-hover:border-(--icon-default) group-hover:bg-[color:color-mix(in_srgb,var(--icon-default)_28%,transparent)]",
          )}
        />
        <span className="min-w-0 truncate">{title}</span>
        {externalSessionLabel ? (
          <span className="ml-1 inline-flex shrink-0 items-center rounded-[5px] border border-[color:color-mix(in_srgb,var(--primary)_20%,transparent)] px-1 py-px text-[8.5px] font-bold leading-none text-(--primary)">
            IM
          </span>
        ) : null}
      </button>
      {canClose ? (
        <button
          aria-label={closeLabel}
          className={cn(
            "absolute right-1 top-1/2 flex h-5 w-5 shrink-0 -translate-y-1/2 items-center justify-center rounded-full text-(--icon-muted) transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)",
            isActive ? "opacity-100" : "opacity-70 group-hover:opacity-100",
          )}
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
