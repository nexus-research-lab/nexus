import { Loader2, Plus, Save } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import {
  SETTINGS_CARD_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import type { SubscriptionPlan } from "@/types/settings/subscription";

import {
  PLAN_STATUSES,
  type PlanDraft,
  type PlanStatus,
  type PlanViewModel,
  createPlanDraft,
  formatTokenLimit,
  normalizePlanStatus,
} from "./subscription-admin-model";
import {
  CONTROL_CLASS_NAME,
  SAVE_BUTTON_CLASS_NAME,
  SubscriptionEmptyState,
  SubscriptionLoadingState,
} from "./subscription-admin-ui";

interface SubscriptionPlanViewProps {
  model: PlanViewModel;
  onChangeDraft: (planKey: string, patch: Partial<PlanDraft>) => void;
  onChangeNewDraft: (patch: Partial<PlanDraft>) => void;
  onCreate: () => Promise<void>;
  onSave: (planKey: string) => Promise<void>;
}

interface SubscriptionPlanRowProps {
  disabled: boolean;
  draft: PlanDraft;
  plan: SubscriptionPlan;
  saving: boolean;
  onChangeDraft: (planKey: string, patch: Partial<PlanDraft>) => void;
  onSave: (planKey: string) => Promise<void>;
}

const PLAN_STATUS_LABEL_KEYS: Record<PlanStatus, TranslationKey> = {
  active: "settings.subscription.plan_status_active",
  archived: "settings.subscription.plan_status_archived",
};

function SubscriptionPlanRow({
  disabled,
  draft,
  plan,
  saving,
  onChangeDraft,
  onSave,
}: SubscriptionPlanRowProps) {
  const { t } = useI18n();
  return (
    <div className="grid gap-4 px-4 py-4 xl:grid-cols-[180px_minmax(0,1fr)_auto] xl:items-start">
      <div className="min-w-0">
        <p className="truncate text-[14px] font-semibold text-(--text-strong)">
          {plan.display_name}
        </p>
        <p className="mt-1 truncate font-mono text-[11px] text-(--text-muted)">
          {plan.plan_key}
        </p>
        <p className="mt-2 text-[11px] text-(--text-soft)">
          {t("settings.subscription.plan_current_limit")}: {formatTokenLimit(
            plan.monthly_token_limit,
            t("settings.subscription.limit_unlimited"),
          )}
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.display_name")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={disabled}
            onChange={(event) => onChangeDraft(plan.plan_key, {
              displayName: event.target.value,
            })}
            value={draft.displayName}
          />
        </label>
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.plan_status")}
          </span>
          <UiSelectMenu
            ariaLabel={t("settings.subscription.plan_status")}
            disabled={disabled}
            menuClassName="min-w-[160px]"
            onChange={(value) => onChangeDraft(plan.plan_key, {
              status: normalizePlanStatus(value),
            })}
            options={PLAN_STATUSES.map((status) => ({
              label: t(PLAN_STATUS_LABEL_KEYS[status]),
              value: status,
            }))}
            size="sm"
            value={draft.status}
          />
        </label>
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.plan_limit")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={disabled}
            inputMode="numeric"
            min={0}
            onChange={(event) => onChangeDraft(plan.plan_key, {
              monthlyTokenLimit: event.target.value,
            })}
            placeholder={t("settings.subscription.limit_unlimited")}
            type="number"
            value={draft.monthlyTokenLimit}
          />
        </label>
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.sort_order")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={disabled}
            inputMode="numeric"
            onChange={(event) => onChangeDraft(plan.plan_key, {
              sortOrder: event.target.value,
            })}
            type="number"
            value={draft.sortOrder}
          />
        </label>
        <label className="space-y-1.5 sm:col-span-2 xl:col-span-5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.notes")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={disabled}
            onChange={(event) => onChangeDraft(plan.plan_key, {
              notes: event.target.value,
            })}
            placeholder={t("settings.subscription.notes_placeholder")}
            value={draft.notes}
          />
        </label>
      </div>

      <div className="flex xl:justify-end">
        <button
          className={SAVE_BUTTON_CLASS_NAME}
          disabled={disabled}
          onClick={() => void onSave(plan.plan_key)}
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

function NewSubscriptionPlanForm({
  disabled,
  draft,
  creating,
  onChange,
  onCreate,
}: {
  disabled: boolean;
  draft: PlanDraft;
  creating: boolean;
  onChange: (patch: Partial<PlanDraft>) => void;
  onCreate: () => Promise<void>;
}) {
  const { t } = useI18n();
  return (
    <div className="border-b border-(--divider-subtle-color) px-4 py-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-[160px_minmax(160px,1fr)_160px_auto] xl:items-end">
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.plan_key")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={disabled}
            onChange={(event) => onChange({ planKey: event.target.value })}
            placeholder={t("settings.subscription.plan_key_placeholder")}
            value={draft.planKey}
          />
        </label>
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.display_name")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={disabled}
            onChange={(event) => onChange({ displayName: event.target.value })}
            placeholder={t("settings.subscription.display_name_placeholder")}
            value={draft.displayName}
          />
        </label>
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.plan_limit")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={disabled}
            inputMode="numeric"
            min={0}
            onChange={(event) => onChange({
              monthlyTokenLimit: event.target.value,
            })}
            placeholder={t("settings.subscription.limit_unlimited")}
            type="number"
            value={draft.monthlyTokenLimit}
          />
        </label>
        <button
          className={SAVE_BUTTON_CLASS_NAME}
          disabled={disabled}
          onClick={() => void onCreate()}
          type="button"
        >
          {creating ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Plus className="h-3.5 w-3.5" />
          )}
          {t("settings.subscription.create_plan")}
        </button>
      </div>
    </div>
  );
}

export function SubscriptionPlanView({
  model,
  onChangeDraft,
  onChangeNewDraft,
  onCreate,
  onSave,
}: SubscriptionPlanViewProps) {
  const { t } = useI18n();
  const disabled = model.loading || model.mutationPending;
  return (
    <section className={SETTINGS_CARD_CLASS_NAME}>
      <NewSubscriptionPlanForm
        creating={model.creating}
        disabled={disabled}
        draft={model.newPlanDraft}
        onChange={onChangeNewDraft}
        onCreate={onCreate}
      />

      {model.loading ? (
        <SubscriptionLoadingState label={t("settings.subscription.loading")} />
      ) : model.plans.length === 0 ? (
        <SubscriptionEmptyState label={t("settings.subscription.plans_empty")} />
      ) : (
        <div className="divide-y divide-(--divider-subtle-color)">
          {model.plans.map((plan) => (
            <SubscriptionPlanRow
              key={plan.plan_key}
              disabled={disabled}
              draft={model.drafts[plan.plan_key] ?? createPlanDraft(plan)}
              onChangeDraft={onChangeDraft}
              onSave={onSave}
              plan={plan}
              saving={model.savingPlanKey === plan.plan_key}
            />
          ))}
        </div>
      )}
    </section>
  );
}
