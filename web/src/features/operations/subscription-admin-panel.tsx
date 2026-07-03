"use client";

import {
  Gauge,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  UsersRound,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  createSubscriptionPlanApi,
  getSubscriptionOverviewApi,
  updateSubscriptionPlanApi,
  updateUserSubscriptionApi,
} from "@/lib/api/subscription-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button-styles";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import type {
  SubscriptionAccount,
  SubscriptionOverview,
  SubscriptionPlan,
  UpsertSubscriptionPlanPayload,
} from "@/types/settings/subscription";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
} from "@/features/settings/settings-panel-ui";

type PlanStatus = "active" | "archived";
type FeedbackTone = "success" | "error";

interface AccountDraft {
  planKey: string;
}

interface PlanDraft {
  planKey: string;
  displayName: string;
  status: PlanStatus;
  monthlyTokenLimit: string;
  notes: string;
  sortOrder: string;
}

interface FeedbackState {
  tone: FeedbackTone;
  title: string;
  message: string;
}

type SubscriptionAdminView = "users" | "plans";

interface SubscriptionAdminPanelProps {
  view: SubscriptionAdminView;
}

interface SubscriptionPlanRowProps {
  draft: PlanDraft;
  plan: SubscriptionPlan;
  saving: boolean;
  t: ReturnType<typeof useI18n>["t"];
  onChangeDraft: (planKey: string, patch: Partial<PlanDraft>) => void;
  onSave: (planKey: string) => void;
}

interface SubscriptionAccountRowProps {
  account: SubscriptionAccount;
  draft: AccountDraft;
  plans: SubscriptionPlan[];
  saving: boolean;
  t: ReturnType<typeof useI18n>["t"];
  onChangeDraft: (ownerUserId: string, patch: Partial<AccountDraft>) => void;
  onSave: (ownerUserId: string) => void;
}

const CONTROL_CLASS_NAME =
  "dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)";
const SAVE_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "sm", tone: "primary", variant: "solid" },
  "gap-1.5",
);
const SECONDARY_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "sm", variant: "surface" },
  "gap-1.5",
);

const PLAN_STATUSES: PlanStatus[] = ["active", "archived"];

const EMPTY_NEW_PLAN_DRAFT: PlanDraft = {
  planKey: "",
  displayName: "",
  status: "active",
  monthlyTokenLimit: "",
  notes: "",
  sortOrder: "100",
};

function buildAccountDrafts(
  accounts: SubscriptionAccount[],
): Record<string, AccountDraft> {
  return accounts.reduce<Record<string, AccountDraft>>((result, account) => {
    result[account.owner_user_id] = { planKey: account.plan_key };
    return result;
  }, {});
}

function buildPlanDrafts(plans: SubscriptionPlan[]): Record<string, PlanDraft> {
  return plans.reduce<Record<string, PlanDraft>>((result, plan) => {
    result[plan.plan_key] = {
      planKey: plan.plan_key,
      displayName: plan.display_name,
      status: normalizePlanStatus(plan.status),
      monthlyTokenLimit:
        plan.monthly_token_limit === null
          ? ""
          : String(plan.monthly_token_limit),
      notes: plan.notes,
      sortOrder: String(plan.sort_order || 100),
    };
    return result;
  }, {});
}

function normalizePlanStatus(value: string): PlanStatus {
  return value === "archived" ? "archived" : "active";
}

function parseNonNegativeInteger(value: string): number | null {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return Number.NaN;
  }
  return Math.floor(parsed);
}

function parseSortOrder(value: string): number {
  const parsed = Number(value.trim());
  if (!Number.isFinite(parsed)) {
    return 100;
  }
  return Math.floor(parsed);
}

function formatTokenCount(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(
    value,
  );
}

function formatDate(value: string): string {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

function formatPercent(value: number | null): string {
  if (value === null) {
    return "--";
  }
  return `${Math.round(value)}%`;
}

function getPlanLimitLabel(
  plan: SubscriptionPlan,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (plan.monthly_token_limit === null) {
    return t("settings.subscription.limit_unlimited");
  }
  return formatTokenCount(plan.monthly_token_limit);
}

function getAccountLimitLabel(
  account: SubscriptionAccount,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (account.monthly_token_limit === null) {
    return t("settings.subscription.limit_unlimited");
  }
  return formatTokenCount(account.monthly_token_limit);
}

function getPlanStatusLabel(
  status: PlanStatus,
  t: ReturnType<typeof useI18n>["t"],
): string {
  return status === "archived"
    ? t("settings.subscription.plan_status_archived")
    : t("settings.subscription.plan_status_active");
}

function buildPlanPayload(
  planKey: string,
  draft: PlanDraft,
): UpsertSubscriptionPlanPayload | null {
  const monthlyTokenLimit = parseNonNegativeInteger(draft.monthlyTokenLimit);
  if (Number.isNaN(monthlyTokenLimit)) {
    return null;
  }
  return {
    plan_key: planKey.trim(),
    display_name: draft.displayName.trim(),
    status: draft.status,
    monthly_token_limit: monthlyTokenLimit,
    notes: draft.notes.trim(),
    sort_order: parseSortOrder(draft.sortOrder),
  };
}

function SubscriptionPlanRow({
  draft,
  plan,
  saving,
  t,
  onChangeDraft,
  onSave,
}: SubscriptionPlanRowProps) {
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
          {t("settings.subscription.plan_current_limit")}:{" "}
          {getPlanLimitLabel(plan, t)}
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <label className="space-y-1.5">
          <span className="text-[11px] font-semibold text-(--text-muted)">
            {t("settings.subscription.display_name")}
          </span>
          <input
            className={CONTROL_CLASS_NAME}
            disabled={saving}
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
            disabled={saving}
            menuClassName="min-w-[160px]"
            onChange={(value) => onChangeDraft(plan.plan_key, {
              status: normalizePlanStatus(value),
            })}
            options={PLAN_STATUSES.map((status) => ({
              label: getPlanStatusLabel(status, t),
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
            disabled={saving}
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
            disabled={saving}
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
            disabled={saving}
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
          disabled={saving}
          onClick={() => onSave(plan.plan_key)}
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

function SubscriptionAccountRow({
  account,
  draft,
  plans,
  saving,
  t,
  onChangeDraft,
  onSave,
}: SubscriptionAccountRowProps) {
  const displayName = account.display_name || account.username;
  const periodLabel = `${formatDate(account.period_start)} - ${formatDate(account.period_end)}`;
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
            {t("settings.subscription.used")}:{" "}
            <strong className="font-semibold text-(--text-default)">
              {formatTokenCount(account.used_tokens)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.percent")}:{" "}
            <strong className="font-semibold text-(--text-default)">
              {formatPercent(account.used_percent)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.sessions")}:{" "}
            <strong className="font-semibold text-(--text-default)">
              {formatTokenCount(account.session_count)}
            </strong>
          </span>
          <span>
            {t("settings.subscription.messages")}:{" "}
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
            disabled={saving || plans.length === 0}
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
            {getAccountLimitLabel(account, t)}
          </div>
        </div>
      </div>

      <div className="flex lg:justify-end">
        <button
          className={SAVE_BUTTON_CLASS_NAME}
          disabled={saving}
          onClick={() => onSave(account.owner_user_id)}
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

export function SubscriptionAdminPanel({ view }: SubscriptionAdminPanelProps) {
  const { t } = useI18n();
  const [overview, setOverview] = useState<SubscriptionOverview | null>(null);
  const [accountDrafts, setAccountDrafts] = useState<Record<string, AccountDraft>>({});
  const [planDrafts, setPlanDrafts] = useState<Record<string, PlanDraft>>({});
  const [newPlanDraft, setNewPlanDraft] = useState<PlanDraft>(EMPTY_NEW_PLAN_DRAFT);
  const [loading, setLoading] = useState(true);
  const [savingUserId, setSavingUserId] = useState<string | null>(null);
  const [savingPlanKey, setSavingPlanKey] = useState<string | null>(null);
  const [creatingPlan, setCreatingPlan] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  const loadOverview = useCallback(async () => {
    try {
      setLoading(true);
      const result = await getSubscriptionOverviewApi();
      setOverview(result);
      setAccountDrafts(buildAccountDrafts(result.accounts));
      setPlanDrafts(buildPlanDrafts(result.plans));
      setFeedback((current) => (current?.tone === "error" ? null : current));
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.load_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.subscription.load_failed_message"),
      });
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview]);

  const handleChangeAccountDraft = useCallback((
    ownerUserId: string,
    patch: Partial<AccountDraft>,
  ) => {
    setAccountDrafts((current) => ({
      ...current,
      [ownerUserId]: {
        ...current[ownerUserId],
        ...patch,
      },
    }));
  }, []);

  const handleChangePlanDraft = useCallback((
    planKey: string,
    patch: Partial<PlanDraft>,
  ) => {
    setPlanDrafts((current) => ({
      ...current,
      [planKey]: {
        ...current[planKey],
        ...patch,
      },
    }));
  }, []);

  const handleSaveAccount = useCallback(async (ownerUserId: string) => {
    const draft = accountDrafts[ownerUserId];
    if (!draft) {
      return;
    }
    try {
      setSavingUserId(ownerUserId);
      const result = await updateUserSubscriptionApi(ownerUserId, {
        plan_key: draft.planKey,
      });
      setOverview(result);
      setAccountDrafts(buildAccountDrafts(result.accounts));
      setPlanDrafts(buildPlanDrafts(result.plans));
      setFeedback({
        tone: "success",
        title: t("settings.subscription.save_success_title"),
        message: t("settings.subscription.save_success_message"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.save_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.subscription.save_failed_message"),
      });
    } finally {
      setSavingUserId(null);
    }
  }, [accountDrafts, t]);

  const handleSavePlan = useCallback(async (planKey: string) => {
    const draft = planDrafts[planKey];
    if (!draft) {
      return;
    }
    const payload = buildPlanPayload(planKey, draft);
    if (!payload) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.plan_save_failed_title"),
        message: t("settings.subscription.plan_limit_invalid"),
      });
      return;
    }
    try {
      setSavingPlanKey(planKey);
      const result = await updateSubscriptionPlanApi(planKey, payload);
      setOverview(result);
      setAccountDrafts(buildAccountDrafts(result.accounts));
      setPlanDrafts(buildPlanDrafts(result.plans));
      setFeedback({
        tone: "success",
        title: t("settings.subscription.plan_save_success_title"),
        message: t("settings.subscription.plan_save_success_message"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.plan_save_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.subscription.plan_save_failed_message"),
      });
    } finally {
      setSavingPlanKey(null);
    }
  }, [planDrafts, t]);

  const handleCreatePlan = useCallback(async () => {
    const payload = buildPlanPayload(newPlanDraft.planKey, newPlanDraft);
    if (!payload) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.plan_create_failed_title"),
        message: t("settings.subscription.plan_limit_invalid"),
      });
      return;
    }
    try {
      setCreatingPlan(true);
      const result = await createSubscriptionPlanApi(payload);
      setOverview(result);
      setAccountDrafts(buildAccountDrafts(result.accounts));
      setPlanDrafts(buildPlanDrafts(result.plans));
      setNewPlanDraft(EMPTY_NEW_PLAN_DRAFT);
      setFeedback({
        tone: "success",
        title: t("settings.subscription.plan_create_success_title"),
        message: t("settings.subscription.plan_create_success_message"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.plan_create_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.subscription.plan_create_failed_message"),
      });
    } finally {
      setCreatingPlan(false);
    }
  }, [newPlanDraft, t]);

  const summary = useMemo(() => {
    const accounts = overview?.accounts ?? [];
    return {
      accountCount: accounts.length,
      planCount: overview?.plans.length ?? 0,
      usedTokens: accounts.reduce(
        (total, account) => total + account.used_tokens,
        0,
      ),
    };
  }, [overview]);

  const accounts = overview?.accounts ?? [];
  const activePlans = (overview?.plans ?? []).filter((plan) => plan.status !== "archived");
  const plans = overview?.plans ?? [];

  return (
    <>
      <div className={cn("mx-auto grid w-full gap-4 px-4 py-4 sm:px-6", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
        <section className="grid gap-1 px-1">
          <p className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
            {view === "plans"
              ? t("settings.subscription.plan_management_title")
              : t("settings.subscription.users_title")}
          </p>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {view === "plans"
              ? t("settings.subscription.plan_management_description")
              : t("settings.subscription.users_description")}
          </p>
        </section>

        {view === "users" ? (
          <section className={SETTINGS_CARD_CLASS_NAME}>
          <div className="grid divide-y divide-(--divider-subtle-color) md:grid-cols-3 md:divide-x md:divide-y-0">
            <div className="px-4 py-3">
              <div className="flex items-center gap-3">
                <div className={SETTINGS_ICON_CLASS_NAME}>
                  <UsersRound className="h-3.5 w-3.5" />
                </div>
                <div>
                  <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                    {formatTokenCount(summary.accountCount)}
                  </p>
                  <p className="mt-1 text-[11px] text-(--text-soft)">
                    {t("settings.subscription.accounts")}
                  </p>
                </div>
              </div>
            </div>
            <div className="px-4 py-3">
              <div className="flex items-center gap-3">
                <div className={SETTINGS_ICON_CLASS_NAME}>
                  <ShieldCheck className="h-3.5 w-3.5" />
                </div>
                <div>
                  <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                    {formatTokenCount(summary.planCount)}
                  </p>
                  <p className="mt-1 text-[11px] text-(--text-soft)">
                    {t("settings.subscription.plans")}
                  </p>
                </div>
              </div>
            </div>
            <div className="px-4 py-3">
              <div className="flex items-center gap-3">
                <div className={SETTINGS_ICON_CLASS_NAME}>
                  <Gauge className="h-3.5 w-3.5" />
                </div>
                <div>
                  <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                    {formatTokenCount(summary.usedTokens)}
                  </p>
                  <p className="mt-1 text-[11px] text-(--text-soft)">
                    {t("settings.subscription.current_month_usage")}
                  </p>
                </div>
              </div>
            </div>
          </div>
          </section>
        ) : null}

        {view === "plans" ? (
          <section className={SETTINGS_CARD_CLASS_NAME}>
            <div className="border-b border-(--divider-subtle-color) px-4 py-4">
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-[160px_minmax(160px,1fr)_160px_auto] xl:items-end">
                <label className="space-y-1.5">
                  <span className="text-[11px] font-semibold text-(--text-muted)">
                    {t("settings.subscription.plan_key")}
                  </span>
                  <input
                    className={CONTROL_CLASS_NAME}
                    disabled={creatingPlan}
                    onChange={(event) => setNewPlanDraft((current) => ({
                      ...current,
                      planKey: event.target.value,
                    }))}
                    placeholder={t("settings.subscription.plan_key_placeholder")}
                    value={newPlanDraft.planKey}
                  />
                </label>
                <label className="space-y-1.5">
                  <span className="text-[11px] font-semibold text-(--text-muted)">
                    {t("settings.subscription.display_name")}
                  </span>
                  <input
                    className={CONTROL_CLASS_NAME}
                    disabled={creatingPlan}
                    onChange={(event) => setNewPlanDraft((current) => ({
                      ...current,
                      displayName: event.target.value,
                    }))}
                    placeholder={t("settings.subscription.display_name_placeholder")}
                    value={newPlanDraft.displayName}
                  />
                </label>
                <label className="space-y-1.5">
                  <span className="text-[11px] font-semibold text-(--text-muted)">
                    {t("settings.subscription.plan_limit")}
                  </span>
                  <input
                    className={CONTROL_CLASS_NAME}
                    disabled={creatingPlan}
                    inputMode="numeric"
                    min={0}
                    onChange={(event) => setNewPlanDraft((current) => ({
                      ...current,
                      monthlyTokenLimit: event.target.value,
                    }))}
                    placeholder={t("settings.subscription.limit_unlimited")}
                    type="number"
                    value={newPlanDraft.monthlyTokenLimit}
                  />
                </label>
                <button
                  className={SAVE_BUTTON_CLASS_NAME}
                  disabled={creatingPlan}
                  onClick={() => {
                    void handleCreatePlan();
                  }}
                  type="button"
                >
                  {creatingPlan ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Plus className="h-3.5 w-3.5" />
                  )}
                  {t("settings.subscription.create_plan")}
                </button>
              </div>
            </div>

            {loading ? (
              <div className="flex items-center justify-center gap-2 px-4 py-10 text-[12px] text-(--text-soft)">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t("settings.subscription.loading")}
              </div>
            ) : plans.length === 0 ? (
              <div className="px-4 py-10 text-center text-[12px] text-(--text-soft)">
                {t("settings.subscription.plans_empty")}
              </div>
            ) : (
              <div className="divide-y divide-(--divider-subtle-color)">
                {plans.map((plan) => (
                  <SubscriptionPlanRow
                    key={plan.plan_key}
                    draft={planDrafts[plan.plan_key] ?? {
                      planKey: plan.plan_key,
                      displayName: plan.display_name,
                      status: normalizePlanStatus(plan.status),
                      monthlyTokenLimit: plan.monthly_token_limit === null ? "" : String(plan.monthly_token_limit),
                      notes: plan.notes,
                      sortOrder: String(plan.sort_order || 100),
                    }}
                    onChangeDraft={handleChangePlanDraft}
                    onSave={handleSavePlan}
                    plan={plan}
                    saving={savingPlanKey === plan.plan_key}
                    t={t}
                  />
                ))}
              </div>
            )}
          </section>
        ) : null}

        {view === "users" ? (
          <section className={SETTINGS_CARD_CLASS_NAME}>
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4 py-3">
              <div className="min-w-0">
                <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.subscription.users_title")}
                </p>
                <p className="mt-1 text-[11px] text-(--text-soft)">
                  {t("settings.subscription.period")}:{" "}
                  {formatDate(overview?.period_start ?? "")} -{" "}
                  {formatDate(overview?.period_end ?? "")}
                </p>
              </div>
              <button
                className={SECONDARY_BUTTON_CLASS_NAME}
                disabled={loading}
                onClick={() => {
                  void loadOverview();
                }}
                type="button"
              >
                {loading ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="h-3.5 w-3.5" />
                )}
                {t("settings.subscription.refresh")}
              </button>
            </div>

            {loading ? (
              <div className="flex items-center justify-center gap-2 px-4 py-10 text-[12px] text-(--text-soft)">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t("settings.subscription.loading")}
              </div>
            ) : accounts.length === 0 ? (
              <div className="px-4 py-10 text-center text-[12px] text-(--text-soft)">
                {t("settings.subscription.users_empty")}
              </div>
            ) : (
              <div className="divide-y divide-(--divider-subtle-color)">
                {accounts.map((account) => (
                  <SubscriptionAccountRow
                    key={account.owner_user_id}
                    account={account}
                    draft={accountDrafts[account.owner_user_id] ?? {
                      planKey: account.plan_key,
                    }}
                    onChangeDraft={handleChangeAccountDraft}
                    onSave={handleSaveAccount}
                    plans={activePlans.length > 0 ? activePlans : plans}
                    saving={savingUserId === account.owner_user_id}
                    t={t}
                  />
                ))}
              </div>
            )}
          </section>
        ) : null}
      </div>

      <FeedbackBannerStack
        items={feedback ? [{
          key: "subscription-feedback",
          message: feedback.message,
          onDismiss: () => setFeedback(null),
          title: feedback.title,
          tone: feedback.tone,
        }] : []}
      />
    </>
  );
}
