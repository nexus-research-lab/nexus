/**
 * # !/usr/bin/env ts
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：provider-config-api.ts
 * # @Date   ：2026/04/14 14:54
 * # @Author ：leemysw
 * # 2026/04/14 14:54   Create
 * # =====================================================
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
import type {
  FetchProviderModelsResponse,
  ProviderConfigPayload,
  ProviderConfigRecord,
  ProviderModelRecord,
  ProviderOptionsResponse,
  ProviderPreset,
  ProviderTestResult,
  UpdateProviderModelPayload,
  UpdateProviderConfigPayload,
} from "@/types/capability/provider";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

const PROVIDER_CONFIG_BASE_URL = `${getAgentApiBaseUrl()}/settings/providers`;
const PROVIDER_PRESETS_URL = `${getAgentApiBaseUrl()}/settings/provider-presets`;
const SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL = `${getAgentApiBaseUrl()}/admin/subscription/providers`;

export interface DeleteProviderConfigResponse {
  provider: string;
  replacement_provider?: string;
  replacement_model?: string;
  reassigned_runtime_count?: number;
}

export async function listProviderConfigsApi(): Promise<
  ProviderConfigRecord[]
> {
  return requestApi<ProviderConfigRecord[]>(PROVIDER_CONFIG_BASE_URL, {
    method: "GET",
  });
}

export async function listSubscriptionProviderConfigsApi(): Promise<
  ProviderConfigRecord[]
> {
  return requestApi<ProviderConfigRecord[]>(
    SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL,
    {
      method: "GET",
    },
  );
}

export async function listProviderOptionsApi(
  runtimeKind?: AgentRuntimeKind | string,
): Promise<ProviderOptionsResponse> {
  const params = new URLSearchParams();
  const normalizedRuntimeKind = runtimeKind?.trim();
  if (normalizedRuntimeKind) {
    params.set("agent_runtime_kind", normalizedRuntimeKind);
  }
  const query = params.toString();
  return requestApi<ProviderOptionsResponse>(
    `${PROVIDER_CONFIG_BASE_URL}/options${query ? `?${query}` : ""}`,
    {
      method: "GET",
    },
  );
}

export async function listProviderPresetsApi(): Promise<ProviderPreset[]> {
  return requestApi<ProviderPreset[]>(PROVIDER_PRESETS_URL, {
    method: "GET",
  });
}

export async function createProviderConfigApi(
  payload: ProviderConfigPayload,
): Promise<ProviderConfigRecord> {
  return requestApi<ProviderConfigRecord>(PROVIDER_CONFIG_BASE_URL, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function createSubscriptionProviderConfigApi(
  payload: ProviderConfigPayload,
): Promise<ProviderConfigRecord> {
  return requestApi<ProviderConfigRecord>(SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updateProviderConfigApi(
  provider: string,
  payload: UpdateProviderConfigPayload,
): Promise<ProviderConfigRecord> {
  return requestApi<ProviderConfigRecord>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function updateSubscriptionProviderConfigApi(
  provider: string,
  payload: UpdateProviderConfigPayload,
): Promise<ProviderConfigRecord> {
  return requestApi<ProviderConfigRecord>(
    `${SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function fetchProviderModelsApi(
  provider: string,
): Promise<FetchProviderModelsResponse> {
  return requestApi<FetchProviderModelsResponse>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/fetch`,
    {
      method: "POST",
    },
  );
}

export async function fetchSubscriptionProviderModelsApi(
  provider: string,
): Promise<FetchProviderModelsResponse> {
  return requestApi<FetchProviderModelsResponse>(
    `${SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/fetch`,
    {
      method: "POST",
    },
  );
}

export async function updateProviderModelApi(
  provider: string,
  modelId: string,
  payload: UpdateProviderModelPayload,
): Promise<ProviderModelRecord> {
  return requestApi<ProviderModelRecord>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(modelId)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function updateSubscriptionProviderModelApi(
  provider: string,
  modelId: string,
  payload: UpdateProviderModelPayload,
): Promise<ProviderModelRecord> {
  return requestApi<ProviderModelRecord>(
    `${SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(modelId)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function testProviderConfigApi(
  provider: string,
): Promise<ProviderTestResult> {
  return requestApi<ProviderTestResult>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/test`,
    {
      method: "POST",
    },
  );
}

export async function testSubscriptionProviderConfigApi(
  provider: string,
): Promise<ProviderTestResult> {
  return requestApi<ProviderTestResult>(
    `${SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/test`,
    {
      method: "POST",
    },
  );
}

export async function testProviderModelApi(
  provider: string,
  modelId: string,
): Promise<ProviderTestResult> {
  return requestApi<ProviderTestResult>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(modelId)}/test`,
    {
      method: "POST",
    },
  );
}

export async function testSubscriptionProviderModelApi(
  provider: string,
  modelId: string,
): Promise<ProviderTestResult> {
  return requestApi<ProviderTestResult>(
    `${SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(modelId)}/test`,
    {
      method: "POST",
    },
  );
}

export async function deleteProviderConfigApi(
  provider: string,
  options: { force?: boolean } = {},
): Promise<DeleteProviderConfigResponse> {
  const searchParams = new URLSearchParams();
  if (options.force) {
    searchParams.set("force", "1");
  }
  const query = searchParams.toString();
  return requestApi<DeleteProviderConfigResponse>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}${query ? `?${query}` : ""}`,
    {
      method: "DELETE",
    },
  );
}

export async function deleteSubscriptionProviderConfigApi(
  provider: string,
  options: { force?: boolean } = {},
): Promise<DeleteProviderConfigResponse> {
  const searchParams = new URLSearchParams();
  if (options.force) {
    searchParams.set("force", "1");
  }
  const query = searchParams.toString();
  return requestApi<DeleteProviderConfigResponse>(
    `${SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}${query ? `?${query}` : ""}`,
    {
      method: "DELETE",
    },
  );
}
