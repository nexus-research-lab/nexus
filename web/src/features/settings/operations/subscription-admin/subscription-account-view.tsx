// INPUT: Subscription 账户、套餐选项、草稿和刷新/保存命令。
// OUTPUT: 可编辑账户套餐绑定列表与未核对写入恢复入口。
// POS: Operations 账户订阅视图；不拥有通用按钮或选择器视觉。

import { Loader2, RefreshCw, Save } from "lucide-react";

import {
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
    <div className="grid gap-4 px-3 py-4 hover:bg-(--surface-interactive-hover-background) @min-[900px]/subscriptions:grid-cols-[minmax(0,1fr)_160px_minmax(0,1.4fr)] @min-[900px]/subscriptions:items-center">
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
      </div>
        <div className={cn(
          "flex flex-wrap gap-x-4 gap-y-1 @min-[900px]/subscriptions:flex-col",
          getUiTypographyClassName({ role: "supporting", tone: "muted" }),
        )}>
          <span>
            <span className="@min-[900px]/subscriptions:hidden">{t("settings.subscription.used")}: </span>
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

      <div className="grid items-end gap-3 @min-[480px]/subscriptions:grid-cols-[minmax(0,1fr)_110px_80px]">
        <label className="grid min-w-0 gap-1.5">
          <span className={cn("@min-[900px]/subscriptions:hidden", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
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
            allowLabelWrap
            size="sm"
            value={draft.planKey}
          />
        </label>
        <div className="grid min-w-0 gap-1.5">
          <span className={cn("@min-[900px]/subscriptions:hidden", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
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
            size="sm"
            variant="surface"
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
      <section className="min-w-0">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-3 pb-3">
          <div className="min-w-0">
            <p className={cn(
              getUiTypographyClassName({ role: "supporting", tone: "muted" }),
            )}>
              {t("settings.subscription.period")}: {formatDate(model.periodStart)} - {formatDate(model.periodEnd)}
            </p>
          </div>
          {model.mutationsBlocked ? (
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
          ) : null}
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
            <div className={cn("hidden grid-cols-[minmax(0,1fr)_160px_minmax(0,1.4fr)] items-center gap-4 px-3 py-2 @min-[900px]/subscriptions:grid", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
              <span>{t("settings.subscription.accounts")}</span>
              <span>{t("settings.subscription.used")}</span>
              <div className="grid grid-cols-[minmax(0,1fr)_110px_80px] gap-3">
                <span>{t("settings.subscription.plan")}</span>
                <span>{t("settings.subscription.effective_limit")}</span>
                <span className="text-right">{t("members.column_actions")}</span>
              </div>
            </div>
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
