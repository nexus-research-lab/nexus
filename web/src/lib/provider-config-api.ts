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
import { request_api } from "@/lib/http";
import type {
  ProviderConfigPayload,
  ProviderConfigRecord,
  ProviderOptionsResponse,
  UpdateProviderConfigPayload,
} from "@/types/provider";

const PROVIDER_CONFIG_BASE_URL = `${get_agent_api_base_url()}/settings/providers`;

export async function list_provider_configs_api(): Promise<ProviderConfigRecord[]> {
  return request_api<ProviderConfigRecord[]>(PROVIDER_CONFIG_BASE_URL, {
    method: "GET",
  });
}

export async function list_provider_options_api(): Promise<ProviderOptionsResponse> {
  return request_api<ProviderOptionsResponse>(`${PROVIDER_CONFIG_BASE_URL}/options`, {
    method: "GET",
  });
}

export async function create_provider_config_api(payload: ProviderConfigPayload): Promise<ProviderConfigRecord> {
  return request_api<ProviderConfigRecord>(PROVIDER_CONFIG_BASE_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

export async function update_provider_config_api(
  provider: string,
  payload: UpdateProviderConfigPayload,
): Promise<ProviderConfigRecord> {
  return request_api<ProviderConfigRecord>(`${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

export async function delete_provider_config_api(provider: string): Promise<{ provider: string }> {
  return request_api<{ provider: string }>(`${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}`, {
    method: "DELETE",
  });
}
