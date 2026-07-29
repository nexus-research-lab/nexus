"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Check,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  ExternalLink,
  Loader2,
  PlugZap,
  ShieldCheck,
} from "lucide-react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getDefaultAgentRuntimeKind, setUserPreferences } from "@/config/runtime-options";
import { invalidateProviderAvailability } from "@/hooks/capability/use-provider-availability";
import {
  createProviderConfigApi,
  listProviderConfigsApi,
  listProviderPresetsApi,
  testProviderConfigApi,
  testProviderModelApi,
  updateProviderConfigApi,
} from "@/lib/api/settings/provider-api";
import {
  getUserPreferencesApi,
  updateUserPreferencesApi,
} from "@/lib/api/settings/preferences-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogNoteClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiButton } from "@/shared/ui/button/button";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import type {
  ProviderConfigRecord,
} from "@/types/capability/provider";

import {
  findManageablePresetProvider,
  listProviderSetupPresets,
  providerSetupModelIsRequired,
  selectInitialProviderSetupPreset,
  type ProviderSetupPreset,
} from "./provider-setup-model";

interface ProviderSetupDialogProps {
  isOpen: boolean;
  onClose: () => void;
}

type SetupStep = 1 | 2 | 3;

interface SetupResult {
  model: string;
  provider: string;
}

const DIALOG_TITLE_ID = "provider-setup-dialog-title";
const DIALOG_DESCRIPTION_ID = "provider-setup-dialog-description";

export function ProviderSetupDialog({
  isOpen,
  onClose,
}: ProviderSetupDialogProps) {
  const { t } = useI18n();
  const runtimeKind = getDefaultAgentRuntimeKind();
  const [step, setStep] = useState<SetupStep>(1);
  const [presets, setPresets] = useState<ProviderSetupPreset[]>([]);
  const [providers, setProviders] = useState<ProviderConfigRecord[]>([]);
  const [selectedPresetKey, setSelectedPresetKey] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [modelId, setModelId] = useState("");
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<SetupResult | null>(null);

  const selected = useMemo(
    () => presets.find((item) => item.preset.preset_key === selectedPresetKey) ?? null,
    [presets, selectedPresetKey],
  );
  const existingProvider = useMemo(
    () => selected
      ? findManageablePresetProvider(providers, selected.preset.preset_key)
      : null,
    [providers, selected],
  );
  const modelRequired = selected
    ? providerSetupModelIsRequired(selected.format, existingProvider)
    : false;
  const apiKeyRequired = !existingProvider?.auth_token_masked?.trim();
  const usesBuiltinEndpoint = selected?.preset.endpoint_mode === "fixed";

  useEffect(() => {
    if (!isOpen) {
      return undefined;
    }
    let cancelled = false;
    setStep(1);
    setLoading(true);
    setBusy(false);
    setError(null);
    setResult(null);
    setApiKey("");
    setBaseUrl("");
    setModelId("");
    void Promise.all([
      listProviderPresetsApi(),
      listProviderConfigsApi(),
    ]).then(([nextPresets, nextProviders]) => {
      if (cancelled) {
        return;
      }
      const setupPresets = listProviderSetupPresets(
        nextPresets,
        runtimeKind,
        nextProviders,
      );
      setPresets(setupPresets);
      setProviders(nextProviders);
      // 优先接续用户已经开始配置的供应商，避免每次都退回目录第一项。
      const first = selectInitialProviderSetupPreset(
        setupPresets,
        nextProviders,
      );
      if (first) {
        setSelectedPresetKey(first.preset.preset_key);
        setBaseUrl(first.format.base_url);
        const configured = findManageablePresetProvider(
          nextProviders,
          first.preset.preset_key,
        );
        setModelId(defaultModelID(configured));
        if (configured?.base_url && first.preset.endpoint_mode !== "fixed") {
          setBaseUrl(configured.base_url);
        }
      } else {
        setSelectedPresetKey("");
      }
    }).catch((loadError: unknown) => {
      if (!cancelled) {
        setError(getErrorMessage(loadError, t("onboarding.provider_setup_load_failed")));
      }
    }).finally(() => {
      if (!cancelled) {
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [isOpen, runtimeKind, t]);

  if (!isOpen) {
    return null;
  }

  const selectPreset = (preset: ProviderSetupPreset) => {
    setSelectedPresetKey(preset.preset.preset_key);
    setError(null);
    setResult(null);
    setApiKey("");
    const configured = findManageablePresetProvider(
      providers,
      preset.preset.preset_key,
    );
    setBaseUrl(
      configured?.base_url && preset.preset.endpoint_mode !== "fixed"
        ? configured.base_url
        : preset.format.base_url,
    );
    setModelId(defaultModelID(configured));
  };

  const handleNext = () => {
    if (step === 1 && selected) {
      setError(null);
      setStep(2);
    }
  };

  const handleBack = () => {
    setError(null);
    setStep(step === 3 ? 2 : 1);
  };

  const handleSubmit = () => {
    if (!selected || busy) {
      return;
    }
    const normalizedApiKey = apiKey.trim();
    const normalizedBaseURL = usesBuiltinEndpoint
      ? selected.format.base_url.trim()
      : baseUrl.trim();
    const normalizedModelID = modelId.trim();
    if (apiKeyRequired && !normalizedApiKey) {
      setError(t("onboarding.provider_setup_api_key_required"));
      return;
    }
    if (!normalizedBaseURL) {
      setError(t("onboarding.provider_setup_base_url_required"));
      return;
    }
    if (modelRequired && !normalizedModelID) {
      setError(t("onboarding.provider_setup_model_required"));
      return;
    }

    setBusy(true);
    setStep(3);
    setError(null);
    setResult(null);
    void persistAndTest({
      apiKey: normalizedApiKey,
      baseURL: normalizedBaseURL,
      modelID: normalizedModelID,
      setup: selected,
      existingProvider,
    }).then(async (testResult) => {
      const provider = testResult.provider.trim()
        || existingProvider?.provider
        || selected.preset.preset_key;
      const model = testResult.model?.trim() || normalizedModelID;
      if (!model) {
        throw new Error(t("onboarding.provider_setup_model_required"));
      }
      const currentPreferences = await getUserPreferencesApi();
      const savedPreferences = await updateUserPreferencesApi({
        default_agent_options: {
          ...currentPreferences.default_agent_options,
          model,
          provider,
        },
      });
      setUserPreferences(savedPreferences);
      invalidateProviderAvailability();
      setResult({
        model,
        provider: selected.preset.display_name,
      });
    }).catch((setupError: unknown) => {
      setError(getErrorMessage(
        setupError,
        t("onboarding.provider_setup_test_failed", {
          message: t("settings.providers.retry_later"),
        }),
      ));
      setStep(2);
    }).finally(() => {
      setBusy(false);
    });
  };

  const close = () => {
    if (!busy) {
      onClose();
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[11050]"
        closeOnBackdrop={!busy}
        describedBy={DIALOG_DESCRIPTION_ID}
        labelledBy={DIALOG_TITLE_ID}
        onClose={close}
      >
        <div className="w-full max-w-xl">
          <UiDialogShell>
            <UiDialogHeader
              closeLabel={t("common.close")}
              onClose={close}
            >
              <div className="min-w-0 flex-1">
                <h2
                  className="dialog-title"
                  id={DIALOG_TITLE_ID}
                >
                  {t("onboarding.provider_setup_title")}
                </h2>
                <p
                  className="dialog-subtitle"
                  id={DIALOG_DESCRIPTION_ID}
                >
                  {t("onboarding.provider_setup_description")}
                </p>
              </div>
            </UiDialogHeader>

            <SetupProgress step={step} />

            <UiDialogBody className="!px-5 !py-4" scrollable>
              {step === 1 ? (
                <ProviderStep
                  error={error}
                  loading={loading}
                  onSelect={selectPreset}
                  presets={presets}
                  selectedPresetKey={selectedPresetKey}
                />
              ) : null}
              {step === 2 && selected ? (
                <CredentialsStep
                  apiKey={apiKey}
                  apiKeyRequired={apiKeyRequired}
                  baseUrl={baseUrl}
                  error={error}
                  existingProvider={existingProvider}
                  modelId={modelId}
                  modelRequired={modelRequired}
                  onApiKeyChange={setApiKey}
                  onBaseUrlChange={setBaseUrl}
                  onModelIDChange={setModelId}
                  setup={selected}
                />
              ) : null}
              {step === 3 ? (
                <VerifyStep
                  busy={busy}
                  result={result}
                />
              ) : null}
            </UiDialogBody>

            <UiDialogFooter className="!px-5 !py-3">
              {step === 1 ? (
                <Link
                  className="mr-auto inline-flex items-center gap-1 text-xs font-medium text-(--text-muted) hover:text-(--text-strong) hover:underline"
                  to={AppRouteBuilders.settings("providers")}
                  onClick={close}
                >
                  {t("onboarding.provider_setup_advanced")}
                  <ExternalLink className="h-3 w-3" />
                </Link>
              ) : (
                <UiButton
                  disabled={busy}
                  onClick={handleBack}
                  size="xs"
                  variant="text"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                  {t("onboarding.provider_setup_back")}
                </UiButton>
              )}

              {step === 1 ? (
                <UiButton
                  disabled={!selected || loading}
                  onClick={handleNext}
                  size="sm"
                  tone="primary"
                  variant="solid"
                >
                  {t("onboarding.provider_setup_next")}
                  <ChevronRight className="h-3.5 w-3.5" />
                </UiButton>
              ) : null}
              {step === 2 ? (
                <UiButton
                  disabled={busy || !selected}
                  onClick={handleSubmit}
                  size="sm"
                  tone="primary"
                  variant="solid"
                >
                  <PlugZap className="h-3.5 w-3.5" />
                  {t("onboarding.provider_setup_submit")}
                </UiButton>
              ) : null}
              {step === 3 && result ? (
                <UiButton
                  onClick={close}
                  size="sm"
                  tone="primary"
                  variant="solid"
                >
                  {t("onboarding.provider_setup_enter_chat")}
                </UiButton>
              ) : null}
            </UiDialogFooter>
          </UiDialogShell>
        </div>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function SetupProgress({ step }: { step: SetupStep }) {
  const { t } = useI18n();
  const steps = [
    t("onboarding.provider_setup_step_provider"),
    t("onboarding.provider_setup_step_credentials"),
    t("onboarding.provider_setup_step_verify"),
  ];
  return (
    <div className="grid grid-cols-3 gap-2 border-b border-(--divider-subtle-color) px-5 py-3">
      {steps.map((label, index) => {
        const stepNumber = index + 1;
        const active = stepNumber === step;
        const completed = stepNumber < step;
        return (
          <div
            aria-current={active ? "step" : undefined}
            className="flex min-w-0 items-center gap-1.5"
            key={label}
          >
            <span
              className={[
                "flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-2xs font-semibold",
                active || completed
                  ? "bg-(--brand-action) text-white"
                  : "bg-(--surface-muted-background) text-(--text-muted)",
              ].join(" ")}
            >
              {completed ? <Check className="h-3 w-3" /> : stepNumber}
            </span>
            <span
              className={[
                "min-w-0 truncate text-2xs font-medium",
                active ? "text-(--text-strong)" : "text-(--text-muted)",
              ].join(" ")}
            >
              {label}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function ProviderStep({
  error,
  loading,
  onSelect,
  presets,
  selectedPresetKey,
}: {
  error: string | null;
  loading: boolean;
  onSelect: (preset: ProviderSetupPreset) => void;
  presets: ProviderSetupPreset[];
  selectedPresetKey: string;
}) {
  const { t } = useI18n();
  if (loading) {
    return (
      <div className="flex min-h-40 items-center justify-center text-(--text-muted)">
        <Loader2 className="h-5 w-5 animate-spin" />
      </div>
    );
  }
  if (error || presets.length === 0) {
    return (
      <div className={getDialogNoteClassName("danger")} role="alert">
        <div className="flex items-start gap-2">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--destructive)" />
          <span>{error || t("onboarding.provider_setup_provider_empty")}</span>
        </div>
      </div>
    );
  }
  return (
    <div className="space-y-3">
      <p className="text-xs leading-5 text-(--text-muted)">
        {t("onboarding.provider_setup_provider_hint")}
      </p>
      <div className="grid gap-2 sm:grid-cols-2">
        {presets.map((item) => {
          const selected = item.preset.preset_key === selectedPresetKey;
          return (
            <button
              aria-pressed={selected}
              className={[
                "flex min-h-20 flex-col items-start gap-1 rounded-[12px] border px-3 py-2.5 text-left transition-[background,border-color,color] duration-(--motion-duration-fast)",
                selected
                  ? "border-(--brand-action) bg-[color:color-mix(in_srgb,var(--brand)_8%,transparent)] text-(--text-strong)"
                  : "border-(--divider-subtle-color) text-(--text-default) hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
              ].join(" ")}
              key={item.preset.preset_key}
              onClick={() => onSelect(item)}
              type="button"
            >
              <span className="text-sm font-semibold">
                {item.preset.display_name}
              </span>
              <span className="line-clamp-2 text-xs leading-4 text-(--text-muted)">
                {item.preset.description}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function CredentialsStep({
  apiKey,
  apiKeyRequired,
  baseUrl,
  error,
  existingProvider,
  modelId,
  modelRequired,
  onApiKeyChange,
  onBaseUrlChange,
  onModelIDChange,
  setup,
}: {
  apiKey: string;
  apiKeyRequired: boolean;
  baseUrl: string;
  error: string | null;
  existingProvider: ProviderConfigRecord | null;
  modelId: string;
  modelRequired: boolean;
  onApiKeyChange: (value: string) => void;
  onBaseUrlChange: (value: string) => void;
  onModelIDChange: (value: string) => void;
  setup: ProviderSetupPreset;
}) {
  const { t } = useI18n();
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <div className="flex h-8 w-8 items-center justify-center radius-control-sm bg-(--surface-interactive-hover-background) text-(--brand-action)">
          <ShieldCheck className="h-4 w-4" />
        </div>
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-(--text-strong)">
            {setup.preset.display_name}
          </h3>
          <p className="text-xs text-(--text-muted)">
            {existingProvider
              ? t("onboarding.provider_setup_api_key_keep")
              : t("onboarding.provider_setup_description")}
          </p>
        </div>
      </div>

      <UiField
        description={existingProvider?.auth_token_masked && !apiKey
          ? t("onboarding.provider_setup_api_key_keep")
          : undefined}
        htmlFor="provider-setup-api-key"
        label={t("onboarding.provider_setup_api_key")}
      >
        <UiInput
          autoCapitalize="off"
          autoComplete="off"
          autoCorrect="off"
          controlSize="md"
          data-form-type="other"
          data-lpignore="true"
          id="provider-setup-api-key"
          name="provider-setup-api-key"
          onChange={(event) => onApiKeyChange(event.target.value)}
          placeholder={t("onboarding.provider_setup_api_key_placeholder")}
          required={apiKeyRequired}
          spellCheck={false}
          type="password"
          value={apiKey}
        />
        {setup.preset.key_url ? (
          <a
            className="mt-1 inline-flex items-center gap-1 text-2xs font-medium text-(--brand-action) hover:underline"
            href={setup.preset.key_url}
            rel="noreferrer"
            target="_blank"
          >
            {t("onboarding.provider_setup_get_api_key")}
            <ExternalLink className="h-3 w-3" />
          </a>
        ) : null}
      </UiField>

      {setup.preset.endpoint_mode !== "fixed" ? (
        <UiField
          htmlFor="provider-setup-base-url"
          label={t("onboarding.provider_setup_base_url")}
        >
          <UiInput
            autoCapitalize="off"
            autoCorrect="off"
            controlSize="md"
            id="provider-setup-base-url"
            onChange={(event) => onBaseUrlChange(event.target.value)}
            placeholder={
              setup.format.base_url_placeholder
              || t("onboarding.provider_setup_base_url_placeholder")
            }
            required
            spellCheck={false}
            type="url"
            value={baseUrl}
          />
        </UiField>
      ) : null}

      {modelRequired ? (
        <UiField
          description={t("onboarding.provider_setup_model_hint")}
          htmlFor="provider-setup-model-id"
          label={t("onboarding.provider_setup_model")}
        >
          <UiInput
            autoCapitalize="off"
            autoCorrect="off"
            controlSize="md"
            id="provider-setup-model-id"
            onChange={(event) => onModelIDChange(event.target.value)}
            placeholder={t("onboarding.provider_setup_model_placeholder")}
            required
            spellCheck={false}
            type="text"
            value={modelId}
          />
        </UiField>
      ) : null}

      {error ? (
        <div className={getDialogNoteClassName("danger")} role="alert">
          <div className="flex items-start gap-2">
            <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--destructive)" />
            <span>{error}</span>
          </div>
        </div>
      ) : null}

    </div>
  );
}

function VerifyStep({
  busy,
  result,
}: {
  busy: boolean;
  result: SetupResult | null;
}) {
  const { t } = useI18n();
  if (busy) {
    return (
      <div
        aria-live="polite"
        className="flex min-h-40 flex-col items-center justify-center gap-3 text-center"
        role="status"
      >
        <Loader2 className="h-6 w-6 animate-spin text-(--brand-action)" />
        <p className="text-sm font-medium text-(--text-strong)">
          {t("onboarding.provider_setup_testing")}
        </p>
      </div>
    );
  }
  if (!result) {
    return null;
  }
  return (
    <div
      aria-live="polite"
      className="flex min-h-40 flex-col items-center justify-center gap-3 text-center"
      role="status"
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--success)_14%,transparent)] text-(--success)">
        <Check className="h-5 w-5" />
      </div>
      <div>
        <h3 className="text-sm font-semibold text-(--text-strong)">
          {t("onboarding.provider_setup_success_title")}
        </h3>
        <p className="mt-1 text-xs leading-5 text-(--text-muted)">
          {t("onboarding.provider_setup_success", {
            model: result.model,
            provider: result.provider,
          })}
        </p>
      </div>
    </div>
  );
}

async function persistAndTest({
  apiKey,
  baseURL,
  modelID,
  setup,
  existingProvider,
}: {
  apiKey: string;
  baseURL: string;
  modelID: string;
  setup: ProviderSetupPreset;
  existingProvider: ProviderConfigRecord | null;
}) {
  const basePayload = {
    api_format: setup.format.api_format,
    base_url: baseURL,
    display_name: setup.preset.display_name,
    enabled: true,
    models_path: setup.format.models_path,
    preset_key: setup.preset.preset_key,
    provider_kind: "llm" as const,
  };
  const record = existingProvider
    ? await updateProviderConfigApi(existingProvider.provider, {
      ...basePayload,
      ...(apiKey ? { auth_token: apiKey } : {}),
    })
    : await createProviderConfigApi({
      ...basePayload,
      auth_token: apiKey,
      provider: setup.preset.preset_key,
      visibility: "private",
    });
  const testResult = modelID
    ? await testProviderModelApi(record.provider, modelID)
    : await testProviderConfigApi(record.provider);
  if (!testResult.success) {
    throw new Error(testResult.error || "Provider 测试失败");
  }
  return testResult;
}

function defaultModelID(
  provider: ProviderConfigRecord | null,
): string {
  return provider?.models.find((model) => model.is_default)?.model_id
    ?? provider?.models.find((model) => model.enabled)?.model_id
    ?? "";
}
