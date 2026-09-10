// INPUT: Subscription 账户、套餐选项、草稿和刷新/保存命令。
// OUTPUT: 账户用量摘要与可编辑套餐绑定列表。
// POS: Operations 账户订阅视图；不拥有通用按钮或选择器视觉。

import { Loader2, RefreshCw, Save } from "lucide-react";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
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
    [t("settings.subscription.accounts"), summary.accountCount],
    [t("settings.subscription.plans"), summary.planCount],
    [t("settings.subscription.current_month_usage"), summary.usedTokens],
  ] as const;
  return (
    <dl className="grid grid-cols-1 gap-4 @min-[480px]/subscriptions:grid-cols-3">
      {items.map(([label, value]) => (
        <div key={label} className="min-w-0">
          <dt className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{label}</dt>
          <dd className={cn("mt-1 tabular-nums", SETTINGS_ITEM_TITLE_CLASS_NAME)}>{formatTokenCount(value)}</dd>
        </div>
      ))}
    </dl>
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
  const saving = savingOwnerUserId === account.owner_user_id;
  return (
    <div className="grid gap-4 px-4 py-4 @min-[800px]/subscriptions:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] @min-[800px]/subscriptions:items-center">
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          <p className={cn(
            "wrap-anywhere",
            SETTINGS_ITEM_TITLE_CLASS_NAME,
          )}>
            {displayName}
          </p>
        </div>
        <p className={cn(
          "mt-1 wrap-anywhere",
          getUiTypographyClassName({ role: "metadata", tone: "soft" }),
        )}>
          {account.username}
        </p>
        <div className={cn(
          "mt-3 flex flex-wrap gap-x-4 gap-y-1",
          getUiTypographyClassName({ role: "supporting", tone: "muted" }),
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
        </div>
      </div>

      <div className="grid items-end gap-3 @min-[480px]/subscriptions:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
        <label className="grid min-w-0 gap-1.5">
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
            options={plans
              .filter((plan) => (
                plan.status === "active" || plan.plan_key === account.plan_key
              ))
              .map((plan) => ({
                label: plan.display_name,
                value: plan.plan_key,
              }))}
            size="md"
            value={draft.planKey}
          />
        </label>
        <div className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.effective_limit")}
          </span>
          <div className={cn(
            "flex min-h-9 items-center tabular-nums",
            getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" }),
          )}>
            {formatTokenLimit(
              account.monthly_token_limit,
              t("settings.subscription.limit_unlimited"),
            )}
          </div>
        </div>
        <div className="flex justify-end">
          <UiButton
            disabled={disabled}
            onClick={() => void onSave(account.owner_user_id)}
            size="md"
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
    <div className="@container/subscriptions grid min-w-0 gap-5">
      <SubscriptionSummary summary={model.summary} />
      <section className={SETTINGS_CARD_CLASS_NAME}>
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4 py-3">
          <div className="min-w-0">
            <p className={cn(
              getUiTypographyClassName({ role: "supporting", tone: "muted" }),
            )}>
              {t("settings.subscription.period")}: {formatDate(model.periodStart)} - {formatDate(model.periodEnd)}
            </p>
          </div>
          <UiButton
            disabled={model.loading || model.mutationPending}
            onClick={() => void onRefresh()}
            size="sm"
            variant="text"
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
    </div>
  );
}
