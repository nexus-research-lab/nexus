// INPUT: Subscription 套餐资源、新建草稿、编辑草稿和保存命令。
// OUTPUT: 套餐创建表单与可编辑套餐列表。
// POS: Operations 套餐订阅视图；不拥有通用按钮或表单视觉。

import { Loader2, Plus, Save } from "lucide-react";

import {
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
    <UiDisclosure
      variant="panel"
      summaryRole="control"
      label={<span className="grid gap-1 wrap-anywhere">
        <span>{plan.display_name}</span>
        <span className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
          {t("settings.subscription.plan_current_limit")}: {formatTokenLimit(plan.monthly_token_limit, t("settings.subscription.limit_unlimited"))}
        </span>
      </span>}
      meta={<UiBadge size="xs">{t(PLAN_STATUS_LABEL_KEYS[normalizePlanStatus(plan.status)])}</UiBadge>}
    >
      <p className={cn("mb-4 wrap-anywhere", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
        {t("settings.subscription.plan_key")}: {plan.plan_key}
      </p>
      <div className="grid items-end gap-4 @min-[480px]/plans:grid-cols-2 @min-[900px]/plans:grid-cols-4">
        <label className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.display_name")}
          </span>
          <UiInput
            variant="surface"
            disabled={disabled}
            onChange={(event) => onChangeDraft(plan.plan_key, {
              displayName: event.target.value,
            })}
            value={draft.displayName}
          />
        </label>
        <label className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.plan_status")}
          </span>
          <UiSelectMenu
            ariaLabel={t("settings.subscription.plan_status")}
            disabled={disabled}
            menuMinWidth={160}
            onChange={(value) => onChangeDraft(plan.plan_key, {
              status: normalizePlanStatus(value),
            })}
            options={PLAN_STATUSES.map((status) => ({
              label: t(PLAN_STATUS_LABEL_KEYS[status]),
              value: status,
            }))}
            size="md"
            value={draft.status}
          />
        </label>
        <label className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.plan_limit")}
          </span>
          <UiInput
            variant="surface"
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
        <label className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.sort_order")}
          </span>
          <UiInput
            variant="surface"
            disabled={disabled}
            inputMode="numeric"
            onChange={(event) => onChangeDraft(plan.plan_key, {
              sortOrder: event.target.value,
            })}
            type="number"
            value={draft.sortOrder}
          />
        </label>
        <label className="grid min-w-0 gap-1.5 @min-[480px]/plans:col-span-2 @min-[900px]/plans:col-span-4">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.notes")}
          </span>
          <UiInput
            variant="surface"
            disabled={disabled}
            onChange={(event) => onChangeDraft(plan.plan_key, {
              notes: event.target.value,
            })}
            placeholder={t("settings.subscription.notes_placeholder")}
            value={draft.notes}
          />
        </label>
      </div>

      <div className="mt-4 flex justify-end">
        <UiButton
          disabled={disabled}
          onClick={() => void onSave(plan.plan_key)}
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
    </UiDisclosure>
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
    <UiDisclosure label={t("settings.subscription.create_plan")} variant="panel">
      <div className="grid items-end gap-4 @min-[480px]/plans:grid-cols-2 @min-[900px]/plans:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_auto]">
        <label className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.plan_key")}
          </span>
          <UiInput
            variant="surface"
            disabled={disabled}
            onChange={(event) => onChange({ planKey: event.target.value })}
            placeholder={t("settings.subscription.plan_key_placeholder")}
            value={draft.planKey}
          />
        </label>
        <label className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.display_name")}
          </span>
          <UiInput
            variant="surface"
            disabled={disabled}
            onChange={(event) => onChange({ displayName: event.target.value })}
            placeholder={t("settings.subscription.display_name_placeholder")}
            value={draft.displayName}
          />
        </label>
        <label className="grid min-w-0 gap-1.5">
          <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
            {t("settings.subscription.plan_limit")}
          </span>
          <UiInput
            variant="surface"
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
        <UiButton
          disabled={disabled}
          onClick={() => void onCreate()}
          size="md"
          tone="primary"
          variant="solid"
        >
          {creating ? (
            <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
          ) : (
            <Plus className="h-3.5 w-3.5" />
          )}
          {t("settings.subscription.create_plan")}
        </UiButton>
      </div>
    </UiDisclosure>
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
  const disabled = model.loading
    || model.mutationPending
    || model.mutationsBlocked;
  return (
    <section className="@container/plans grid min-w-0 gap-4">
      <NewSubscriptionPlanForm
        creating={model.creating}
        disabled={disabled}
        draft={model.newPlanDraft}
        onChange={onChangeNewDraft}
        onCreate={onCreate}
      />

      {model.loading ? (
        <UiResourceState
          size="md"
          state="loading"
          title={t("settings.subscription.loading")}
          variant="plain"
        />
      ) : model.plans.length === 0 ? (
        <UiResourceState
          size="md"
          state="empty"
          title={t("settings.subscription.plans_empty")}
          variant="plain"
        />
      ) : (
        <div className="grid gap-3">
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
