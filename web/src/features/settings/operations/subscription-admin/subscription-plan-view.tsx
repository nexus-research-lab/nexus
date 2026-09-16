// INPUT: Subscription 套餐资源、新建草稿、编辑草稿和保存命令。
// OUTPUT: 套餐创建表单与可编辑套餐列表。
// POS: Operations 套餐订阅视图；不拥有通用按钮或表单视觉。

import { useId, useState } from "react";
import { Loader2, Plus, Save } from "lucide-react";

import {
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDialogPortal, UiDialogBackdrop, UiDialogShell, UiDialogHeader, UiDialogBody, UiDialogFooter } from "@/shared/ui/dialog/dialog";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { SubscriptionPlan } from "@/types/settings/subscription";

import {
  PLAN_STATUSES,
  type PlanDraft,
  type PlanStatus,
  type PlanViewModel,
  createPlanDraft,
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

const PLAN_COLUMNS_CLASS_NAME = "@min-[900px]/plans:grid-cols-[minmax(0,1fr)_100px_140px_70px_minmax(0,1.3fr)_80px]";
const PLAN_ROW_CLASS_NAME = cn("grid min-w-0 items-center gap-3 px-3 py-3 @min-[480px]/plans:grid-cols-2", PLAN_COLUMNS_CLASS_NAME);

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
    <article aria-label={plan.display_name} className={cn(PLAN_ROW_CLASS_NAME, "border-b border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background)")}>
        <label className="grid min-w-0 gap-1.5">
          <span className={cn("@min-[900px]/plans:sr-only", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
            {t("settings.subscription.display_name")}
          </span>
          <UiInput
            controlSize="sm"
            variant="surface"
            disabled={disabled}
            onChange={(event) => onChangeDraft(plan.plan_key, {
              displayName: event.target.value,
            })}
            value={draft.displayName}
          />
        </label>
        <label className="grid min-w-0 gap-1.5">
          <span className={cn("@min-[900px]/plans:sr-only", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
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
            size="sm"
            value={draft.status}
          />
        </label>
        <label className="grid min-w-0 gap-1.5">
          <span className={cn("@min-[900px]/plans:sr-only", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
            {t("settings.subscription.plan_limit")}
          </span>
          <UiInput
            controlSize="sm"
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
          <span className={cn("@min-[900px]/plans:sr-only", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
            {t("settings.subscription.sort_order")}
          </span>
          <UiInput
            controlSize="sm"
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
        <label className="grid min-w-0 gap-1.5">
          <span className={cn("@min-[900px]/plans:sr-only", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
            {t("settings.subscription.notes")}
          </span>
          <UiInput
            controlSize="sm"
            variant="surface"
            disabled={disabled}
            onChange={(event) => onChangeDraft(plan.plan_key, {
              notes: event.target.value,
            })}
            placeholder={t("settings.subscription.notes_placeholder")}
            value={draft.notes}
          />
        </label>

      <div className="flex justify-end">
        <UiButton
          disabled={disabled}
          onClick={() => void onSave(plan.plan_key)}
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
    </article>
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
  const [open, setOpen] = useState(false);
  const titleId = useId();
  const close = () => { if (!creating) setOpen(false); };
  return (
    <>
      <UiButton className="w-fit" onClick={() => setOpen(true)} size="sm" variant="ghost" aria-haspopup="dialog">
        <Plus className="h-4 w-4" />
        {t("settings.subscription.create_plan")}
      </UiButton>
      {open ? <UiDialogPortal>
        <UiDialogBackdrop labelledBy={titleId} onClose={close}>
          <UiDialogShell size="md" viewport="adaptiveMax">
            <UiDialogHeader appearance="plain" title={t("settings.subscription.create_plan")} titleId={titleId} onClose={close} />
            <UiDialogBody className="grid gap-4 px-5" scrollable>
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
              <label className="grid min-w-0 gap-1.5">
                <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>{t("settings.subscription.notes")}</span>
                <UiInput
                  variant="surface"
                  disabled={disabled}
                  onChange={(event) => onChange({ notes: event.target.value })}
                  placeholder={t("settings.subscription.notes_placeholder")}
                  value={draft.notes}
                />
              </label>
            </UiDialogBody>
            <UiDialogFooter>
              <UiButton disabled={creating} onClick={close} size="sm" variant="surface">{t("common.cancel")}</UiButton>
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
            </UiDialogFooter>
          </UiDialogShell>
        </UiDialogBackdrop>
      </UiDialogPortal> : null}
    </>
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
        key={model.newPlanDraft.planKey}
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
        <div className="grid">
          <div className={cn("hidden items-center gap-3 border-b border-(--divider-subtle-color) px-3 py-2 @min-[900px]/plans:grid", PLAN_COLUMNS_CLASS_NAME, SETTINGS_CONTROL_LABEL_CLASS_NAME)}>
            {(["settings.subscription.display_name", "settings.subscription.plan_status", "settings.subscription.plan_limit", "settings.subscription.sort_order", "settings.subscription.notes", "members.column_actions"] as const).map((key) => <span key={key}>{t(key)}</span>)}
          </div>
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
