import {
  BookOpenText,
  Check,
  CircleCheck,
  Copy,
  GitFork,
  LoaderCircle,
  type LucideIcon,
} from "lucide-react";
import {
  useCallback,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import type {
  GoalCompletionReceipt,
  RecalledMemoryReference,
} from "@/types/conversation/message/entity";

import type { AssistantFooterStats } from "./assistant-message-model";
import { buildGoalCompletionReceiptItems } from "./goal-completion-receipt";

interface CopyActionPresentation {
  className?: string;
  icon: LucideIcon;
}

const COPY_ACTION_PRESENTATION: Record<"copied" | "idle", CopyActionPresentation> = {
  copied: { className: "text-(--success)", icon: Check },
  idle: { icon: Copy },
};

export function AssistantMessageStats({
  copied,
  goalCompletionReceipt,
  memories = [],
  model,
  onCopy,
  onFork,
  stats,
}: {
  copied: boolean;
  goalCompletionReceipt: GoalCompletionReceipt | null;
  memories: RecalledMemoryReference[];
  model?: string;
  onCopy?: () => Promise<void>;
  onFork?: () => Promise<void>;
  stats: AssistantFooterStats | null;
}) {
  const { locale, t } = useI18n();
  const statsItems = [
    stats?.duration,
    stats?.tokens,
    stats?.cost,
    stats?.cacheReadTokens
      ? t("message.cache_read", { count: stats.cacheReadTokens })
      : null,
  ].filter((item): item is string => Boolean(item));
  const modelName = model?.trim() || null;
  const hasSecondaryMetadata = statsItems.length > 0 || Boolean(modelName);
  const receiptItems = goalCompletionReceipt
    ? buildGoalCompletionReceiptItems(goalCompletionReceipt, locale, t)
    : [];

  return (
    <div className="nexus-chat-message-stats min-w-0 pt-1.5 text-xs text-(--text-muted)">
      {receiptItems.length > 0 ? (
        <div
          className="flex min-w-0 items-center gap-1.5 leading-none text-[color:color-mix(in_srgb,var(--success)_82%,var(--text-default)_18%)]"
          data-goal-completion-receipt
        >
          <CircleCheck aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
          <div className="flex min-w-0 flex-nowrap items-center gap-x-1.5 overflow-hidden whitespace-nowrap font-medium">
            {receiptItems.map((item, index) => (
              <span className="contents" key={`${item}-${index}`}>
                {index > 0 ? <span aria-hidden="true">·</span> : null}
                <span className="truncate tabular-nums">{item}</span>
              </span>
            ))}
          </div>
          {!hasSecondaryMetadata ? (
            <AssistantStatsTrailing
              copied={copied}
              memories={memories}
              onCopy={onCopy}
              onFork={onFork}
            />
          ) : null}
        </div>
      ) : null}

      {hasSecondaryMetadata || receiptItems.length === 0 ? (
        <div className={cn(
          "flex min-w-0 items-center justify-between gap-3",
          receiptItems.length > 0 && "pt-1.5",
        )}>
          <div className="flex min-w-0 flex-1 items-center">
            {statsItems.length > 0 ? (
              <div className="nexus-chat-message-stat-list flex min-w-0 flex-nowrap items-center gap-x-1.5 overflow-hidden whitespace-nowrap leading-none">
                {statsItems.map((item, index) => (
                  <span className="contents" key={`${item}-${index}`}>
                    {index > 0 ? (
                      <span className="shrink-0 text-(--text-soft)/70">•</span>
                    ) : null}
                    <span className="min-w-0 truncate tabular-nums text-(--text-muted)">
                      {item}
                    </span>
                  </span>
                ))}
              </div>
            ) : null}
            {modelName ? (
              <span
                className={cn(
                  "flex shrink-0 items-center gap-x-1.5 whitespace-nowrap leading-none text-(--text-muted)",
                  statsItems.length > 0 && "ml-1.5",
                )}
              >
                {statsItems.length > 0 ? (
                  <span className="text-(--text-soft)/70">•</span>
                ) : null}
                <span>{modelName}</span>
              </span>
            ) : null}
          </div>

          <AssistantStatsTrailing
            copied={copied}
            memories={memories}
            onCopy={onCopy}
            onFork={onFork}
          />
        </div>
      ) : null}
    </div>
  );
}

function AssistantStatsTrailing({
  copied,
  memories,
  onCopy,
  onFork,
}: {
  copied: boolean;
  memories: RecalledMemoryReference[];
  onCopy?: () => Promise<void>;
  onFork?: () => Promise<void>;
}) {
  return (
    <div className="ml-auto flex h-6 shrink-0 items-center gap-0.5">
      {memories.length > 0 ? (
        <AssistantMemoryReferences memories={memories} />
      ) : null}
      <div className="flex items-center opacity-100 transition-opacity duration-(--motion-duration-fast) sm:opacity-0 sm:group-hover:opacity-100 sm:focus-within:opacity-100">
        {onCopy ? <AssistantCopyAction copied={copied} onCopy={onCopy} /> : null}
        {onFork ? <AssistantForkAction onFork={onFork} /> : null}
      </div>
    </div>
  );
}

function AssistantForkAction({
  onFork,
}: {
  onFork: () => Promise<void>;
}) {
  const { t } = useI18n();
  const [state, setState] = useState<"failed" | "idle" | "pending">("idle");
  const label = state === "failed"
    ? t("message.fork_failed")
    : state === "pending"
    ? t("message.forking")
    : t("message.fork_to_new_chat");
  const handleFork = useCallback(async () => {
    setState("pending");
    try {
      await onFork();
      setState("idle");
    } catch {
      setState("failed");
    }
  }, [onFork]);
  const Icon = state === "pending" ? LoaderCircle : GitFork;
  return (
    <button
      aria-label={label}
      className={cn(
        "inline-flex h-5 w-5 items-center justify-center rounded-md text-(--icon-muted) transition-[color,background] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
        state === "failed" && "text-(--danger)",
      )}
      disabled={state === "pending"}
      onClick={() => void handleFork()}
      title={label}
      type="button"
    >
      <Icon className={cn("h-3 w-3", state === "pending" && "animate-spin")} />
    </button>
  );
}

const MEMORY_POPOVER_MAX_HEIGHT = 320;
const MEMORY_POPOVER_MIN_HEIGHT = 96;
const MEMORY_POPOVER_WIDTH = 360;

function AssistantMemoryReferences({
  memories,
}: {
  memories: RecalledMemoryReference[];
}) {
  const { t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const close = useCallback(() => setIsOpen(false), []);
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => (
    resolveAnchoredOverlayPosition({
      align: "end",
      anchor,
      estimatedHeight: Math.min(80 + memories.length * 48, MEMORY_POPOVER_MAX_HEIGHT),
      gap: 8,
      maxHeight: MEMORY_POPOVER_MAX_HEIGHT,
      minHeight: MEMORY_POPOVER_MIN_HEIGHT,
      minWidth: MEMORY_POPOVER_WIDTH,
      placement: "auto",
    })
  ), [memories.length]);
  const {
    overlayId,
    overlayPosition,
    overlayRef,
    overlayStyle,
    portalContainer,
  } = useAnchoredOverlayLayer({
    anchorRef: triggerRef,
    disabled: false,
    estimatePosition,
    isOpen,
    onClose: close,
  });
  const label = t("message.recalled_memories");

  return (
    <>
      <button
        ref={triggerRef}
        aria-controls={isOpen ? overlayId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={label}
        className={cn(
          "inline-flex h-6 w-6 items-center justify-center rounded-full text-(--icon-muted) transition-[color,background] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
          isOpen && "bg-(--surface-interactive-hover-background) text-(--icon-strong)",
        )}
        onClick={() => setIsOpen((current) => !current)}
        title={label}
        type="button"
      >
        <BookOpenText className="h-3.5 w-3.5" />
      </button>

      {isOpen && portalContainer ? createPortal(
        <div
          ref={overlayRef}
          aria-label={label}
          className={cn(
            "fixed z-[140] overflow-y-auto p-4",
            OVERLAY_SURFACE_CLASS_NAME,
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
          )}
          data-placement={overlayPosition?.placement ?? "top"}
          role="dialog"
          style={overlayStyle}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          <h3 className="text-sm font-medium text-(--text-strong)">
            {label}
          </h3>
          <ul className="mt-3 list-disc space-y-2 pl-5 text-sm leading-5 text-(--text-muted)">
            {memories.map((memory) => (
              <li key={`${memory.name}\u0000${memory.description}`}>
                {memory.description}
              </li>
            ))}
          </ul>
        </div>,
        portalContainer,
      ) : null}
    </>
  );
}

function AssistantCopyAction({
  copied,
  onCopy,
}: {
  copied: boolean;
  onCopy: () => Promise<void>;
}) {
  const { t } = useI18n();
  const presentation = COPY_ACTION_PRESENTATION[copied ? "copied" : "idle"];
  const Icon = presentation.icon;
  return (
    <button
      aria-label={t("message.copy_reply")}
      className={cn(
        "inline-flex h-5 w-5 items-center justify-center rounded-md text-(--icon-muted) transition-[color,background] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
        presentation.className,
      )}
      onClick={onCopy}
      title={t("message.copy_reply")}
      type="button"
    >
      <Icon className="h-3 w-3" />
    </button>
  );
}
