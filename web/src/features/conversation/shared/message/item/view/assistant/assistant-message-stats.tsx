/**
 * INPUT: Assistant 统计、Goal 完成回执、模型与记忆引用投影。
 * OUTPUT: 可压缩的消息尾部元数据、共享微型动作与语义记忆引用浮层。
 * POS: Assistant 消息正文下方的唯一统计与回执展示面。
 */
import {
  BookOpenText,
  Check,
  CircleCheck,
  Copy,
  GitFork,
  LoaderCircle,
} from "lucide-react";
import {
  useCallback,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveAnchoredOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-model";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import {
  ANCHORED_OVERLAY_MOTION_CLASS_NAME,
  OVERLAY_SURFACE_CLASS_NAME,
} from "@/shared/ui/overlay/overlay-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type {
  GoalCompletionReceipt,
  RecalledMemoryReference,
} from "@/types/conversation/message/entity";

import type { AssistantFooterStats } from "./assistant-message-model";
import { buildGoalCompletionReceiptItems } from "./goal-completion-receipt";

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
          className="flex min-w-0 items-center gap-1.5 leading-tight text-[color:color-mix(in_srgb,var(--success)_82%,var(--text-default)_18%)]"
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
              <div className="nexus-chat-message-stat-list flex min-w-0 flex-nowrap items-center gap-x-1.5 overflow-hidden whitespace-nowrap leading-tight">
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
    <UiIconButton
      aria-label={label}
      disabled={state === "pending"}
      onClick={() => void handleFork()}
      size="2xs"
      tone={state === "failed" ? "danger" : "default"}
      tooltip={label}
      variant="ghost"
    >
      <Icon
        className={state === "pending"
          ? getUiSpinnerClassName({ size: "xs" })
          : "h-3 w-3"}
      />
    </UiIconButton>
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
      <UiIconButton
        ref={triggerRef}
        aria-controls={isOpen ? overlayId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={label}
        onClick={() => setIsOpen((current) => !current)}
        shape="round"
        size="xs"
        tooltip={label}
        variant="ghost"
      >
        <BookOpenText className="h-3.5 w-3.5" />
      </UiIconButton>

      {isOpen && portalContainer ? createPortal(
        <div
          ref={overlayRef}
          aria-label={label}
          className={cn(
            "fixed ui-layer-popover overflow-y-auto p-4",
            OVERLAY_SURFACE_CLASS_NAME,
            ANCHORED_OVERLAY_MOTION_CLASS_NAME,
          )}
          data-placement={overlayPosition?.placement ?? "top"}
          role="dialog"
          style={overlayStyle}
          {...OPEN_OVERLAY_DATA_ATTRIBUTES}
        >
          <h3 className={getUiTypographyClassName({
            role: "metadata",
            tone: "strong",
            weight: "semibold",
          })}>
            {label}
          </h3>
          <ul className={cn(
            "mt-3 list-disc space-y-2 pl-5",
            getUiTypographyClassName({ role: "supporting", tone: "muted" }),
          )}>
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
  const Icon = copied ? Check : Copy;
  return (
    <UiIconButton
      aria-label={t("message.copy_reply")}
      onClick={onCopy}
      size="2xs"
      tone={copied ? "success" : "default"}
      tooltip={t("message.copy_reply")}
      variant="ghost"
    >
      <Icon className="h-3 w-3" />
    </UiIconButton>
  );
}
