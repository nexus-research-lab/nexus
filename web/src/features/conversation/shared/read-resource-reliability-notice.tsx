/**
 * INPUT: 只读资源失败后的结果、已有内容影响和安全恢复动作。
 * OUTPUT: 不自动消失、不抢焦点的紧凑 Problem / Impact / Recovery 提示。
 * POS: Conversation 读取资源共用的可靠性视图；不解析错误文本，也不触发自动重试。
 */
"use client";

import { CircleAlert, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";

export function ReadResourceReliabilityNotice({
  className,
  impact,
  isRefreshing,
  onRefresh,
  problem,
  resource,
  stale,
}: {
  className?: string;
  impact: string;
  isRefreshing: boolean;
  onRefresh: () => void;
  problem: string;
  resource: string;
  stale: boolean;
}) {
  const { t } = useI18n();
  return (
    <section
      aria-label={problem}
      className={cn(
        "flex min-w-0 flex-wrap items-start gap-x-2.5 gap-y-1 border-y border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--warning)_2%,var(--surface-control-background))] px-3 py-2 text-xs text-(--text-muted) sm:flex-nowrap",
        className,
      )}
      data-read-resource={resource}
      data-read-resource-state={stale ? "stale" : "error"}
      role="status"
    >
      <CircleAlert
        aria-hidden="true"
        className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--warning)"
      />
      <div className="w-[calc(100%-1.5rem)] min-w-0 flex-none sm:w-auto sm:flex-1">
        <p className="shrink-0 font-medium leading-5 text-(--text-strong)">{problem}</p>
        <RecoverySummary className="mt-0.5 min-w-0" impact={impact} />
      </div>
      <button
        className="ml-6 inline-flex h-7 shrink-0 items-center gap-1 rounded-[7px] px-1.5 font-medium text-(--primary) transition-colors hover:bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] disabled:cursor-wait disabled:opacity-60 motion-reduce:transition-none sm:ml-0"
        disabled={isRefreshing}
        onClick={onRefresh}
        type="button"
      >
        <RefreshCw
          aria-hidden="true"
          className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin motion-reduce:animate-none")}
        />
        {t("state.reload_check")}
      </button>
    </section>
  );
}
