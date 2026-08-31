/**
 * INPUT: Provider 配置、可选 configuration_version 与模型/测试命令。
 * OUTPUT: 旧响应体兼容的 Provider 结果；条件写通过强 If-Match 发送精确聚合版本。
 * POS: Provider/订阅 Provider HTTP transport 边界；版本不替代 owner、Provider key 或请求身份。
 */

import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  CCSwitchPreview,
  CCSwitchSyncPayload,
  CCSwitchSyncResult,
  DeleteProviderModelResponse,
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
const CC_SWITCH_IMPORT_URL = `${getAgentApiBaseUrl()}/settings/provider-imports/cc-switch`;
const SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL = `${getAgentApiBaseUrl()}/admin/subscription/providers`;

export interface DeleteProviderConfigResponse {
  provider: string;
  fallback_to_default?: boolean;
  affected_runtime_count?: number;
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

export async function previewCCSwitchApi(
  configDir?: string,
): Promise<CCSwitchPreview> {
  return requestApi<CCSwitchPreview>(`${CC_SWITCH_IMPORT_URL}/preview`, {
    method: "POST",
    body: JSON.stringify({ config_dir: configDir?.trim() || undefined }),
  });
}

export async function syncCCSwitchApi(
  payload: CCSwitchSyncPayload,
): Promise<CCSwitchSyncResult> {
  return requestApi<CCSwitchSyncResult>(`${CC_SWITCH_IMPORT_URL}/sync`, {
    method: "POST",
    body: JSON.stringify(payload),
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
  options?: { expectedVersion?: number },
): Promise<ProviderConfigRecord> {
  return requestApi<ProviderConfigRecord>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
      headers: providerIfMatchHeaders(options?.expectedVersion),
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

export async function deleteProviderModelApi(
  provider: string,
  modelId: string,
): Promise<DeleteProviderModelResponse> {
  return requestApi<DeleteProviderModelResponse>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(modelId)}`,
    { method: "DELETE" },
  );
}

export async function deleteSubscriptionProviderModelApi(
  provider: string,
  modelId: string,
): Promise<DeleteProviderModelResponse> {
  return requestApi<DeleteProviderModelResponse>(
    `${SUBSCRIPTION_PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(modelId)}`,
    { method: "DELETE" },
  );
}

export async function testProviderConfigApi(
  provider: string,
  options?: { expectedVersion?: number },
): Promise<ProviderTestResult> {
  return requestApi<ProviderTestResult>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/test`,
    {
      method: "POST",
      headers: providerIfMatchHeaders(options?.expectedVersion),
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
  options?: { expectedVersion?: number },
): Promise<ProviderTestResult> {
  return requestApi<ProviderTestResult>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(modelId)}/test`,
    {
      method: "POST",
      headers: providerIfMatchHeaders(options?.expectedVersion),
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
  options: { expectedVersion?: number; force?: boolean } = {},
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
      headers: providerIfMatchHeaders(options.expectedVersion),
    },
  );
}

function providerIfMatchHeaders(expectedVersion: number | undefined) {
  return Number.isSafeInteger(expectedVersion) && (expectedVersion ?? 0) > 0
    ? { "If-Match": `"provider-${expectedVersion}"` }
    : undefined;
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
