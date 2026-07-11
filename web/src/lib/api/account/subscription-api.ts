import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/core/http";
import type {
  SubscriptionOverview,
  UpsertSubscriptionPlanPayload,
  UpdateUserSubscriptionPayload,
} from "@/types/settings/subscription";

const SUBSCRIPTION_ADMIN_BASE_URL = `${getAgentApiBaseUrl()}/admin/subscription`;

export async function getSubscriptionOverviewApi(): Promise<SubscriptionOverview> {
  return requestApi<SubscriptionOverview>(`${SUBSCRIPTION_ADMIN_BASE_URL}/overview`, {
    method: "GET",
  });
}

export async function updateUserSubscriptionApi(
  ownerUserId: string,
  payload: UpdateUserSubscriptionPayload,
): Promise<SubscriptionOverview> {
  return requestApi<SubscriptionOverview>(
    `${SUBSCRIPTION_ADMIN_BASE_URL}/users/${encodeURIComponent(ownerUserId)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function createSubscriptionPlanApi(
  payload: UpsertSubscriptionPlanPayload,
): Promise<SubscriptionOverview> {
  return requestApi<SubscriptionOverview>(`${SUBSCRIPTION_ADMIN_BASE_URL}/plans`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updateSubscriptionPlanApi(
  planKey: string,
  payload: UpsertSubscriptionPlanPayload,
): Promise<SubscriptionOverview> {
  return requestApi<SubscriptionOverview>(
    `${SUBSCRIPTION_ADMIN_BASE_URL}/plans/${encodeURIComponent(planKey)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}
