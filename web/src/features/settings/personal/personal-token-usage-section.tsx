"use client";

import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import {
  Database,
  Gauge,
  KeyRound,
  LockKeyhole,
  ShieldCheck,
} from "lucide-react";

import type { TokenUsageSummary } from "@/lib/api/account/auth-api";
import { cn } from "@/shared/ui/class-name";
import { formatTokens } from "@/lib/format/token-count";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import {
  buildTokenUsagePresentation,
  type TokenUsageMetricKey,
  type TokenUsageValueKey,
} from "./personal-settings-model";

interface UsageMetricDefinition {
  icon: LucideIcon;
  key: TokenUsageMetricKey;
  labelKey: TranslationKey;
}

const USAGE_METRIC_DEFINITIONS: readonly UsageMetricDefinition[] = [
  { key: "quota", icon: ShieldCheck, labelKey: "settings.personal.quota_limit" },
  { key: "input", icon: KeyRound, labelKey: "settings.personal.input_tokens" },
  { key: "output", icon: LockKeyhole, labelKey: "settings.personal.output_tokens" },
  { key: "cache", icon: Database, labelKey: "settings.personal.cache_tokens" },
];

const TOKEN_CHART_DEFINITIONS: readonly {
  className: string;
  key: TokenUsageValueKey;
}[] = [
  { key: "input", className: "bg-primary" },
  { key: "output", className: "bg-sky-500" },
  { key: "cache", className: "bg-amber-500" },
];

export function PersonalTokenUsageSection({
  usage,
}: {
  usage: TokenUsageSummary | undefined;
}) {
  const { locale, t } = useI18n();
  const presentation = buildTokenUsagePresentation(usage, locale, t);

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
                value: presentation.updatedAt,
              })}
            </p>
          </div>
        </div>
        <div className="text-left lg:text-right">
          <div className="text-[24px] font-semibold tracking-tight text-(--text-strong)">
            {presentation.totalTokens}
          </div>
          <div className="mt-1 text-[11px] font-medium text-(--text-soft)">
            {t("settings.personal.total_tokens")}
          </div>
        </div>
      </div>

      <div className="mx-3 border-t border-(--divider-subtle-color)" />

      <div className="grid gap-2 px-3 py-3 sm:grid-cols-2">
        {USAGE_METRIC_DEFINITIONS.map((definition) => {
          const Icon = definition.icon;
          return (
            <UsageMetric
              icon={<Icon className="h-3.5 w-3.5" />}
              key={definition.key}
              label={t(definition.labelKey)}
              value={presentation.metrics[definition.key]}
            />
          );
        })}
      </div>

      <div className="mx-3 border-t border-(--divider-subtle-color)" />

      <TokenUsageChart
        values={presentation.tokenValues}
        labels={{
          input: t("settings.personal.input_tokens"),
          output: t("settings.personal.output_tokens"),
          cache: t("settings.personal.cache_tokens"),
        }}
      />

      <div className="mx-3 border-t border-(--divider-subtle-color)" />

      <div className="grid gap-2 px-3 py-2.5 text-[11px] text-(--text-soft) sm:grid-cols-2">
        <span>{t("settings.personal.session_count", { count: presentation.sessionCount })}</span>
        <span>{t("settings.personal.message_count", { count: presentation.messageCount })}</span>
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
  values,
  labels,
}: {
  values: Record<TokenUsageValueKey, number>;
  labels: Record<TokenUsageValueKey, string>;
}) {
  const total = Math.max(Object.values(values).reduce((sum, value) => sum + value, 0), 1);
  const items = TOKEN_CHART_DEFINITIONS.map((definition) => ({
    ...definition,
    label: labels[definition.key],
    value: values[definition.key],
  }));

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
