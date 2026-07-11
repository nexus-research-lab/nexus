"use client";

import type { ReactNode } from "react";
import {
  Database,
  Gauge,
  KeyRound,
  LockKeyhole,
  ShieldCheck,
} from "lucide-react";

import type { TokenUsageSummary } from "@/lib/api/account/auth-api";
import { cn, formatTokens } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";

function formatUpdatedAt(value: string, locale: "zh" | "en"): string {
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) {
    return "--";
  }
  return date.toLocaleString(locale === "zh" ? "zh-CN" : "en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function PersonalTokenUsageSection({
  usage,
}: {
  usage: TokenUsageSummary | undefined;
}) {
  const { locale, t } = useI18n();
  const quotaText = usage?.quota_limit_tokens == null
    ? t("settings.personal.quota_unset")
    : `${formatTokens(usage.total_tokens)} / ${formatTokens(usage.quota_limit_tokens)}`;

  return (
    <section className="order-last overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
      <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
            <Gauge className="h-3.5 w-3.5" />
          </div>
          <div className="min-w-0">
            <h3 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
              {t("settings.personal.token_usage_title")}
            </h3>
            <p className="mt-1 text-[12px] leading-5 text-(--text-soft)">
              {t("settings.personal.updated_at", {
                value: usage ? formatUpdatedAt(usage.updated_at, locale) : "--",
              })}
            </p>
          </div>
        </div>
        <div className="text-left lg:text-right">
          <div className="text-[24px] font-semibold tracking-tight text-(--text-strong)">
            {formatTokens(usage?.total_tokens ?? 0)}
          </div>
          <div className="mt-1 text-[11px] font-medium text-(--text-soft)">
            {t("settings.personal.total_tokens")}
          </div>
        </div>
      </div>

      <div className="mx-3 border-t border-(--divider-subtle-color)" />

      <div className="grid gap-2 px-3 py-3 sm:grid-cols-2">
        <UsageMetric
          icon={<ShieldCheck className="h-3.5 w-3.5" />}
          label={t("settings.personal.quota_limit")}
          value={quotaText}
        />
        <UsageMetric
          icon={<KeyRound className="h-3.5 w-3.5" />}
          label={t("settings.personal.input_tokens")}
          value={formatTokens(usage?.input_tokens ?? 0)}
        />
        <UsageMetric
          icon={<LockKeyhole className="h-3.5 w-3.5" />}
          label={t("settings.personal.output_tokens")}
          value={formatTokens(usage?.output_tokens ?? 0)}
        />
        <UsageMetric
          icon={<Database className="h-3.5 w-3.5" />}
          label={t("settings.personal.cache_tokens")}
          value={formatTokens(
            (usage?.cache_creation_input_tokens ?? 0) + (usage?.cache_read_input_tokens ?? 0),
          )}
        />
      </div>

      <div className="mx-3 border-t border-(--divider-subtle-color)" />

      <TokenUsageChart
        usage={usage}
        labels={{
          input: t("settings.personal.input_tokens"),
          output: t("settings.personal.output_tokens"),
          cache: t("settings.personal.cache_tokens"),
        }}
      />

      <div className="mx-3 border-t border-(--divider-subtle-color)" />

      <div className="grid gap-2 px-3 py-2.5 text-[11px] text-(--text-soft) sm:grid-cols-2">
        <span>{t("settings.personal.session_count", { count: usage?.session_count ?? 0 })}</span>
        <span>{t("settings.personal.message_count", { count: usage?.message_count ?? 0 })}</span>
      </div>
    </section>
  );
}

function UsageMetric({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="flex min-w-0 items-center gap-3 rounded-[12px] border border-(--divider-subtle-color) bg-transparent px-3 py-3">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[10px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
        {icon}
      </div>
      <div className="min-w-0">
        <div className="truncate text-[11px] font-medium text-(--text-soft)">
          {label}
        </div>
        <div className="mt-1 truncate text-[14px] font-semibold text-(--text-strong)">
          {value}
        </div>
      </div>
    </div>
  );
}

function TokenUsageChart({
  usage,
  labels,
}: {
  usage: TokenUsageSummary | undefined;
  labels: {
    input: string;
    output: string;
    cache: string;
  };
}) {
  const inputTokens = usage?.input_tokens ?? 0;
  const outputTokens = usage?.output_tokens ?? 0;
  const cacheTokens = (usage?.cache_creation_input_tokens ?? 0) + (usage?.cache_read_input_tokens ?? 0);
  const total = Math.max(inputTokens + outputTokens + cacheTokens, 1);
  const items = [
    {
      key: "input",
      label: labels.input,
      value: inputTokens,
      className: "bg-primary",
    },
    {
      key: "output",
      label: labels.output,
      value: outputTokens,
      className: "bg-sky-500",
    },
    {
      key: "cache",
      label: labels.cache,
      value: cacheTokens,
      className: "bg-amber-500",
    },
  ];

  return (
    <div className="px-3 py-3">
      <div className="flex h-2 overflow-hidden rounded-full bg-[color:color-mix(in_srgb,var(--divider-subtle-color)_55%,transparent)]">
        {items.map((item) => (
          <div
            className={cn(item.value > 0 ? "min-w-[2px]" : "", item.className)}
            key={item.key}
            style={{ width: `${(item.value / total) * 100}%` }}
          />
        ))}
      </div>
      <div className="mt-3 grid gap-2 sm:grid-cols-3">
        {items.map((item) => (
          <div className="flex min-w-0 items-center gap-2 text-[11px] text-(--text-soft)" key={item.key}>
            <span className={cn("h-2 w-2 shrink-0 rounded-full", item.className)} />
            <span className="min-w-0 flex-1 truncate">{item.label}</span>
            <span className="font-semibold text-(--text-strong)">{formatTokens(item.value)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
