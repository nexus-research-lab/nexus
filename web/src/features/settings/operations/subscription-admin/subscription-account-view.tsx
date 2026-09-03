// INPUT: Subscription 账户、套餐选项、草稿和刷新/保存命令。
// OUTPUT: 账户用量摘要与可编辑套餐绑定列表。
// POS: Operations 账户订阅视图；不拥有通用按钮或选择器视觉。

import { Gauge, Loader2, RefreshCw, Save, ShieldCheck, UsersRound } from "lucide-react";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { SubscriptionAccount } from "@/types/settings/subscription";

import {
  type AccountDraft,
  type AccountViewModel,
  type SubscriptionSummary as SubscriptionSummaryModel,
  createAccountDraft,
  formatDate,
  formatPercent,
  formatTokenCount,
  formatTokenLimit,
} from "./subscription-admin-model";
interface SubscriptionAccountViewProps {
  model: AccountViewModel;
  onChangeDraft: (ownerUserId: string, patch: Partial<AccountDraft>) => void;
  onRefresh: () => Promise<void>;
  onSave: (ownerUserId: string) => Promise<void>;
}

interface SubscriptionAccountRowProps {
  account: SubscriptionAccount;
  disabled: boolean;
  draft: AccountDraft;
  plans: AccountViewModel["plans"];
  savingOwnerUserId: string | null;
  onChangeDraft: (ownerUserId: string, patch: Partial<AccountDraft>) => void;
  onSave: (ownerUserId: string) => Promise<void>;
}

function SubscriptionSummary({
  summary,
}: {
  summary: SubscriptionSummaryModel;
}) {
  const { t } = useI18n();
  const items = [
    {
      icon: UsersRound,
      label: t("settings.subscription.accounts"),
      value: summary.accountCount,
    },
    {
      icon: ShieldCheck,
      label: t("settings.subscription.plans"),
      value: summary.planCount,
    },
    {
      icon: Gauge,
      label: t("settings.subscription.current_month_usage"),
      value: summary.usedTokens,
    },
  ];
  return (
    <section className={SETTINGS_CARD_CLASS_NAME}>
      <div className="grid divide-y divide-(--divider-subtle-color) md:grid-cols-3 md:divide-x md:divide-y-0">
        {items.map((item) => {
          const Icon = item.icon;
          return (
            <div key={item.label} className="px-4 py-3">
              <div className="flex items-center gap-3">
                <div className={SETTINGS_ICON_CLASS_NAME}>
                  <Icon className="h-3.5 w-3.5" />
                </div>
                <div>
                  <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                    {formatTokenCount(item.value)}
                  </p>
                  <p className={cn(
                    "mt-1",
                    getUiTypographyClassName({ role: "caption", tone: "soft" }),
                  )}>
                    {item.label}
                  </p>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function SubscriptionAccountRow({
  account,
  disabled,
  draft,
  plans,
  savingOwnerUserId,
  onChangeDraft,
  onSave,
}: SubscriptionAccountRowProps) {
  const { t } = useI18n();
  const displayName = account.display_name || account.username;
  const periodLabel = `${formatDate(account.period_start)} - ${formatDate(account.period_end)}`;
  const saving = savingOwnerUserId === account.owner_user_id;
  return (
    <div className="grid gap-4 px-4 py-4 lg:grid-cols-[minmax(180px,1.1fr)_minmax(0,1fr)_auto] lg:items-start">
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          <p className={cn(
            "truncate",
            getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
          )}>
            {displayName}
          </p>
          <UiBadge className="uppercase" size="xs">
            {account.role}
          </UiBadge>
        </div>
        <p className={cn(
          "mt-1 truncate",
          getUiTypographyClassName({ role: "metadata", tone: "soft" }),
        )}>
          {account.username}
        </p>
        <div className={cn(
          "mt-3 grid grid-cols-2 gap-2",
          getUiTypographyClassName({ role: "caption", tone: "muted" }),
        )}>
          <span>
            {t("settings.subscription.used")}: {" "}
            <strong className="ui-type-tone-default ui-type-weight-semibold">
              {formatTokenCount(account.used_tokens)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.percent")}: {" "}
            <strong className="ui-type-tone-default ui-type-weight-semibold">
              {formatPercent(account.used_percent)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.sessions")}: {" "}
            <strong className="ui-type-tone-default ui-type-weight-semibold">
              {formatTokenCount(account.session_count)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.messages")}: {" "}
            <strong className="ui-type-tone-default ui-type-weight-semibold">
              {formatTokenCount(account.message_count)}
            </strong>
          </span>
        </div>
        <p className={cn(
          "mt-2",
          getUiTypographyClassName({ role: "caption", tone: "soft" }),
        )}>
          {t("settings.subscription.period")}: {periodLabel}
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <label className="space-y-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.plan")}
          </span>
          <UiSelectMenu
            ariaLabel={t("settings.subscription.plan")}
            disabled={disabled || plans.length === 0}
            menuMinWidth={180}
            onChange={(value) => onChangeDraft(account.owner_user_id, {
              planKey: value,
            })}
            options={plans.map((plan) => ({
              label: plan.display_name,
              value: plan.plan_key,
            }))}
            size="sm"
            value={draft.planKey}
          />
        </label>
        <div className="space-y-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.effective_limit")}
          </span>
          <div className={cn(
            "flex h-9 items-center radius-control-lg border border-(--divider-subtle-color) px-3",
            getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" }),
          )}>
            {formatTokenLimit(
              account.monthly_token_limit,
              t("settings.subscription.limit_unlimited"),
            )}
          </div>
        </div>
      </div>

      <div className="flex lg:justify-end">
        <UiButton
          disabled={disabled}
          onClick={() => void onSave(account.owner_user_id)}
          size="sm"
          tone="primary"
          variant="solid"
        >
          {saving ? (
            <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
          ) : (
            <Save className="h-3.5 w-3.5" />
          )}
          {t("settings.subscription.save")}
        </UiButton>
      </div>
    </div>
  );
}

export function SubscriptionAccountView({
  model,
  onChangeDraft,
  onRefresh,
  onSave,
}: SubscriptionAccountViewProps) {
  const { t } = useI18n();
  const disabled = model.loading
    || model.mutationPending
    || model.mutationsBlocked;
  return (
    <>
      <SubscriptionSummary summary={model.summary} />
      <section className={SETTINGS_CARD_CLASS_NAME}>
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4 py-3">
          <div className="min-w-0">
            <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
              {t("settings.subscription.users_title")}
            </p>
            <p className={cn(
              "mt-1",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}>
              {t("settings.subscription.period")}: {formatDate(model.periodStart)} - {formatDate(model.periodEnd)}
            </p>
          </div>
          <UiButton
            disabled={disabled}
            onClick={() => void onRefresh()}
            size="sm"
            variant="surface"
          >
            {model.loading ? (
              <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
            ) : (
              <RefreshCw className="h-3.5 w-3.5" />
            )}
            {t("settings.subscription.refresh")}
          </UiButton>
        </div>

        {model.loading ? (
          <UiResourceState
            size="sm"
            state="loading"
            title={t("settings.subscription.loading")}
            variant="plain"
          />
        ) : model.accounts.length === 0 ? (
          <UiResourceState
            size="sm"
            state="empty"
            title={t("settings.subscription.users_empty")}
            variant="plain"
          />
        ) : (
          <div className="divide-y divide-(--divider-subtle-color)">
            {model.accounts.map((account) => (
              <SubscriptionAccountRow
                key={account.owner_user_id}
                account={account}
                disabled={disabled}
                draft={model.drafts[account.owner_user_id] ?? createAccountDraft(account)}
                onChangeDraft={onChangeDraft}
                onSave={onSave}
                plans={model.plans}
                savingOwnerUserId={model.savingOwnerUserId}
              />
            ))}
          </div>
        )}
      </section>
    </>
  );
}
