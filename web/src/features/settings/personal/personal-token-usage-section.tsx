// INPUT: 用户累计 Token 与每日用量。
// OUTPUT: 紧凑指标、独立热力图与每日趋势，精确数值通过悬浮和键盘聚焦查看。
// POS: 个人用量展示，不推算账本缺失数据。
import type { TokenUsageSummary } from "@/lib/api/account/auth-api";
import { formatTokens } from "@/lib/format/token-count";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { UiButton } from "@/shared/ui/button/button";
import { buildTokenUsagePresentation } from "./personal-settings-model";
import { PersonalUsageHeatmap, PersonalUsageHistory } from "./personal-usage-history";
import { SETTINGS_CARD_CLASS_NAME } from "../shared/settings-panel-ui";

export function PersonalTokenUsageSection({ usage }: { usage: TokenUsageSummary | undefined }) {
  const { locale, t } = useI18n();
  const presentation = buildTokenUsagePresentation(usage, locale, t);
  const metrics = [
    { label: t("settings.personal.total_tokens"), value: usage?.total_tokens },
    { label: t("settings.personal.input_tokens"), value: usage?.input_tokens },
    { label: t("settings.personal.output_tokens"), value: usage?.output_tokens },
    { label: t("settings.personal.cache_tokens"), value: usage ? usage.cache_creation_input_tokens + usage.cache_read_input_tokens : undefined },
    ...(usage?.quota_limit_tokens != null ? [{ label: t("settings.personal.quota_limit"), value: usage.quota_limit_tokens }] : []),
  ];
  const detail = [
    t("settings.personal.session_count", { count: presentation.sessionCount }),
    t("settings.personal.message_count", { count: presentation.messageCount }),
    t("settings.personal.updated_at", { value: presentation.updatedAt }),
  ].join(" · ");

  return <section className="space-y-4">
    <dl className={`${SETTINGS_CARD_CLASS_NAME} flex flex-wrap py-3`}>
      {metrics.map((metric, index) => <div key={metric.label} className="min-w-28 flex-1 space-y-1 border-r border-(--divider-subtle-color) px-3 text-center last:border-r-0">
          <dt className="ui-type-metadata text-(--text-muted)">{metric.label}</dt>
          <dd><UiTooltip label={`${metric.label} · ${metric.value?.toLocaleString(locale) ?? "—"}${index === 0 ? ` / ${detail}` : ""}`}>
            <UiButton variant="ghost" className="ui-type-section-title h-auto min-h-0 p-0 tabular-nums text-(--text-strong)">{metric.value == null ? "—" : formatTokens(metric.value, locale)}</UiButton>
          </UiTooltip></dd>
      </div>)}
    </dl>
    {usage?.daily && usage.daily.length > 0 ? <>
      <PersonalUsageHeatmap days={usage.daily} />
      <PersonalUsageHistory days={usage.daily} />
    </> : null}
  </section>;
}
