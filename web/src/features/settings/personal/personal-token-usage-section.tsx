/**
 * INPUT: 用户 Token 用量汇总。
 * OUTPUT: 总量、额度、三类明细、构成比例和覆盖范围。
 * POS: 个人设置的用量摘要；数值用于精确阅读，图表用于快速比较构成。
 */
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
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import {
  buildTokenUsagePresentation,
  type TokenUsageMetricKey,
  type TokenUsageValueKey,
} from "./personal-settings-model";
import { SETTINGS_CARD_CLASS_NAME } from "../shared/settings-panel-ui";

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
  { key: "output", className: "bg-(--accent)" },
  { key: "cache", className: "bg-(--warning)" },
];

export function PersonalTokenUsageSection({
  usage,
}: {
  usage: TokenUsageSummary | undefined;
}) {
  const { locale, t } = useI18n();
  const presentation = buildTokenUsagePresentation(usage, locale, t);

  return (
    <section className={cn("order-last", SETTINGS_CARD_CLASS_NAME)}>
      <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center radius-control-lg bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
            <Gauge className="h-3.5 w-3.5" />
          </div>
          <div className="min-w-0">
            <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
              {t("settings.personal.token_usage_title")}
            </h3>
            <p className={cn(
              "mt-1",
              getUiTypographyClassName({ role: "metadata", tone: "soft" }),
            )}>
              {t("settings.personal.updated_at", {
                value: presentation.updatedAt,
              })}
            </p>
          </div>
        </div>
        <div className="text-left lg:text-right">
          <div className={getUiTypographyClassName({ role: "objectTitle", tone: "strong" })}>
            {presentation.totalTokens}
          </div>
          <div className={cn(
            "mt-1",
            getUiTypographyClassName({ role: "caption", tone: "soft", weight: "medium" }),
          )}>
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

      <div className={cn(
        "grid gap-2 px-3 py-2.5 sm:grid-cols-2",
        getUiTypographyClassName({ role: "caption", tone: "soft" }),
      )}>
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
    <div className="flex min-w-0 items-center gap-3 surface-radius-md border border-(--divider-subtle-color) bg-transparent px-3 py-3">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center radius-control-md bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
        {icon}
      </div>
      <div className="min-w-0">
        <div className={cn(
          "truncate",
          getUiTypographyClassName({ role: "caption", tone: "soft", weight: "medium" }),
        )}>
          {label}
        </div>
        <div className={cn(
          "mt-1 truncate",
          getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
        )}>
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
          <div
            className={cn(
              "flex min-w-0 items-center gap-2",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}
            key={item.key}
          >
            <span className={cn("h-2 w-2 shrink-0 rounded-full", item.className)} />
            <span className="min-w-0 flex-1 truncate">{item.label}</span>
            <span className={getUiTypographyClassName({
              role: "caption",
              tone: "strong",
              weight: "semibold",
            })}>
              {formatTokens(item.value)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
