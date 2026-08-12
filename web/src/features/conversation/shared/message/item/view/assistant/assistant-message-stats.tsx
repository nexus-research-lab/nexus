import { Check, CircleCheck, Copy, type LucideIcon } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { GoalCompletionReceipt } from "@/types/conversation/message/entity";

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
  model,
  onCopy,
  stats,
  streaming,
}: {
  copied: boolean;
  goalCompletionReceipt: GoalCompletionReceipt | null;
  model?: string;
  onCopy?: () => Promise<void>;
  stats: AssistantFooterStats | null;
  streaming: boolean;
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
              onCopy={onCopy}
              streaming={streaming}
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
            onCopy={onCopy}
            streaming={streaming}
          />
        </div>
      ) : null}
    </div>
  );
}

function AssistantStatsTrailing({
  copied,
  onCopy,
  streaming,
}: {
  copied: boolean;
  onCopy?: () => Promise<void>;
  streaming: boolean;
}) {
  if (streaming) {
    return (
      <span
        aria-hidden="true"
        className="ml-auto inline-flex h-1.5 w-1.5 shrink-0 rounded-full bg-(--text-soft) opacity-70"
      />
    );
  }

  return (
    <div className="ml-auto flex h-5 shrink-0 items-center gap-0.5 opacity-0 transition-opacity duration-(--motion-duration-fast) sm:group-hover:opacity-100">
      {onCopy ? <AssistantCopyAction copied={copied} onCopy={onCopy} /> : null}
    </div>
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
