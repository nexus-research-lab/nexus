import {
  createProviderConfigApi,
  fetchProviderModelsApi,
  listProviderOptionsApi,
  listProviderConfigsApi,
  listProviderPresetsApi,
  testProviderConfigApi,
  updateProviderConfigApi,
  updateProviderModelApi,
} from "@/lib/api/settings/provider-api";
import {
  getUserPreferencesApi,
  updateUserPreferencesApi,
} from "@/lib/api/settings/preferences-api";
import type {
  ProviderConfigRecord,
  ProviderModelSelection,
  ProviderPreset,
  ProviderPresetFormat,
  UpdateProviderConfigPayload,
} from "@/types/capability/provider";

import { buildHomeOnboardingProviderKey } from "./home-agent-onboarding";

const HOME_ONBOARDING_PROVIDER_ORDER = [
  "anthropic",
  "openai",
  "deepseek",
  "qwen-token-plan",
  "minimax-token-plan",
  "glm-coding-plan",
  "kimi-code",
  "volcengine-coding-plan",
  "doubao",
  "dashscope",
  "modelscope",
] as const;

export interface HomeOnboardingProviderChoice {
  displayName: string;
  keyUrl: string;
  presetKey: string;
}

export interface HomeOnboardingProviderResult {
  model: string;
  models: ProviderModelSelection[];
  provider: string;
  success: boolean;
}

export interface HomeOnboardingModelResolution {
  choices: ProviderModelSelection[];
  defaultSelection: ProviderModelSelection | null;
  suggestedSelection: ProviderModelSelection | null;
}

export async function configureHomeOnboardingProvider(
  token: string,
  presetKey: string,
): Promise<HomeOnboardingProviderResult> {
  const normalizedToken = token.trim();
  const normalizedPresetKey = presetKey.trim();
  const onboardingProvider = buildHomeOnboardingProviderKey(
    normalizedPresetKey,
  );
  if (!normalizedToken || !normalizedPresetKey) {
    return failedResult(onboardingProvider);
  }

  const [configs, presets] = await Promise.all([
    listProviderConfigsApi(),
    listProviderPresetsApi(),
  ]);
  const preset = resolveOnboardingPreset(presets, normalizedPresetKey);
  const format = resolveOnboardingFormat(preset);
  const existing = configs.find((item) => (
    item.provider === onboardingProvider
    && item.visibility === "private"
    && item.can_manage
  )) ?? null;

  const pendingRecord = existing
    ? await updateOnboardingProviderToken(
      onboardingProvider,
      preset,
      format,
      normalizedToken,
    )
    : await createOrRecoverOnboardingProvider(
      onboardingProvider,
      preset,
      format,
      normalizedToken,
    );

  const testResult = await testProviderConfigApi(pendingRecord.provider);
  if (!testResult.success || !testResult.model?.trim()) {
    await clearRejectedToken(pendingRecord);
    return failedResult(onboardingProvider);
  }

  const modelId = testResult.model.trim();
  try {
    await fetchProviderModelsApi(pendingRecord.provider);
  } catch {
    // Connectivity already passed. Fall back to the models discovered during
    // validation when this Provider does not expose a model-list endpoint.
  }
  const enabledRecord = await updateProviderConfigApi(
    pendingRecord.provider,
    buildProviderPayload(preset, format, true),
  );
  const refreshedRecord = (await listProviderConfigsApi()).find(
    (item) => item.provider === enabledRecord.provider,
  ) ?? enabledRecord;
  const models = refreshedRecord.models.map((model) => ({
    model: model.model_id,
    model_display_name: model.display_name || model.model_id,
    provider: refreshedRecord.provider,
    provider_display_name:
      refreshedRecord.display_name || refreshedRecord.provider,
  }));
  const suggestedModel = models.find((model) => model.model === modelId)
    ?? models[0];
  if (!suggestedModel) {
    await clearRejectedToken(enabledRecord);
    return failedResult(onboardingProvider);
  }

  return {
    model: suggestedModel.model,
    models,
    provider: enabledRecord.provider,
    success: true,
  };
}

async function createOrRecoverOnboardingProvider(
  provider: string,
  preset: ProviderPreset,
  format: ProviderPresetFormat,
  authToken: string,
): Promise<ProviderConfigRecord> {
  try {
    return await createProviderConfigApi({
      provider_kind: "llm",
      provider,
      visibility: "private",
      preset_key: preset.preset_key,
      api_format: format.api_format,
      display_name: preset.display_name,
      auth_token: authToken,
      base_url: format.base_url,
      models_path: format.models_path,
      enabled: false,
    });
  } catch (createError) {
    // Several onboarding tabs can observe an empty provider list at the same
    // time. If another tab wins the create race, the current token must still
    // replace that record instead of surfacing a generic request error or
    // validating the stale credential written by the other tab.
    const concurrentRecord = (await listProviderConfigsApi()).find(
      (item) => (
        item.provider === provider
        && item.visibility === "private"
        && item.can_manage
      ),
    );
    if (!concurrentRecord) {
      throw createError;
    }
    return updateOnboardingProviderToken(
      provider,
      preset,
      format,
      authToken,
    );
  }
}

function updateOnboardingProviderToken(
  provider: string,
  preset: ProviderPreset,
  format: ProviderPresetFormat,
  authToken: string,
): Promise<ProviderConfigRecord> {
  return updateProviderConfigApi(
    provider,
    buildProviderPayload(preset, format, false, authToken),
  );
}

export async function listHomeOnboardingProviderChoices(): Promise<
  HomeOnboardingProviderChoice[]
> {
  const presets = await listProviderPresetsApi();
  const order = new Map<string, number>(
    HOME_ONBOARDING_PROVIDER_ORDER.map(
      (presetKey, index): [string, number] => [presetKey, index],
    ),
  );
  return presets
    .filter((preset) => (
      preset.provider_kind === "llm"
      && preset.preset_key !== "custom"
      && preset.formats.some((format) => (
        (!format.provider_kind || format.provider_kind === "llm")
        && (
          format.api_format === "anthropic_messages"
          || format.api_format === "chat_completions"
          || format.api_format === "responses"
        )
      ))
    ))
    .map((preset) => ({
      displayName: preset.display_name,
      keyUrl: preset.key_url,
      presetKey: preset.preset_key,
    }))
    .sort((left, right) => (
      (order.get(left.presetKey) ?? Number.MAX_SAFE_INTEGER)
      - (order.get(right.presetKey) ?? Number.MAX_SAFE_INTEGER)
      || left.displayName.localeCompare(right.displayName)
    ));
}

export async function resolveHomeOnboardingModelSelection(): Promise<
  ProviderModelSelection | null
> {
  const resolution = await resolveHomeOnboardingModelOptions();
  return resolution.defaultSelection ?? resolution.suggestedSelection;
}

export async function resolveHomeOnboardingModelOptions(
  onboardingPresetKey?: string | null,
): Promise<
  HomeOnboardingModelResolution
> {
  const [options, preferences, configs] = await Promise.all([
    listProviderOptionsApi(),
    getUserPreferencesApi(),
    listProviderConfigsApi(),
  ]);
  const availableChoices = options.items.flatMap((provider) =>
    provider.models.map((model) => ({
      model: model.model_id,
      model_display_name: model.display_name,
      provider: provider.provider,
      provider_display_name: provider.display_name,
    })),
  );
  const defaultProvider = preferences.default_agent_options.provider?.trim();
  const defaultModel = preferences.default_agent_options.model?.trim();
  const defaultSelection = defaultProvider && defaultModel
    ? availableChoices.find((choice) =>
      choice.provider === defaultProvider && choice.model === defaultModel,
    ) ?? null
    : null;
  const onboardingProvider = onboardingPresetKey
    ? buildHomeOnboardingProviderKey(onboardingPresetKey)
    : null;
  const onboardingRecord = configs.find((item) => (
    item.provider === onboardingProvider
    && item.enabled
    && Boolean(item.auth_token_masked.trim())
  ));
  const onboardingChoices = onboardingRecord?.models.map((model) => ({
    model: model.model_id,
    model_display_name: model.display_name || model.model_id,
    provider: onboardingRecord.provider,
    provider_display_name:
      onboardingRecord.display_name || onboardingRecord.provider,
  })) ?? [];
  const choices = onboardingChoices.length > 0
    ? onboardingChoices
    : availableChoices;
  const suggestedOnboardingModel = onboardingRecord?.models.find(
    (model) => model.is_default,
  ) ?? onboardingRecord?.models.find((model) => model.enabled)
    ?? onboardingRecord?.models[0];
  const suggestedProvider = options.items.find(
    (item) => item.provider === onboardingProvider,
  ) ?? options.items[0];
  const suggestedModel = suggestedProvider?.models.find((item) => item.is_default)
    ?? suggestedProvider?.models[0];
  const suggestedSelection = onboardingRecord && suggestedOnboardingModel
    ? onboardingChoices.find(
      (choice) => choice.model === suggestedOnboardingModel.model_id,
    ) ?? null
    : suggestedProvider && suggestedModel
    ? choices.find((choice) =>
      choice.provider === suggestedProvider.provider
      && choice.model === suggestedModel.model_id,
    ) ?? null
    : options.default_selection;
  return {
    choices,
    defaultSelection,
    suggestedSelection,
  };
}

export async function setHomeOnboardingDefaultModel(
  selection: Pick<ProviderModelSelection, "provider" | "model">,
): Promise<void> {
  const providerRecord = (await listProviderConfigsApi()).find(
    (record) => record.provider === selection.provider,
  );
  const modelRecord = providerRecord?.models.find(
    (model) => model.model_id === selection.model,
  );
  if (providerRecord?.can_manage && modelRecord) {
    await updateProviderModelApi(providerRecord.provider, modelRecord.model_id, {
      enabled: true,
      is_default: true,
      capabilities_override: modelRecord.capabilities_override,
      context_window: modelRecord.context_window,
      max_output_tokens: modelRecord.max_output_tokens,
      provider_options: modelRecord.provider_options,
    });
  }
  const preferences = await getUserPreferencesApi();
  await updateUserPreferencesApi({
    default_agent_options: {
      ...preferences.default_agent_options,
      provider: selection.provider,
      model: selection.model,
    },
  });
}

function resolveOnboardingPreset(
  presets: ProviderPreset[],
  presetKey: string,
): ProviderPreset {
  const preset = presets.find(
    (item) => item.preset_key === presetKey && item.provider_kind === "llm",
  );
  if (!preset) {
    throw new Error(`Nexus onboarding provider preset is unavailable: ${presetKey}`);
  }
  return preset;
}

function resolveOnboardingFormat(
  preset: ProviderPreset,
): ProviderPresetFormat {
  const format = preset.formats.find((item) => (
    item.api_format === preset.default_api_format
    && (!item.provider_kind || item.provider_kind === "llm")
  )) ?? preset.formats.find((item) => (
    !item.provider_kind || item.provider_kind === "llm"
  ));
  if (!format) {
    throw new Error("Nexus onboarding provider format is unavailable");
  }
  return format;
}

function buildProviderPayload(
  preset: ProviderPreset,
  format: ProviderPresetFormat,
  enabled: boolean,
  authToken?: string,
): UpdateProviderConfigPayload {
  const payload: UpdateProviderConfigPayload = {
    provider_kind: "llm",
    preset_key: preset.preset_key,
    api_format: format.api_format,
    display_name: preset.display_name,
    base_url: format.base_url,
    models_path: format.models_path,
    enabled,
  };
  return authToken === undefined
    ? payload
    : { ...payload, auth_token: authToken };
}

async function clearRejectedToken(
  record: ProviderConfigRecord,
): Promise<void> {
  try {
    await updateProviderConfigApi(record.provider, {
      provider_kind: record.provider_kind,
      preset_key: record.preset_key,
      api_format: record.api_format,
      display_name: record.display_name,
      auth_token: "",
      base_url: record.base_url,
      models_path: record.models_path,
      enabled: false,
    });
  } catch {
    // The original validation failure is more useful than cleanup feedback.
  }
}

function failedResult(provider: string): HomeOnboardingProviderResult {
  return {
    model: "",
    models: [],
    provider,
    success: false,
  };
}
