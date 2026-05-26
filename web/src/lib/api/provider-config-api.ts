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

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
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

const PROVIDER_CONFIG_BASE_URL = `${get_agent_api_base_url()}/settings/providers`;
const PROVIDER_PRESETS_URL = `${get_agent_api_base_url()}/settings/provider-presets`;

export async function list_provider_configs_api(): Promise<
  ProviderConfigRecord[]
> {
  return request_api<ProviderConfigRecord[]>(PROVIDER_CONFIG_BASE_URL, {
    method: "GET",
  });
}

export async function list_provider_options_api(): Promise<ProviderOptionsResponse> {
  return request_api<ProviderOptionsResponse>(
    `${PROVIDER_CONFIG_BASE_URL}/options`,
    {
      method: "GET",
    },
  );
}

export async function list_provider_presets_api(): Promise<ProviderPreset[]> {
  return request_api<ProviderPreset[]>(PROVIDER_PRESETS_URL, {
    method: "GET",
  });
}

export async function create_provider_config_api(
  payload: ProviderConfigPayload,
): Promise<ProviderConfigRecord> {
  return request_api<ProviderConfigRecord>(PROVIDER_CONFIG_BASE_URL, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function update_provider_config_api(
  provider: string,
  payload: UpdateProviderConfigPayload,
): Promise<ProviderConfigRecord> {
  return request_api<ProviderConfigRecord>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function fetch_provider_models_api(
  provider: string,
): Promise<FetchProviderModelsResponse> {
  return request_api<FetchProviderModelsResponse>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/fetch`,
    {
      method: "POST",
    },
  );
}

export async function update_provider_model_api(
  provider: string,
  model_id: string,
  payload: UpdateProviderModelPayload,
): Promise<ProviderModelRecord> {
  return request_api<ProviderModelRecord>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(model_id)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );
}

export async function set_default_provider_model_api(
  provider: string,
  model_id: string,
): Promise<ProviderModelRecord> {
  return request_api<ProviderModelRecord>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(model_id)}/default`,
    {
      method: "POST",
    },
  );
}

export async function test_provider_config_api(
  provider: string,
): Promise<ProviderTestResult> {
  return request_api<ProviderTestResult>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/test`,
    {
      method: "POST",
    },
  );
}

export async function test_provider_model_api(
  provider: string,
  model_id: string,
): Promise<ProviderTestResult> {
  return request_api<ProviderTestResult>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}/models/${encodeURIComponent(model_id)}/test`,
    {
      method: "POST",
    },
  );
}

export async function delete_provider_config_api(
  provider: string,
  options: { force?: boolean } = {},
): Promise<{
  provider: string;
  replacement_provider?: string;
  replacement_model?: string;
  reassigned_runtime_count?: number;
}> {
  const search_params = new URLSearchParams();
  if (options.force) {
    search_params.set("force", "1");
  }
  const query = search_params.toString();
  return request_api<{
    provider: string;
    replacement_provider?: string;
    replacement_model?: string;
    reassigned_runtime_count?: number;
  }>(
    `${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}${query ? `?${query}` : ""}`,
    {
      method: "DELETE",
    },
  );
}
