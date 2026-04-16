/**
 * # !/usr/bin/env ts
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：provider.ts
 * # @Date   ：2026/04/14 14:54
 * # @Author ：leemysw
 * # 2026/04/14 14:54   Create
 * # =====================================================
 */

export interface ProviderConfigRecord {
  id: string;
  provider: string;
  display_name: string;
  auth_token_masked: string;
  base_url: string;
  model: string;
  enabled: boolean;
  is_default: boolean;
  usage_count: number;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface ProviderOption {
  provider: string;
  display_name: string;
  is_default: boolean;
}

export interface ProviderOptionsResponse {
  default_provider: string | null;
  items: ProviderOption[];
}

export interface ProviderConfigPayload {
  provider: string;
  display_name: string;
  auth_token: string;
  base_url: string;
  model: string;
  enabled: boolean;
  is_default: boolean;
}

export interface UpdateProviderConfigPayload {
  display_name: string;
  auth_token?: string;
  base_url: string;
  model: string;
  enabled: boolean;
  is_default: boolean;
}

export function format_provider_label(provider?: string | null, display_name?: string | null): string {
  const normalized_display_name = display_name?.trim();
  if (normalized_display_name) {
    return normalized_display_name;
  }

  const normalized_provider = provider?.trim();
  if (!normalized_provider) {
    return "Provider";
  }

  return normalized_provider
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}
