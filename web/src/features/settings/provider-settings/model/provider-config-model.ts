import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  ProviderConfigPayload,
  ProviderConfigRecord,
  ProviderPreset,
  UpdateProviderConfigPayload,
} from "@/types/capability/provider";

import {
  getPresetFormat,
  presetUsesBuiltinEndpoint,
} from "./provider-preset-model";
import type { ProviderDraft } from "./provider-settings-types";

type TranslateFn = (
  key: TranslationKey,
  params?: Record<string, string | number>,
) => string;

export function toProviderDraft(item: ProviderConfigRecord): ProviderDraft {
  return {
    provider_kind: item.provider_kind,
    provider: item.provider,
    preset_key: item.preset_key || "custom",
    api_format: item.api_format,
    display_name: item.display_name || item.provider,
    auth_token: "",
    base_url: item.base_url,
    models_path: item.models_path || "",
    enabled: item.enabled,
  };
}

export function getProviderTitle(item: ProviderConfigRecord): string {
  return item.display_name || item.provider;
}

export function isCustomProviderRecord(item: ProviderConfigRecord): boolean {
  return !item.preset_key || item.preset_key === "custom";
}

export function getUsageAgentTitle(
  agent: ProviderConfigRecord["used_by_agents"][number],
): string {
  return agent.display_name?.trim() || agent.name?.trim() || agent.agent_id;
}

function getEffectiveEndpointFormat(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
) {
  return presetUsesBuiltinEndpoint(preset)
    ? getPresetFormat(preset, draft.api_format)
    : null;
}

function getEffectiveBaseUrl(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): string {
  return getEffectiveEndpointFormat(draft, preset)?.base_url ?? draft.base_url;
}

export function getEffectiveModelsPath(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): string {
  return getEffectiveEndpointFormat(draft, preset)?.models_path
    ?? draft.models_path;
}

function buildProviderBasePayload(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): UpdateProviderConfigPayload {
  return {
    provider_kind: draft.provider_kind,
    preset_key: draft.preset_key,
    api_format: draft.api_format,
    display_name: draft.display_name.trim() || draft.provider.trim(),
    base_url: getEffectiveBaseUrl(draft, preset).trim(),
    models_path: getEffectiveModelsPath(draft, preset).trim(),
    enabled: draft.enabled,
  };
}

export function buildProviderUpdatePayload(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): UpdateProviderConfigPayload {
  const payload = buildProviderBasePayload(draft, preset);
  const authToken = draft.auth_token.trim();
  return authToken ? { ...payload, auth_token: authToken } : payload;
}

export function buildProviderCreatePayload(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  visibility: ProviderConfigRecord["visibility"],
): ProviderConfigPayload {
  return {
    ...buildProviderBasePayload(draft, preset),
    provider: draft.provider.trim(),
    visibility,
    auth_token: draft.auth_token.trim(),
    provider_kind: draft.provider_kind,
  };
}

export function buildProviderEnabledPayload(
  record: ProviderConfigRecord,
  enabled: boolean,
  authToken: string,
): UpdateProviderConfigPayload {
  const payload: UpdateProviderConfigPayload = {
    provider_kind: record.provider_kind,
    preset_key: record.preset_key,
    api_format: record.api_format,
    display_name: getProviderTitle(record),
    base_url: record.base_url,
    models_path: record.models_path || "",
    enabled,
  };
  const normalizedToken = authToken.trim();
  if (!enabled) {
    return { ...payload, auth_token: "" };
  }
  return normalizedToken ? { ...payload, auth_token: normalizedToken } : payload;
}

export function getProviderDraftError(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  isCreating: boolean,
  translate: TranslateFn,
): string | null {
  if (!draft.provider.trim()) {
    return translate(draft.preset_key === "custom"
      ? "settings.providers.validation_provider_name_required"
      : "settings.providers.validation_provider_required");
  }
  if (!getEffectiveBaseUrl(draft, preset).trim()) {
    return translate("settings.providers.validation_base_url_required");
  }
  if (!draft.api_format.trim()) {
    return translate("settings.providers.validation_api_format_required");
  }
  if (isCreating && !draft.auth_token.trim()) {
    return translate("settings.providers.validation_auth_token_required");
  }
  return null;
}

export function providerDraftHasChanges(
  draft: ProviderDraft,
  record: ProviderConfigRecord | null,
  preset: ProviderPreset | null,
): boolean {
  if (!record || draft.auth_token.trim()) {
    return true;
  }
  const draftValues = projectComparableDraft(draft, preset);
  const recordValues = projectComparableRecord(record);
  return PROVIDER_COMPARISON_FIELDS.some(
    (field) => draftValues[field] !== recordValues[field],
  );
}

interface ComparableProviderConfig {
  apiFormat: ProviderConfigRecord["api_format"];
  baseUrl: string;
  displayName: string;
  enabled: boolean;
  modelsPath: string;
  presetKey: string;
}

const PROVIDER_COMPARISON_FIELDS: readonly (keyof ComparableProviderConfig)[] = [
  "presetKey",
  "apiFormat",
  "displayName",
  "baseUrl",
  "modelsPath",
  "enabled",
];

function projectComparableDraft(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): ComparableProviderConfig {
  return {
    presetKey: draft.preset_key,
    apiFormat: draft.api_format,
    displayName: draft.display_name.trim() || draft.provider.trim(),
    baseUrl: getEffectiveBaseUrl(draft, preset).trim(),
    modelsPath: getEffectiveModelsPath(draft, preset).trim(),
    enabled: draft.enabled,
  };
}

function projectComparableRecord(
  record: ProviderConfigRecord,
): ComparableProviderConfig {
  return {
    presetKey: record.preset_key || "custom",
    apiFormat: record.api_format,
    displayName: getProviderTitle(record),
    baseUrl: record.base_url,
    modelsPath: record.models_path || "",
    enabled: record.enabled,
  };
}

export function providerHasActiveConfig(
  item: ProviderConfigRecord | null | undefined,
): boolean {
  return !!item?.enabled && !!item.auth_token_masked?.trim();
}

export function normalizeCustomProviderKey(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}
