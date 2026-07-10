import { Gauge, Loader2, RefreshCw, Save, ShieldCheck, UsersRound } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
} from "@/features/settings/settings-panel-ui";
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
import {
  SAVE_BUTTON_CLASS_NAME,
  SECONDARY_BUTTON_CLASS_NAME,
  SubscriptionEmptyState,
  SubscriptionLoadingState,
} from "./subscription-admin-ui";

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
                  <p className="mt-1 text-[11px] text-(--text-soft)">
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
          <p className="truncate text-[14px] font-semibold text-(--text-strong)">
            {displayName}
          </p>
          <span className="rounded-full border border-(--divider-subtle-color) px-2 py-0.5 text-[10px] font-semibold uppercase text-(--text-muted)">
            {account.role}
          </span>
        </div>
        <p className="mt-1 truncate text-[12px] text-(--text-soft)">
          {account.username}
        </p>
        <div className="mt-3 grid grid-cols-2 gap-2 text-[11px] text-(--text-muted)">
          <span>
            {t("settings.subscription.used")}: {" "}
            <strong className="font-semibold text-(--text-default)">
              {formatTokenCount(account.used_tokens)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.percent")}: {" "}
            <strong className="font-semibold text-(--text-default)">
              {formatPercent(account.used_percent)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.sessions")}: {" "}
            <strong className="font-semibold text-(--text-default)">
              {formatTokenCount(account.session_count)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.messages")}: {" "}
            <strong className="font-semibold text-(--text-default)">
              {formatTokenCount(account.message_count)}
            </strong>
          </span>
        </div>
        <p className="mt-2 text-[11px] text-(--text-soft)">
          {t("settings.subscription.period")}: {periodLabel}
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.plan")}
          </span>
          <UiSelectMenu
            ariaLabel={t("settings.subscription.plan")}
            disabled={disabled || plans.length === 0}
            menuClassName="min-w-[180px]"
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
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.effective_limit")}
          </span>
          <div className="flex h-9 items-center rounded-xl border border-(--divider-subtle-color) px-3 text-sm font-semibold text-(--text-strong)">
            {formatTokenLimit(
              account.monthly_token_limit,
              t("settings.subscription.limit_unlimited"),
            )}
          </div>
        </div>
      </div>

      <div className="flex lg:justify-end">
        <button
          className={SAVE_BUTTON_CLASS_NAME}
          disabled={disabled}
          onClick={() => void onSave(account.owner_user_id)}
          type="button"
        >
          {saving ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Save className="h-3.5 w-3.5" />
          )}
          {t("settings.subscription.save")}
        </button>
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
  const disabled = model.loading || model.mutationPending;
  return (
    <>
      <SubscriptionSummary summary={model.summary} />
      <section className={SETTINGS_CARD_CLASS_NAME}>
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4 py-3">
          <div className="min-w-0">
            <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
              {t("settings.subscription.users_title")}
            </p>
            <p className="mt-1 text-[11px] text-(--text-soft)">
              {t("settings.subscription.period")}: {formatDate(model.periodStart)} - {formatDate(model.periodEnd)}
            </p>
          </div>
          <button
            className={SECONDARY_BUTTON_CLASS_NAME}
            disabled={disabled}
            onClick={() => void onRefresh()}
            type="button"
          >
            {model.loading ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="h-3.5 w-3.5" />
            )}
            {t("settings.subscription.refresh")}
          </button>
        </div>

        {model.loading ? (
          <SubscriptionLoadingState label={t("settings.subscription.loading")} />
        ) : model.accounts.length === 0 ? (
          <SubscriptionEmptyState label={t("settings.subscription.users_empty")} />
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
