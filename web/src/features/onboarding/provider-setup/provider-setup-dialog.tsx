/**
 * INPUT: 模型服务目录、已有配置、连接验证与默认模型命令。
 * OUTPUT: 单栏 plain 模型服务连接向导。
 * POS: 首次使用的 Provider 配置边界；不承担品牌展示或功能营销。
 */
"use client";

import {
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  ArrowDownToLine,
  Check,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  ExternalLink,
  Loader2,
  Settings2,
} from "lucide-react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { getDefaultAgentRuntimeKind, setUserPreferences } from "@/config/runtime-options";
import { ProviderIcon } from "@/features/settings/provider-settings/components/provider-settings-icon";
import { ProviderCCSwitchDialog } from "@/features/provider-imports/cc-switch/provider-ccswitch-dialog";
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
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogNoteClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type {
  CCSwitchSyncResult,
  ProviderApiFormat,
  ProviderConfigRecord,
} from "@/types/capability/provider";

import {
  findManageablePresetProvider,
  listCustomProviderSetupFormats,
  listProviderSetupPresets,
  providerSetupModelIsRequired,
  selectInitialProviderSetupPreset,
  type ProviderSetupPreset,
} from "./provider-setup-model";

interface ProviderSetupDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onStart?: () => void;
}

type SetupScene = "provider" | "credentials" | "custom" | "verify" | "ready";
type JourneyPhase = "connect" | "discover" | "start";

interface SetupResult {
  model: string;
  provider: string;
}

interface ProviderConnectionDraft {
  apiFormat: ProviderApiFormat;
  apiKey: string;
  baseURL: string;
  displayName: string;
  existingProvider: ProviderConfigRecord | null;
  modelID: string;
  modelRequired: boolean;
  modelsPath: string;
  presetKey: string;
  providerKey: string;
}

const FEATURED_PROVIDER_COUNT = 4;
const DIALOG_TITLE_ID = "provider-setup-dialog-title";

export function ProviderSetupDialog({
  isOpen,
  onClose,
  onStart,
}: ProviderSetupDialogProps) {
  const { t } = useI18n();
  const runtimeKind = getDefaultAgentRuntimeKind();
  const canImportFromCCSwitch = isDesktopRuntime();
  const [scene, setScene] = useState<SetupScene>("provider");
  const [ccSwitchOpen, setCCSwitchOpen] = useState(false);
  const [presets, setPresets] = useState<ProviderSetupPreset[]>([]);
  const [customSetups, setCustomSetups] = useState<ProviderSetupPreset[]>([]);
  const [providers, setProviders] = useState<ProviderConfigRecord[]>([]);
  const [selectedPresetKey, setSelectedPresetKey] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [modelId, setModelId] = useState("");
  const [customProviderKey, setCustomProviderKey] = useState("");
  const [customProviderName, setCustomProviderName] = useState("");
  const [customApiFormat, setCustomApiFormat] = useState<ProviderApiFormat | "">("");
  const [customApiKey, setCustomApiKey] = useState("");
  const [customBaseUrl, setCustomBaseUrl] = useState("");
  const [customModelId, setCustomModelId] = useState("");
  const [showAllProviders, setShowAllProviders] = useState(false);
  const [verifyPhase, setVerifyPhase] = useState(0);
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
  const customSetup = useMemo(
    () => customSetups.find(
      (item) => item.format.api_format === customApiFormat,
    ) ?? null,
    [customApiFormat, customSetups],
  );
  const customExistingProvider = useMemo(
    () => providers.find((provider) => (
      provider.can_manage
      && provider.provider === customProviderKey
      && provider.preset_key === "custom"
    )) ?? null,
    [customProviderKey, providers],
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
    setScene("provider");
    setLoading(true);
    setBusy(false);
    setCCSwitchOpen(false);
    setError(null);
    setResult(null);
    setApiKey("");
    setBaseUrl("");
    setModelId("");
    setCustomProviderKey(createCustomProviderKey());
    setCustomProviderName("");
    setCustomApiFormat("");
    setCustomApiKey("");
    setCustomBaseUrl("");
    setCustomModelId("");
    setShowAllProviders(false);
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
      const nextCustomSetups = listCustomProviderSetupFormats(
        nextPresets,
        runtimeKind,
      );
      setPresets(setupPresets);
      setCustomSetups(nextCustomSetups);
      setProviders(nextProviders);
      setCustomApiFormat(selectInitialCustomAPIFormat(nextCustomSetups));
      // 优先接续用户已经开始配置的供应商，避免每次都退回目录第一项。
      const first = selectInitialProviderSetupPreset(
        setupPresets,
        nextProviders,
      );
      if (first) {
        const firstIndex = setupPresets.findIndex(
          (item) => item.preset.preset_key === first.preset.preset_key,
        );
        setSelectedPresetKey(first.preset.preset_key);
        setShowAllProviders(firstIndex >= FEATURED_PROVIDER_COUNT);
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

  useEffect(() => {
    if (scene !== "verify" || !busy) {
      return undefined;
    }
    setVerifyPhase(0);
    const discoverTimer = window.setTimeout(() => setVerifyPhase(1), 650);
    const defaultTimer = window.setTimeout(() => setVerifyPhase(2), 1350);
    return () => {
      window.clearTimeout(discoverTimer);
      window.clearTimeout(defaultTimer);
    };
  }, [busy, scene]);

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

  const handleBack = () => {
    setError(null);
    setScene("provider");
  };

  const submitConnection = (
    draft: ProviderConnectionDraft,
    failureScene: "credentials" | "custom",
  ) => {
    if (busy) {
      return;
    }
    const normalizedApiKey = draft.apiKey.trim();
    const normalizedBaseURL = draft.baseURL.trim();
    const normalizedModelID = draft.modelID.trim();
    const requiresAPIKey = !draft.existingProvider?.auth_token_masked?.trim();
    if (requiresAPIKey && !normalizedApiKey) {
      setError(t("onboarding.provider_setup_api_key_required"));
      return;
    }
    if (!normalizedBaseURL) {
      setError(t("onboarding.provider_setup_base_url_required"));
      return;
    }
    if (draft.modelRequired && !normalizedModelID) {
      setError(t("onboarding.provider_setup_model_required"));
      return;
    }

    setBusy(true);
    setScene("verify");
    setError(null);
    setResult(null);
    void persistAndTest({
      apiKey: normalizedApiKey,
      baseURL: normalizedBaseURL,
      displayName: draft.displayName.trim(),
      apiFormat: draft.apiFormat,
      modelsPath: draft.modelsPath,
      modelID: normalizedModelID,
      presetKey: draft.presetKey,
      providerKey: draft.providerKey,
      existingProvider: draft.existingProvider,
    }).then(async (testResult) => {
      const provider = testResult.provider.trim()
        || draft.existingProvider?.provider
        || draft.providerKey;
      const model = testResult.model?.trim() || normalizedModelID;
      if (!model) {
        throw new Error(t("onboarding.provider_setup_model_required"));
      }
      await persistDefaultModelSelections({ model, provider });
      setResult({
        model,
        provider: draft.displayName.trim(),
      });
      setScene("ready");
    }).catch(async (setupError: unknown) => {
      // 测试失败前 Provider 可能已经落库；刷新记录后允许用户原地修正并重试。
      try {
        setProviders(await listProviderConfigsApi());
      } catch {
        // 保留原错误作为主反馈，目录刷新失败不覆盖真实连接原因。
      }
      setError(getErrorMessage(
        setupError,
        t("onboarding.provider_setup_test_failed", {
          message: t("settings.providers.retry_later"),
        }),
      ));
      setScene(failureScene);
    }).finally(() => {
      setBusy(false);
    });
  };

  const handleSubmit = () => {
    if (!selected) {
      return;
    }
    submitConnection({
      apiFormat: selected.format.api_format,
      apiKey,
      baseURL: usesBuiltinEndpoint ? selected.format.base_url : baseUrl,
      displayName: selected.preset.display_name,
      existingProvider,
      modelID: modelId,
      modelRequired,
      modelsPath: selected.format.models_path,
      presetKey: selected.preset.preset_key,
      providerKey: existingProvider?.provider ?? selected.preset.preset_key,
    }, "credentials");
  };

  const handleCustomSubmit = () => {
    const displayName = customProviderName.trim();
    if (!displayName) {
      setError(t("onboarding.provider_setup_custom_name_required"));
      return;
    }
    if (!customSetup || !customApiFormat) {
      setError(t("onboarding.provider_setup_custom_format_required"));
      return;
    }
    submitConnection({
      apiFormat: customApiFormat,
      apiKey: customApiKey,
      baseURL: customBaseUrl,
      displayName,
      existingProvider: customExistingProvider,
      modelID: customModelId,
      modelRequired: true,
      modelsPath: customSetup.format.models_path,
      presetKey: "custom",
      providerKey: customExistingProvider?.provider ?? customProviderKey,
    }, "custom");
  };

  const handleCCSwitchSynced = async (syncResult: CCSwitchSyncResult) => {
    const selection = syncResult.default_selection;
    if (!selection) {
      throw new Error(t("onboarding.provider_setup_ccswitch_default_failed"));
    }
    await persistDefaultModelSelections({
      model: selection.model,
      provider: selection.provider,
    });
    setResult({
      model: selection.model_display_name || selection.model,
      provider: selection.provider_display_name || selection.provider,
    });
    setScene("ready");
  };

  const close = () => {
    if (!busy) {
      onClose();
    }
  };

  const start = () => {
    if (busy) {
      return;
    }
    onClose();
    onStart?.();
  };

  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop
          className="z-[11050]"
          closeOnBackdrop={!busy}
          labelledBy={DIALOG_TITLE_ID}
          onClose={close}
        >
          <UiDialogShell
            className="h-[min(620px,calc(100dvh-2rem))] !max-w-[620px]"
            size="lg"
          >
            <UiDialogHeader
              appearance="plain"
              closeLabel={t("common.close")}
              onClose={close}
              title={t("onboarding.provider_setup_title")}
              titleId={DIALOG_TITLE_ID}
            />

            <UiDialogBody className="!min-h-0 !flex-1 !overflow-hidden !p-0">
              <div
                className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden px-5 pt-4 sm:px-6"
                key={scene}
              >
                <JourneyProgress scene={scene} />
                <div
                  className="mt-4 flex min-h-0 flex-1 flex-col overflow-hidden animate-in fade-in-0 slide-in-from-bottom-1 duration-(--motion-duration-layout)"
                >
                  {scene === "provider" ? (
                    <ProviderScene
                        error={error}
                        loading={loading}
                        onContinue={() => {
                          if (selected) {
                            setError(null);
                            setScene("credentials");
                          }
                        }}
                        onCustom={() => {
                          setError(null);
                          setScene("custom");
                        }}
                        onImportCCSwitch={() => setCCSwitchOpen(true)}
                        onSelect={selectPreset}
                        onShowAllChange={setShowAllProviders}
                        presets={presets}
                        providers={providers}
                        selectedPresetKey={selectedPresetKey}
                        showAll={showAllProviders}
                        supportsCCSwitch={canImportFromCCSwitch}
                        supportsCustom={customSetups.length > 0}
                    />
                  ) : null}
                  {scene === "credentials" && selected ? (
                    <CredentialsScene
                        apiKey={apiKey}
                        apiKeyRequired={apiKeyRequired}
                        baseUrl={baseUrl}
                        error={error}
                        existingProvider={existingProvider}
                        modelId={modelId}
                        modelRequired={modelRequired}
                        onApiKeyChange={(value) => {
                          setApiKey(value);
                          setError(null);
                        }}
                        onBack={handleBack}
                        onBaseUrlChange={(value) => {
                          setBaseUrl(value);
                          setError(null);
                        }}
                        onModelIDChange={(value) => {
                          setModelId(value);
                          setError(null);
                        }}
                        onSubmit={handleSubmit}
                        setup={selected}
                    />
                  ) : null}
                  {scene === "custom" ? (
                    <CustomProviderScene
                        apiFormat={customApiFormat}
                        apiKey={customApiKey}
                        baseUrl={customBaseUrl}
                        error={error}
                        existingProvider={customExistingProvider}
                        formats={customSetups}
                        modelId={customModelId}
                        onApiFormatChange={(value) => {
                          const nextSetup = customSetups.find(
                            (item) => item.format.api_format === value,
                          );
                          if (nextSetup) {
                            setCustomApiFormat(nextSetup.format.api_format);
                            setError(null);
                          }
                        }}
                        onApiKeyChange={(value) => {
                          setCustomApiKey(value);
                          setError(null);
                        }}
                        onBack={handleBack}
                        onBaseUrlChange={(value) => {
                          setCustomBaseUrl(value);
                          setError(null);
                        }}
                        onModelIDChange={(value) => {
                          setCustomModelId(value);
                          setError(null);
                        }}
                        onNameChange={(value) => {
                          setCustomProviderName(value);
                          setError(null);
                        }}
                        onSubmit={handleCustomSubmit}
                        providerName={customProviderName}
                    />
                  ) : null}
                  {scene === "verify" ? <VerifyScene phase={verifyPhase} /> : null}
                  {scene === "ready" && result ? (
                    <ReadyScene
                      onStart={start}
                      result={result}
                    />
                  ) : null}
                </div>
              </div>
            </UiDialogBody>
          </UiDialogShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      {canImportFromCCSwitch ? (
        <ProviderCCSwitchDialog
          isOpen={ccSwitchOpen}
          onClose={() => setCCSwitchOpen(false)}
          onSynced={handleCCSwitchSynced}
          requireDefault
        />
      ) : null}
    </>
  );
}

function JourneyProgress({ scene }: { scene: SetupScene }) {
  const { t } = useI18n();
  const currentPhase = resolveJourneyPhase(scene);
  const phases: Array<{ id: JourneyPhase; label: string }> = [
    { id: "connect", label: t("onboarding.provider_setup_step_provider") },
    { id: "discover", label: t("onboarding.provider_setup_step_credentials") },
    { id: "start", label: t("onboarding.provider_setup_step_verify") },
  ];
  const currentIndex = phases.findIndex((phase) => phase.id === currentPhase);
  return (
    <div
      aria-label={`${currentIndex + 1} / ${phases.length} · ${phases[currentIndex]?.label ?? ""}`}
      className="flex h-5 items-center gap-4"
    >
      <div className="flex min-w-0 items-baseline gap-2">
        <span className="text-2xs tabular-nums text-(--text-muted)">
          {currentIndex + 1} / {phases.length}
        </span>
        <span className="truncate text-xs font-medium text-(--text-strong)">
          {phases[currentIndex]?.label}
        </span>
      </div>
      <div aria-hidden="true" className="ml-auto flex w-24 gap-1">
        {phases.map((phase, index) => (
          <span
            className={[
              "h-1 flex-1 rounded-full transition-colors duration-(--motion-duration-normal)",
              index <= currentIndex ? "bg-(--brand-action)" : "bg-(--divider-subtle-color)",
            ].join(" ")}
            key={phase.id}
          />
        ))}
      </div>
    </div>
  );
}

function ProviderScene({
  error,
  loading,
  onContinue,
  onCustom,
  onImportCCSwitch,
  onSelect,
  onShowAllChange,
  presets,
  providers,
  selectedPresetKey,
  showAll,
  supportsCCSwitch,
  supportsCustom,
}: {
  error: string | null;
  loading: boolean;
  onContinue: () => void;
  onCustom: () => void;
  onImportCCSwitch: () => void;
  onSelect: (preset: ProviderSetupPreset) => void;
  onShowAllChange: (showAll: boolean) => void;
  presets: ProviderSetupPreset[];
  providers: ProviderConfigRecord[];
  selectedPresetKey: string;
  showAll: boolean;
  supportsCCSwitch: boolean;
  supportsCustom: boolean;
}) {
  const { t } = useI18n();
  const visiblePresets = resolveVisiblePresets(presets, selectedPresetKey, showAll);
  return (
    <>
      <SceneMessage
        body={t("onboarding.provider_setup_provider_hint")}
        title={t("onboarding.provider_setup_provider_title")}
      />
      <div className="soft-scrollbar mt-5 min-h-0 flex-1 overflow-y-auto pr-1">
        {loading ? (
          <div className="flex min-h-40 items-center justify-center text-(--text-muted)">
            <Loader2 className="h-5 w-5 animate-spin" />
          </div>
        ) : null}
        {!loading && (error || presets.length === 0) ? (
          <div className={getDialogNoteClassName("danger")} role="alert">
            <div className="flex items-start gap-2">
              <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--destructive)" />
              <span>{error || t("onboarding.provider_setup_provider_empty")}</span>
            </div>
          </div>
        ) : null}
        {!loading && !error && presets.length > 0 ? (
          <div className="border-y border-(--divider-subtle-color)">
            {visiblePresets.map((item) => {
              const presetKey = item.preset.preset_key;
              const selected = presetKey === selectedPresetKey;
              const configured = Boolean(findManageablePresetProvider(providers, presetKey));
              return (
                <button
                  aria-pressed={selected}
                  className="group flex w-full items-center gap-3 border-b border-(--divider-subtle-color) px-1 py-2.5 text-left last:border-b-0"
                  key={presetKey}
                  onClick={() => onSelect(item)}
                  type="button"
                >
                  <ProviderIcon
                    name={item.preset.display_name}
                    presetKey={presetKey}
                    size="sm"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-(--text-strong)">
                      {item.preset.display_name}
                    </span>
                  </span>
                  {configured ? (
                    <span className="shrink-0 text-2xs font-medium text-(--success)">
                      {t("onboarding.provider_setup_provider_configured")}
                    </span>
                  ) : null}
                  <span className={selected ? "flex h-4 w-4 items-center justify-center rounded-full bg-(--brand-action) text-white" : "h-4 w-4 rounded-full border border-(--divider-strong-color)"}>
                    {selected ? <Check className="h-2.5 w-2.5" /> : null}
                  </span>
                </button>
              );
            })}
          </div>
        ) : null}
        {!loading && presets.length > FEATURED_PROVIDER_COUNT ? (
          <button
            className="mt-3 text-xs font-medium text-(--text-muted) hover:text-(--text-strong)"
            onClick={() => onShowAllChange(!showAll)}
            type="button"
          >
            {showAll
              ? t("onboarding.provider_setup_provider_show_less")
              : t("onboarding.provider_setup_provider_show_more", {
                count: Math.max(0, presets.length - FEATURED_PROVIDER_COUNT),
              })}
          </button>
        ) : null}
      </div>
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-t border-(--divider-subtle-color) pb-5 pt-3">
        <div className="mr-auto flex flex-wrap items-center gap-2">
          <UiButton
            disabled={!supportsCustom || loading}
            onClick={onCustom}
            size="sm"
            variant="surface"
          >
            <Settings2 className="h-3.5 w-3.5" />
            {t("onboarding.provider_setup_custom_action")}
          </UiButton>
          {supportsCCSwitch ? (
            <UiButton
              disabled={loading}
              onClick={onImportCCSwitch}
              size="sm"
              variant="surface"
            >
              <ArrowDownToLine className="h-3.5 w-3.5" />
              {t("onboarding.provider_setup_ccswitch_action")}
            </UiButton>
          ) : null}
        </div>
        <UiButton
          disabled={!selectedPresetKey || loading}
          onClick={onContinue}
          size="sm"
          tone="primary"
          variant="solid"
        >
          {t("onboarding.provider_setup_provider_continue")}
          <ChevronRight className="h-3.5 w-3.5" />
        </UiButton>
      </div>
    </>
  );
}

function CredentialsScene({
  apiKey,
  apiKeyRequired,
  baseUrl,
  error,
  existingProvider,
  modelId,
  modelRequired,
  onApiKeyChange,
  onBack,
  onBaseUrlChange,
  onModelIDChange,
  onSubmit,
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
  onBack: () => void;
  onBaseUrlChange: (value: string) => void;
  onModelIDChange: (value: string) => void;
  onSubmit: () => void;
  setup: ProviderSetupPreset;
}) {
  const { t } = useI18n();
  const apiKeyInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    // 场景切换不会重建模态框，主动把键盘焦点交给首要输入项。
    apiKeyInputRef.current?.focus({ preventScroll: true });
  }, []);

  return (
    <form
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <SceneMessage
        body={existingProvider
          ? t("onboarding.provider_setup_credentials_saved_description")
          : t("onboarding.provider_setup_credentials_description")}
        title={t("onboarding.provider_setup_credentials_title", {
          provider: setup.preset.display_name,
        })}
      />

      <div className="soft-scrollbar mt-5 min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
        <UiField
          description={existingProvider?.auth_token_masked && !apiKey
            ? t("onboarding.provider_setup_api_key_keep")
            : undefined}
          htmlFor="provider-setup-api-key"
          label={t("onboarding.provider_setup_api_key")}
          required={apiKeyRequired}
        >
          <UiInput
            ref={apiKeyInputRef}
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
            required
          >
            <UiInput
              autoCapitalize="off"
              autoCorrect="off"
              controlSize="md"
              id="provider-setup-base-url"
              onChange={(event) => onBaseUrlChange(event.target.value)}
              placeholder={setup.format.base_url_placeholder || t("onboarding.provider_setup_base_url_placeholder")}
              required
              spellCheck={false}
              type="url"
              value={baseUrl}
            />
          </UiField>
        ) : null}

        {modelRequired ? (
          <UiField
            htmlFor="provider-setup-model-id"
            label={t("onboarding.provider_setup_model")}
            required
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

      <div className="flex shrink-0 items-center justify-end gap-2 border-t border-(--divider-subtle-color) pb-5 pt-3">
        <UiButton onClick={onBack} size="sm" type="button" variant="text">
          <ChevronLeft className="h-3.5 w-3.5" />
          {t("onboarding.provider_setup_back")}
        </UiButton>
        <UiButton size="sm" tone="primary" type="submit" variant="solid">
          {t("onboarding.provider_setup_submit")}
        </UiButton>
      </div>
    </form>
  );
}

function CustomProviderScene({
  apiFormat,
  apiKey,
  baseUrl,
  error,
  existingProvider,
  formats,
  modelId,
  onApiFormatChange,
  onApiKeyChange,
  onBack,
  onBaseUrlChange,
  onModelIDChange,
  onNameChange,
  onSubmit,
  providerName,
}: {
  apiFormat: ProviderApiFormat | "";
  apiKey: string;
  baseUrl: string;
  error: string | null;
  existingProvider: ProviderConfigRecord | null;
  formats: ProviderSetupPreset[];
  modelId: string;
  onApiFormatChange: (value: string) => void;
  onApiKeyChange: (value: string) => void;
  onBack: () => void;
  onBaseUrlChange: (value: string) => void;
  onModelIDChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onSubmit: () => void;
  providerName: string;
}) {
  const { t } = useI18n();
  const formatOptions = formats.map((setup) => ({
    label: customAPIFormatLabel(setup.format.api_format),
    value: setup.format.api_format,
  }));
  return (
    <form
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <SceneMessage
        body={t("onboarding.provider_setup_custom_description")}
        title={t("onboarding.provider_setup_custom_title")}
      />

      <div className="soft-scrollbar mt-5 min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
        <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_160px]">
          <UiField
            htmlFor="provider-setup-custom-name"
            label={t("onboarding.provider_setup_custom_name")}
            required
          >
            <UiInput
              autoCapitalize="off"
              autoCorrect="off"
              controlSize="md"
              id="provider-setup-custom-name"
              onChange={(event) => onNameChange(event.target.value)}
              placeholder={t("onboarding.provider_setup_custom_name_placeholder")}
              required
              spellCheck={false}
              type="text"
              value={providerName}
            />
          </UiField>
          <UiField
            htmlFor="provider-setup-custom-format"
            label={t("onboarding.provider_setup_custom_format")}
            required
          >
            <UiSelectMenu
              ariaLabel={t("onboarding.provider_setup_custom_format")}
              className="w-full"
              id="provider-setup-custom-format"
              onChange={onApiFormatChange}
              options={formatOptions}
              placeholder={t("onboarding.provider_setup_custom_format_placeholder")}
              size="md"
              surface="dialog"
              value={apiFormat}
            />
          </UiField>
        </div>

        <UiField
          description={existingProvider?.auth_token_masked && !apiKey
            ? t("onboarding.provider_setup_api_key_keep")
            : undefined}
          htmlFor="provider-setup-custom-api-key"
          label={t("onboarding.provider_setup_api_key")}
          required={!existingProvider?.auth_token_masked?.trim()}
        >
          <UiInput
            autoCapitalize="off"
            autoComplete="off"
            autoCorrect="off"
            controlSize="md"
            data-form-type="other"
            data-lpignore="true"
            id="provider-setup-custom-api-key"
            name="provider-setup-custom-api-key"
            onChange={(event) => onApiKeyChange(event.target.value)}
            placeholder={t("onboarding.provider_setup_api_key_placeholder")}
            required={!existingProvider?.auth_token_masked?.trim()}
            spellCheck={false}
            type="password"
            value={apiKey}
          />
        </UiField>

        <UiField
          htmlFor="provider-setup-custom-base-url"
          label={t("onboarding.provider_setup_base_url")}
          required
        >
          <UiInput
            autoCapitalize="off"
            autoCorrect="off"
            controlSize="md"
            id="provider-setup-custom-base-url"
            onChange={(event) => onBaseUrlChange(event.target.value)}
            placeholder={t("onboarding.provider_setup_base_url_placeholder")}
            required
            spellCheck={false}
            type="url"
            value={baseUrl}
          />
        </UiField>

        <UiField
          htmlFor="provider-setup-custom-model-id"
          label={t("onboarding.provider_setup_model")}
          required
        >
          <UiInput
            autoCapitalize="off"
            autoCorrect="off"
            controlSize="md"
            id="provider-setup-custom-model-id"
            onChange={(event) => onModelIDChange(event.target.value)}
            placeholder={t("onboarding.provider_setup_model_placeholder")}
            required
            spellCheck={false}
            type="text"
            value={modelId}
          />
        </UiField>

        {error ? (
          <div className={getDialogNoteClassName("danger")} role="alert">
            <div className="flex items-start gap-2">
              <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--destructive)" />
              <span>{error}</span>
            </div>
          </div>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center justify-end gap-2 border-t border-(--divider-subtle-color) pb-5 pt-3">
        <UiButton onClick={onBack} size="sm" type="button" variant="text">
          <ChevronLeft className="h-3.5 w-3.5" />
          {t("onboarding.provider_setup_back")}
        </UiButton>
        <UiButton size="sm" tone="primary" type="submit" variant="solid">
          {t("onboarding.provider_setup_submit")}
        </UiButton>
      </div>
    </form>
  );
}

function VerifyScene({ phase }: { phase: number }) {
  const { t } = useI18n();
  const lines = [
    t("onboarding.provider_setup_verify_identity"),
    t("onboarding.provider_setup_verify_models"),
    t("onboarding.provider_setup_verify_default"),
  ];
  return (
    <>
      <SceneMessage
        body={t("onboarding.provider_setup_verify_description")}
        title={t("onboarding.provider_setup_verify_title")}
      />
      <div
        aria-live="polite"
        className="my-auto flex items-center gap-3 border-y border-(--divider-subtle-color) py-4"
        role="status"
      >
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-(--surface-muted-background)">
          <Loader2 className="h-3.5 w-3.5 animate-spin text-(--brand-action)" />
        </span>
        <span className="text-sm font-medium text-(--text-strong)">
          {lines[phase] ?? lines[lines.length - 1]}
        </span>
        <span className="ml-auto text-2xs tabular-nums text-(--text-muted)">
          {Math.min(phase + 1, lines.length)} / {lines.length}
        </span>
      </div>
    </>
  );
}

function ReadyScene({
  onStart,
  result,
}: {
  onStart: () => void;
  result: SetupResult;
}) {
  const { t } = useI18n();
  return (
    <>
      <SceneMessage
        body={t("onboarding.provider_setup_success", {
          model: result.model,
          provider: result.provider,
        })}
        title={t("onboarding.provider_setup_success_title")}
      />
      <div className="mt-auto flex shrink-0 justify-end border-t border-(--divider-subtle-color) pb-5 pt-3">
        <UiButton onClick={onStart} size="sm" tone="primary" variant="solid">
          {t("onboarding.provider_setup_enter_chat")}
          <ChevronRight className="h-3.5 w-3.5" />
        </UiButton>
      </div>
    </>
  );
}

function SceneMessage({
  body,
  title,
}: {
  body: string;
  title: string;
}) {
  return (
    <div>
      <h3 className="text-[22px] font-semibold tracking-[-0.02em] text-(--text-strong)">
        {title}
      </h3>
      <p className="mt-2 max-w-[42ch] text-sm leading-5 text-(--text-muted)">
        {body}
      </p>
    </div>
  );
}

function resolveJourneyPhase(scene: SetupScene): JourneyPhase {
  if (scene === "ready") {
    return "start";
  }
  if (scene === "credentials" || scene === "custom" || scene === "verify") {
    return "discover";
  }
  return "connect";
}

function resolveVisiblePresets(
  presets: ProviderSetupPreset[],
  selectedPresetKey: string,
  showAll: boolean,
): ProviderSetupPreset[] {
  if (showAll || presets.length <= FEATURED_PROVIDER_COUNT) {
    return presets;
  }
  const featured = presets.slice(0, FEATURED_PROVIDER_COUNT);
  const selected = presets.find((item) => item.preset.preset_key === selectedPresetKey);
  if (!selected || featured.some((item) => item.preset.preset_key === selectedPresetKey)) {
    return featured;
  }
  return [...featured.slice(0, FEATURED_PROVIDER_COUNT - 1), selected];
}

async function persistAndTest({
  apiFormat,
  apiKey,
  baseURL,
  displayName,
  modelsPath,
  modelID,
  presetKey,
  providerKey,
  existingProvider,
}: {
  apiFormat: ProviderApiFormat;
  apiKey: string;
  baseURL: string;
  displayName: string;
  modelsPath: string;
  modelID: string;
  presetKey: string;
  providerKey: string;
  existingProvider: ProviderConfigRecord | null;
}) {
  const basePayload = {
    api_format: apiFormat,
    base_url: baseURL,
    display_name: displayName,
    enabled: true,
    models_path: modelsPath,
    preset_key: presetKey,
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
      provider: providerKey,
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

function defaultModelID(provider: ProviderConfigRecord | null): string {
  return provider?.models.find((model) => model.is_default)?.model_id
    ?? provider?.models.find((model) => model.enabled)?.model_id
    ?? "";
}

function selectInitialCustomAPIFormat(
  setups: readonly ProviderSetupPreset[],
): ProviderApiFormat | "" {
  return setups.find(
    (setup) => setup.format.api_format === "chat_completions",
  )?.format.api_format
    ?? setups[0]?.format.api_format
    ?? "";
}

function customAPIFormatLabel(apiFormat: ProviderApiFormat): string {
  switch (apiFormat) {
    case "chat_completions":
      return "OpenAI Chat";
    case "responses":
      return "OpenAI Responses";
    case "anthropic_messages":
      return "Anthropic Messages";
    default:
      return apiFormat;
  }
}

function createCustomProviderKey(): string {
  const timestamp = Date.now().toString(36);
  const random = Math.random().toString(36).slice(2, 8);
  return `custom-${timestamp}-${random}`;
}

async function persistDefaultModelSelections({
  model,
  provider,
}: {
  model: string;
  provider: string;
}): Promise<void> {
  const currentPreferences = await getUserPreferencesApi();
  const selection = { model, provider };
  const savedPreferences = await updateUserPreferencesApi({
    default_agent_options: {
      ...currentPreferences.default_agent_options,
      model,
      provider,
    },
    default_background_model_selection: selection,
  });
  setUserPreferences(savedPreferences);
  invalidateProviderAvailability();
}
