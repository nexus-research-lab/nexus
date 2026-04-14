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
import { request_api } from "@/lib/http";
import type {
  ProviderConfigPayload,
  ProviderConfigRecord,
  ProviderOptionsResponse,
  UpdateProviderConfigPayload,
} from "@/types/provider";

const PROVIDER_CONFIG_BASE_URL = `${getAgentApiBaseUrl()}/settings/providers`;

export async function listProviderConfigsApi(): Promise<ProviderConfigRecord[]> {
  return request_api<ProviderConfigRecord[]>(PROVIDER_CONFIG_BASE_URL, {
    method: "GET",
  });
}

export async function listProviderOptionsApi(): Promise<ProviderOptionsResponse> {
  return request_api<ProviderOptionsResponse>(`${PROVIDER_CONFIG_BASE_URL}/options`, {
    method: "GET",
  });
}

export async function createProviderConfigApi(payload: ProviderConfigPayload): Promise<ProviderConfigRecord> {
  return request_api<ProviderConfigRecord>(PROVIDER_CONFIG_BASE_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

export async function updateProviderConfigApi(
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

export async function deleteProviderConfigApi(provider: string): Promise<{ provider: string }> {
  return request_api<{ provider: string }>(`${PROVIDER_CONFIG_BASE_URL}/${encodeURIComponent(provider)}`, {
    method: "DELETE",
  });
}
