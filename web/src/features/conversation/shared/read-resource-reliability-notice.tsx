/**
 * INPUT: 只读资源失败后的结果、已有内容影响和安全恢复动作。
 * OUTPUT: 不自动消失、不抢焦点的紧凑 Problem / Impact / Recovery 提示。
 * POS: Conversation 读取资源共用的可靠性视图；不解析错误文本，也不触发自动重试。
 */
"use client";

import { CircleAlert, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

export function ReadResourceReliabilityNotice({
  className,
  impact,
  isRefreshing,
  nextStep,
  onRefresh,
  problem,
  resource,
  stale,
}: {
  className?: string;
  impact: string;
  isRefreshing: boolean;
  nextStep: string;
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
        "flex min-w-0 items-start gap-2.5 border-y border-[color:color-mix(in_srgb,var(--warning)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_5%,var(--surface-control-background))] px-3 py-2 text-xs text-(--text-muted)",
        className,
      )}
      data-read-resource={resource}
      data-read-resource-state={stale ? "stale" : "error"}
      role="status"
    >
      <CircleAlert
        aria-hidden="true"
        className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)"
      />
      <div className="min-w-0 flex-1">
        <p className="font-semibold leading-5 text-(--text-strong)">{problem}</p>
        <p className="mt-0.5 leading-5">{impact}</p>
        <p className="leading-5">{nextStep}</p>
      </div>
      <button
        className="inline-flex h-7 shrink-0 items-center gap-1 rounded-[7px] px-2 font-medium text-(--primary) transition-colors hover:bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)] disabled:cursor-wait disabled:opacity-60"
        disabled={isRefreshing}
        onClick={onRefresh}
        type="button"
      >
        <RefreshCw
          aria-hidden="true"
          className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")}
        />
        {t("state.reload_check")}
      </button>
    </section>
  );
}
