"use client";

import {
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  Check,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  Clock3,
  ExternalLink,
  Loader2,
  MessageSquare,
  PlugZap,
  Puzzle,
  UsersRound,
} from "lucide-react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getDefaultAgentRuntimeKind, setUserPreferences } from "@/config/runtime-options";
import { ProviderIcon } from "@/features/settings/provider-settings/components/provider-settings-icon";
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
import type { ProviderConfigRecord } from "@/types/capability/provider";

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
  onStart?: () => void;
}

type SetupScene = "welcome" | "provider" | "credentials" | "verify" | "ready";
type JourneyPhase = "connect" | "discover" | "start";

interface SetupResult {
  model: string;
  provider: string;
}

const FEATURED_PROVIDER_COUNT = 4;
const DIALOG_TITLE_ID = "provider-setup-dialog-title";
const DIALOG_DESCRIPTION_ID = "provider-setup-dialog-description";

export function ProviderSetupDialog({
  isOpen,
  onClose,
  onStart,
}: ProviderSetupDialogProps) {
  const { t } = useI18n();
  const runtimeKind = getDefaultAgentRuntimeKind();
  const [scene, setScene] = useState<SetupScene>("welcome");
  const [presets, setPresets] = useState<ProviderSetupPreset[]>([]);
  const [providers, setProviders] = useState<ProviderConfigRecord[]>([]);
  const [selectedPresetKey, setSelectedPresetKey] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [modelId, setModelId] = useState("");
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
    setScene("welcome");
    setLoading(true);
    setBusy(false);
    setError(null);
    setResult(null);
    setApiKey("");
    setBaseUrl("");
    setModelId("");
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
      setPresets(setupPresets);
      setProviders(nextProviders);
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
    if (scene === "credentials") {
      setScene("provider");
      return;
    }
    setScene("welcome");
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
    setScene("verify");
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
      setScene("credentials");
    }).finally(() => {
      setBusy(false);
    });
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
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[11050]"
        closeOnBackdrop={!busy}
        describedBy={DIALOG_DESCRIPTION_ID}
        labelledBy={DIALOG_TITLE_ID}
        onClose={close}
      >
        <div className="w-full max-w-[620px]">
          <UiDialogShell className="h-[500px] max-h-[calc(100dvh-2rem)]" size="lg">
            <UiDialogHeader className="!h-12 !border-b-0 !px-5 !py-0" closeLabel={t("common.close")} onClose={close}>
              <div className="flex min-w-0 flex-1 items-center gap-2.5">
                <img alt="" className="h-6 w-6" src="/logo.webp" />
                <h2 className="text-xs font-semibold text-(--text-strong)" id={DIALOG_TITLE_ID}>
                  {t("onboarding.provider_setup_title")}
                </h2>
                <p className="sr-only" id={DIALOG_DESCRIPTION_ID}>
                  {t("onboarding.provider_setup_description")}
                </p>
              </div>
            </UiDialogHeader>

            <UiDialogBody className="!min-h-0 !flex-1 !overflow-hidden !p-0">
              <div className="grid h-full min-h-0 w-full md:grid-cols-[176px_minmax(0,1fr)]">
                <NexusPresence ready={scene === "ready"} />
                <div
                  className="soft-scrollbar flex min-h-0 min-w-0 flex-col overflow-y-auto px-5 pb-5 pt-4 sm:px-7"
                  key={scene}
                >
                  <JourneyProgress scene={scene} />
                  <div
                    className="mt-6 flex min-h-0 flex-1 flex-col animate-in fade-in-0 slide-in-from-bottom-1 duration-(--motion-duration-layout)"
                  >
                    {scene === "welcome" ? (
                      <WelcomeScene
                        onClose={close}
                        onContinue={() => setScene("provider")}
                      />
                    ) : null}
                    {scene === "provider" ? (
                      <ProviderScene
                        error={error}
                        loading={loading}
                        onAdvanced={close}
                        onBack={handleBack}
                        onContinue={() => {
                          if (selected) {
                            setError(null);
                            setScene("credentials");
                          }
                        }}
                        onSelect={selectPreset}
                        onShowAllChange={setShowAllProviders}
                        presets={presets}
                        providers={providers}
                        selectedPresetKey={selectedPresetKey}
                        showAll={showAllProviders}
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
                    {scene === "verify" ? <VerifyScene phase={verifyPhase} /> : null}
                    {scene === "ready" && result ? (
                      <ReadyScene
                        onStart={start}
                        result={result}
                      />
                    ) : null}
                  </div>
                </div>
              </div>
            </UiDialogBody>
          </UiDialogShell>
        </div>
      </UiDialogBackdrop>
    </UiDialogPortal>
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

function NexusPresence({ ready }: { ready: boolean }) {
  return (
    <aside
      aria-hidden="true"
      className="relative hidden min-h-0 overflow-hidden border-r border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--brand)_5%,var(--modal-dialog-body-background))] md:flex"
    >
      <div className="absolute left-5 top-5 flex items-center gap-2 text-2xs font-semibold tracking-[0.18em] text-(--text-muted)">
        <span className={ready ? "h-1.5 w-1.5 rounded-full bg-(--success)" : "h-1.5 w-1.5 rounded-full bg-(--brand-action)"} />
        NEXUS
      </div>
      <div className="absolute inset-x-0 bottom-0 h-[286px]">
        <img
          alt=""
          className={[
            "absolute bottom-0 left-1/2 h-[270px] max-w-none -translate-x-1/2 object-contain transition-[transform,filter] duration-500",
            ready ? "-rotate-2 scale-[1.03] drop-shadow-[0_14px_22px_rgba(54,63,91,0.12)]" : "scale-100",
          ].join(" ")}
          src="/nexus/nexus-mascot-front-wave.png"
        />
      </div>
    </aside>
  );
}

function WelcomeScene({
  onClose,
  onContinue,
}: {
  onClose: () => void;
  onContinue: () => void;
}) {
  const { t } = useI18n();
  return (
    <>
      <SceneMessage
        body={t("onboarding.provider_setup_welcome_description")}
        title={t("onboarding.provider_setup_welcome_title")}
      />
      <div className="mt-auto flex flex-wrap items-center justify-end gap-2 pt-5">
        <UiButton onClick={onClose} size="sm" variant="text">
          {t("onboarding.provider_setup_welcome_later")}
        </UiButton>
        <UiButton onClick={onContinue} size="sm" tone="primary" variant="solid">
          {t("onboarding.provider_setup_welcome_action")}
          <ChevronRight className="h-3.5 w-3.5" />
        </UiButton>
      </div>
    </>
  );
}

function ProviderScene({
  error,
  loading,
  onAdvanced,
  onBack,
  onContinue,
  onSelect,
  onShowAllChange,
  presets,
  providers,
  selectedPresetKey,
  showAll,
}: {
  error: string | null;
  loading: boolean;
  onAdvanced: () => void;
  onBack: () => void;
  onContinue: () => void;
  onSelect: (preset: ProviderSetupPreset) => void;
  onShowAllChange: (showAll: boolean) => void;
  presets: ProviderSetupPreset[];
  providers: ProviderConfigRecord[];
  selectedPresetKey: string;
  showAll: boolean;
}) {
  const { t } = useI18n();
  const visiblePresets = resolveVisiblePresets(presets, selectedPresetKey, showAll);
  return (
    <>
      <SceneMessage
        body={t("onboarding.provider_setup_provider_hint")}
        title={t("onboarding.provider_setup_provider_title")}
      />
      <div className="mt-5">
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
      <div className="mt-auto flex flex-wrap items-center gap-2 pt-4">
        <Link
          className="mr-auto inline-flex items-center gap-1 text-xs font-medium text-(--text-muted) hover:text-(--text-strong) hover:underline"
          onClick={onAdvanced}
          to={AppRouteBuilders.settings("providers")}
        >
          {t("onboarding.provider_setup_advanced")}
          <ExternalLink className="h-3 w-3" />
        </Link>
        <UiButton onClick={onBack} size="sm" variant="text">
          <ChevronLeft className="h-3.5 w-3.5" />
          {t("onboarding.provider_setup_back")}
        </UiButton>
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
  return (
    <>
      <SceneMessage
        body={existingProvider
          ? t("onboarding.provider_setup_credentials_saved_description")
          : t("onboarding.provider_setup_credentials_description")}
        title={t("onboarding.provider_setup_credentials_title", {
          provider: setup.preset.display_name,
        })}
      />

      <div className="mt-5 space-y-3">
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
          <UiField htmlFor="provider-setup-base-url" label={t("onboarding.provider_setup_base_url")}>
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

      <div className="mt-auto flex items-center justify-end gap-2 pt-4">
        <UiButton onClick={onBack} size="sm" variant="text">
          <ChevronLeft className="h-3.5 w-3.5" />
          {t("onboarding.provider_setup_back")}
        </UiButton>
        <UiButton onClick={onSubmit} size="sm" tone="primary" variant="solid">
          <PlugZap className="h-3.5 w-3.5" />
          {t("onboarding.provider_setup_submit")}
        </UiButton>
      </div>
    </>
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
      <div className="flex flex-1 items-center py-5">
        <div className="w-full">
          <p className="mb-2 text-xs font-medium text-(--text-strong)">
            {t("onboarding.provider_setup_features_title")}
          </p>
          <div className="grid grid-cols-2 border-y border-(--divider-subtle-color)">
            <FeatureItem
              className="border-b border-r border-(--divider-subtle-color) pr-3"
              icon={<MessageSquare className="h-4 w-4" />}
              title={t("onboarding.provider_setup_feature_agent_title")}
            />
            <FeatureItem
              className="border-b border-(--divider-subtle-color) pl-3"
              icon={<UsersRound className="h-4 w-4" />}
              title={t("onboarding.provider_setup_feature_room_title")}
            />
            <FeatureItem
              className="border-r border-(--divider-subtle-color) pr-3"
              icon={<Puzzle className="h-4 w-4" />}
              title={t("onboarding.provider_setup_feature_capability_title")}
            />
            <FeatureItem
              className="pl-3"
              icon={<Clock3 className="h-4 w-4" />}
              title={t("onboarding.provider_setup_feature_context_title")}
            />
          </div>
        </div>
      </div>
      <div className="flex justify-end">
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

function FeatureItem({
  className,
  icon,
  title,
}: {
  className: string;
  icon: ReactNode;
  title: string;
}) {
  return (
    <div className={`flex min-w-0 items-center gap-2.5 py-3 ${className}`}>
      <span className="shrink-0 text-(--brand-action)">{icon}</span>
      <span className="truncate text-xs font-medium text-(--text-strong)">{title}</span>
    </div>
  );
}

function resolveJourneyPhase(scene: SetupScene): JourneyPhase {
  if (scene === "ready") {
    return "start";
  }
  if (scene === "verify") {
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

function defaultModelID(provider: ProviderConfigRecord | null): string {
  return provider?.models.find((model) => model.is_default)?.model_id
    ?? provider?.models.find((model) => model.enabled)?.model_id
    ?? "";
}
