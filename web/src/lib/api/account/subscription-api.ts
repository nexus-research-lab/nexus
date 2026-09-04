import {
  getAgentApiBaseUrl,
  getControlAuthBaseUrl,
} from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  SubscriptionOverview,
  UpsertSubscriptionPlanPayload,
  UpdateUserSubscriptionPayload,
} from "@/types/settings/subscription";

const CONTROL_SUBSCRIPTION_BASE_URL = `${getControlAuthBaseUrl()}/subscription`;
const NEXUS_SUBSCRIPTION_USAGE_URL = `${getAgentApiBaseUrl()}/admin/subscription/usage`;

interface ControlSubscriptionAccount {
  user_id: string;
  username: string;
  display_name: string;
  role: string;
  user_status: string;
  plan_key: string;
  plan_name: string;
  monthly_token_limit: number | null;
  created_at: string;
  updated_at: string;
}

interface ControlSubscriptionOverview {
  plans: SubscriptionOverview["plans"];
  accounts: ControlSubscriptionAccount[];
  updated_at: string;
}

interface NexusSubscriptionUsage {
  accounts: Array<{
    control_user_id: string;
    used_tokens: number;
    session_count: number;
    message_count: number;
  }>;
  period_start: string;
  period_end: string;
  updated_at: string;
}

export async function getSubscriptionOverviewApi(): Promise<SubscriptionOverview> {
  const [control, usage] = await Promise.all([
    getControlSubscriptionOverview(),
    getNexusSubscriptionUsage(),
  ]);
  return mergeSubscriptionOverview(control, usage);
}

export async function updateUserSubscriptionApi(
  ownerUserId: string,
  payload: UpdateUserSubscriptionPayload,
): Promise<SubscriptionOverview> {
  const usage = await getNexusSubscriptionUsage();
  const control = await requestApi<ControlSubscriptionOverview>(
    `${CONTROL_SUBSCRIPTION_BASE_URL}/users/${encodeURIComponent(ownerUserId)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
  return mergeSubscriptionOverview(control, usage);
}

export async function createSubscriptionPlanApi(
  payload: UpsertSubscriptionPlanPayload,
): Promise<SubscriptionOverview> {
  const usage = await getNexusSubscriptionUsage();
  const control = await requestApi<ControlSubscriptionOverview>(`${CONTROL_SUBSCRIPTION_BASE_URL}/plans`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
  return mergeSubscriptionOverview(control, usage);
}

export async function updateSubscriptionPlanApi(
  planKey: string,
  payload: UpsertSubscriptionPlanPayload,
): Promise<SubscriptionOverview> {
  const usage = await getNexusSubscriptionUsage();
  const control = await requestApi<ControlSubscriptionOverview>(
    `${CONTROL_SUBSCRIPTION_BASE_URL}/plans/${encodeURIComponent(planKey)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
  return mergeSubscriptionOverview(control, usage);
}

function getControlSubscriptionOverview(): Promise<ControlSubscriptionOverview> {
  return requestApi<ControlSubscriptionOverview>(
    `${CONTROL_SUBSCRIPTION_BASE_URL}/overview`,
    { method: "GET" },
  );
}

function getNexusSubscriptionUsage(): Promise<NexusSubscriptionUsage> {
  return requestApi<NexusSubscriptionUsage>(NEXUS_SUBSCRIPTION_USAGE_URL, {
    method: "GET",
  });
}

function mergeSubscriptionOverview(
  control: ControlSubscriptionOverview,
  usage: NexusSubscriptionUsage,
): SubscriptionOverview {
  const usageByUser = new Map(
    usage.accounts.map((account) => [account.control_user_id, account]),
  );
  return {
    plans: control.plans,
    accounts: control.accounts.map((account) => {
      const local = usageByUser.get(account.user_id);
      const usedTokens = local?.used_tokens ?? 0;
      const limit = account.monthly_token_limit;
      return {
        owner_user_id: account.user_id,
        username: account.username,
        display_name: account.display_name,
        role: account.role,
        user_status: account.user_status,
        plan_key: account.plan_key,
        plan_name: account.plan_name,
        monthly_token_limit: limit,
        used_tokens: usedTokens,
        used_percent: limit !== null && limit > 0
          ? usedTokens / limit * 100
          : null,
        session_count: local?.session_count ?? 0,
        message_count: local?.message_count ?? 0,
        period_start: usage.period_start,
        period_end: usage.period_end,
        created_at: account.created_at,
        updated_at: account.updated_at,
      };
    }),
    period_start: usage.period_start,
    period_end: usage.period_end,
    updated_at: control.updated_at,
  };
}
