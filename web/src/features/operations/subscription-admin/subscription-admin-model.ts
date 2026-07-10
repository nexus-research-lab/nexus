import type {
  SubscriptionAccount,
  SubscriptionOverview,
  SubscriptionPlan,
  UpsertSubscriptionPlanPayload,
} from "@/types/settings/subscription";

export type PlanStatus = "active" | "archived";
export type SubscriptionAdminView = "users" | "plans";

export interface AccountDraft {
  planKey: string;
}

export interface PlanDraft {
  planKey: string;
  displayName: string;
  status: PlanStatus;
  monthlyTokenLimit: string;
  notes: string;
  sortOrder: string;
}

export interface FeedbackState {
  tone: "success" | "error";
  title: string;
  message: string;
}

export interface SubscriptionAdminSnapshot {
  overview: SubscriptionOverview | null;
  accountDrafts: Record<string, AccountDraft>;
  planDrafts: Record<string, PlanDraft>;
}

export interface SubscriptionSummary {
  accountCount: number;
  planCount: number;
  usedTokens: number;
}

export interface AccountViewModel {
  accounts: SubscriptionAccount[];
  drafts: Record<string, AccountDraft>;
  loading: boolean;
  mutationPending: boolean;
  periodEnd: string;
  periodStart: string;
  plans: SubscriptionPlan[];
  savingOwnerUserId: string | null;
  summary: SubscriptionSummary;
}

export interface PlanViewModel {
  creating: boolean;
  drafts: Record<string, PlanDraft>;
  loading: boolean;
  mutationPending: boolean;
  newPlanDraft: PlanDraft;
  plans: SubscriptionPlan[];
  savingPlanKey: string | null;
}

export type PendingSubscriptionMutation =
  | { kind: "account"; ownerUserId: string }
  | { kind: "plan"; planKey: string }
  | { kind: "create-plan" };

export const PLAN_STATUSES: PlanStatus[] = ["active", "archived"];

export const EMPTY_SUBSCRIPTION_SNAPSHOT: SubscriptionAdminSnapshot = {
  overview: null,
  accountDrafts: {},
  planDrafts: {},
};

const TOKEN_COUNT_FORMATTER = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 0,
});
const SHORT_DATE_FORMATTER = new Intl.DateTimeFormat(undefined, {
  month: "2-digit",
  day: "2-digit",
});

export function createEmptyPlanDraft(): PlanDraft {
  return {
    planKey: "",
    displayName: "",
    status: "active",
    monthlyTokenLimit: "",
    notes: "",
    sortOrder: "100",
  };
}

export function normalizePlanStatus(value: string): PlanStatus {
  return value === "archived" ? "archived" : "active";
}

export function createAccountDraft(account: SubscriptionAccount): AccountDraft {
  return { planKey: account.plan_key };
}

export function createPlanDraft(plan: SubscriptionPlan): PlanDraft {
  return {
    planKey: plan.plan_key,
    displayName: plan.display_name,
    status: normalizePlanStatus(plan.status),
    monthlyTokenLimit:
      plan.monthly_token_limit === null
        ? ""
        : String(plan.monthly_token_limit),
    notes: plan.notes,
    sortOrder: String(plan.sort_order),
  };
}

export function buildSubscriptionSnapshot(
  overview: SubscriptionOverview,
): SubscriptionAdminSnapshot {
  return {
    overview,
    accountDrafts: Object.fromEntries(
      overview.accounts.map((account) => [
        account.owner_user_id,
        createAccountDraft(account),
      ]),
    ),
    planDrafts: Object.fromEntries(
      overview.plans.map((plan) => [plan.plan_key, createPlanDraft(plan)]),
    ),
  };
}

export function buildSubscriptionSummary(
  accounts: SubscriptionAccount[],
  plans: SubscriptionPlan[],
): SubscriptionSummary {
  return {
    accountCount: accounts.length,
    planCount: plans.length,
    usedTokens: accounts.reduce(
      (total, account) => total + account.used_tokens,
      0,
    ),
  };
}

export function getSelectablePlans(plans: SubscriptionPlan[]): SubscriptionPlan[] {
  const activePlans = plans.filter((plan) => plan.status !== "archived");
  return activePlans.length > 0 ? activePlans : plans;
}

function parseMonthlyTokenLimit(
  value: string,
): { valid: true; value: number | null } | { valid: false } {
  const normalized = value.trim();
  if (!normalized) {
    return { valid: true, value: null };
  }
  const parsed = Number(normalized);
  if (!Number.isInteger(parsed) || parsed < 0) {
    return { valid: false };
  }
  return { valid: true, value: parsed };
}

function parseSortOrder(value: string): number {
  const parsed = Number(value.trim());
  return Number.isFinite(parsed) ? Math.trunc(parsed) : 100;
}

export function buildPlanPayload(
  planKey: string,
  draft: PlanDraft,
): UpsertSubscriptionPlanPayload | null {
  const monthlyTokenLimit = parseMonthlyTokenLimit(draft.monthlyTokenLimit);
  if (!monthlyTokenLimit.valid) {
    return null;
  }
  return {
    plan_key: planKey.trim(),
    display_name: draft.displayName.trim(),
    status: draft.status,
    monthly_token_limit: monthlyTokenLimit.value,
    notes: draft.notes.trim(),
    sort_order: parseSortOrder(draft.sortOrder),
  };
}

export function formatTokenCount(value: number): string {
  return TOKEN_COUNT_FORMATTER.format(value);
}

export function formatDate(value: string): string {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : SHORT_DATE_FORMATTER.format(date);
}

export function formatPercent(value: number | null): string {
  return value === null ? "--" : `${Math.round(value)}%`;
}

export function formatTokenLimit(
  value: number | null,
  unlimitedLabel: string,
): string {
  return value === null ? unlimitedLabel : formatTokenCount(value);
}
