/**
 * INPUT: Dialog lifecycle, authenticated owner scope, Provider catalog and form edits.
 * OUTPUT: Owner-fenced Provider setup view model and explicit user commands.
 * POS: Provider setup React controller; durable side effects remain in provider-setup-workflow.
 */
"use client";

import { useEffect, useMemo, useState } from "react";

import { resolveAuthOwnerScope } from "@/app/auth/auth-owner-scope";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { getDefaultAgentRuntimeKind } from "@/config/runtime-options";
import {
  listProviderConfigsApi,
  listProviderPresetsApi,
} from "@/lib/api/settings/provider-api";
import { useAuth } from "@/shared/auth/auth-context";
import {
  assertAuthOwnerScopeGenerationCurrent,
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  isAuthOwnerScopeSupersededError,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useI18n } from "@/shared/i18n/i18n-context";
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
import {
  readProviderSetupJournal,
  removeProviderSetupJournal,
  type ProviderSetupJournal,
} from "./provider-setup-recovery";
import {
  persistImportedDefaultSelection,
  runProviderSetupWorkflow,
  type ProviderConnectionDraft,
  type ProviderSetupFailure,
  type ProviderSetupFailureScene,
  type ProviderSetupResult,
} from "./provider-setup-workflow";

const FEATURED_PROVIDER_COUNT = 4;

export type ProviderSetupScene =
  | "credentials"
  | "custom"
  | "provider"
  | "ready"
  | "verify";

interface UseProviderSetupControllerInput {
  isOpen: boolean;
  onClose: () => void;
  onStart?: () => void;
}

export interface ProviderSetupController {
  busy: boolean;
  canImportFromCCSwitch: boolean;
  ccSwitchOpen: boolean;
  close: () => void;
  credentials: {
    apiKey: string;
    apiKeyRequired: boolean;
    baseURL: string;
    existingProvider: ProviderConfigRecord | null;
    modelID: string;
    modelRequired: boolean;
    onApiKeyChange: (value: string) => void;
    onBaseURLChange: (value: string) => void;
    onModelIDChange: (value: string) => void;
    onSubmit: () => void;
    setup: ProviderSetupPreset | null;
  };
  custom: {
    apiFormat: ProviderApiFormat | "";
    apiKey: string;
    baseURL: string;
    existingProvider: ProviderConfigRecord | null;
    formats: ProviderSetupPreset[];
    modelID: string;
    onApiFormatChange: (value: string) => void;
    onApiKeyChange: (value: string) => void;
    onBaseURLChange: (value: string) => void;
    onModelIDChange: (value: string) => void;
    onNameChange: (value: string) => void;
    onSubmit: () => void;
    providerName: string;
  };
  failure: ProviderSetupFailure | null;
  handleCCSwitchSynced: (result: CCSwitchSyncResult) => Promise<void>;
  loading: boolean;
  onBack: () => void;
  onContinue: () => void;
  onCustom: () => void;
  onImportCCSwitch: () => void;
  onRetryLoad: () => void;
  onSelectPreset: (preset: ProviderSetupPreset) => void;
  onShowAllChange: (showAll: boolean) => void;
  onStart: () => void;
  onStartNewIntent: () => void;
  providerCatalog: {
    presets: ProviderSetupPreset[];
    providers: ProviderConfigRecord[];
    selectedPresetKey: string;
    showAll: boolean;
    supportsCustom: boolean;
  };
  recoveryLocked: boolean;
  result: ProviderSetupResult | null;
  scene: ProviderSetupScene;
  setCCSwitchOpen: (open: boolean) => void;
  submitLabel: string;
  verifyPhase: 0 | 1 | 2;
}

export function useProviderSetupController({
  isOpen,
  onClose,
  onStart,
}: UseProviderSetupControllerInput): ProviderSetupController {
  const { t } = useI18n();
  const { status: authStatus } = useAuth();
  const ownerScope = authStatus ? resolveAuthOwnerScope(authStatus) : null;
  const runtimeKind = getDefaultAgentRuntimeKind();
  const canImportFromCCSwitch = isDesktopRuntime();
  const [scene, setScene] = useState<ProviderSetupScene>("provider");
  const [ccSwitchOpen, setCCSwitchOpen] = useState(false);
  const [presets, setPresets] = useState<ProviderSetupPreset[]>([]);
  const [customSetups, setCustomSetups] = useState<ProviderSetupPreset[]>([]);
  const [providers, setProviders] = useState<ProviderConfigRecord[]>([]);
  const [selectedPresetKey, setSelectedPresetKey] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseURL, setBaseURL] = useState("");
  const [modelID, setModelID] = useState("");
  const [customProviderKey, setCustomProviderKey] = useState("");
  const [customProviderName, setCustomProviderName] = useState("");
  const [customApiFormat, setCustomApiFormat] = useState<ProviderApiFormat | "">("");
  const [customApiKey, setCustomApiKey] = useState("");
  const [customBaseURL, setCustomBaseURL] = useState("");
  const [customModelID, setCustomModelID] = useState("");
  const [showAllProviders, setShowAllProviders] = useState(false);
  const [verifyPhase, setVerifyPhase] = useState<0 | 1 | 2>(0);
  const [loadGeneration, setLoadGeneration] = useState(0);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<ProviderSetupFailure | null>(null);
  const [result, setResult] = useState<ProviderSetupResult | null>(null);
  const [journal, setJournal] = useState<ProviderSetupJournal | null>(null);

  useEffect(() => subscribeAuthOwnerScopeGeneration(() => {
    // Journal is owner-keyed and contains no credential. Keep the old owner's
    // recovery fence durable, but remove it from the mounted React tree.
    setJournal(null);
    setFailure(null);
  }), []);

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
    () => customSetups.find((item) => item.format.api_format === customApiFormat) ?? null,
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
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const recovered = ownerScope
      ? readProviderSetupJournal(ownerScope).journal
      : null;
    resetDialogForLoad(recovered);
    void Promise.all([
      listProviderPresetsApi(),
      listProviderConfigsApi(),
    ]).then(([nextPresets, nextProviders]) => {
      if (cancelled || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
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
      setCustomApiFormat(
        recovered?.apiFormat ?? selectInitialCustomAPIFormat(nextCustomSetups),
      );
      if (recovered) {
        restoreJournal(recovered, nextProviders);
        if (recovered.outcome === "unknown") {
          setFailure(failureForUnknownStage(recovered.stage));
        }
        return;
      }
      selectInitialPreset(setupPresets, nextProviders);
    }).catch(() => {
      if (!cancelled && isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setFailure({
          kind: "read",
          problemKey: "onboarding.provider_setup_load_failed",
        });
      }
    }).finally(() => {
      if (!cancelled && isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [isOpen, loadGeneration, ownerScope, runtimeKind]);

  const resetDialogForLoad = (recovered: ProviderSetupJournal | null) => {
    setScene("provider");
    setLoading(true);
    setBusy(false);
    setCCSwitchOpen(false);
    setPresets([]);
    setCustomSetups([]);
    setProviders([]);
    setSelectedPresetKey("");
    setFailure(null);
    setResult(null);
    setJournal(recovered);
    setApiKey("");
    setBaseURL("");
    setModelID("");
    setCustomProviderKey(recovered?.providerKey ?? createCustomProviderKey());
    setCustomProviderName("");
    setCustomApiKey("");
    setCustomBaseURL("");
    setCustomModelID("");
    setShowAllProviders(false);
    setVerifyPhase(0);
  };

  const selectInitialPreset = (
    setupPresets: ProviderSetupPreset[],
    nextProviders: ProviderConfigRecord[],
  ) => {
    const first = selectInitialProviderSetupPreset(setupPresets, nextProviders);
    if (!first) {
      setSelectedPresetKey("");
      return;
    }
    const firstIndex = setupPresets.findIndex(
      (item) => item.preset.preset_key === first.preset.preset_key,
    );
    const configured = findManageablePresetProvider(
      nextProviders,
      first.preset.preset_key,
    );
    setSelectedPresetKey(first.preset.preset_key);
    setShowAllProviders(firstIndex >= FEATURED_PROVIDER_COUNT);
    setBaseURL(
      configured?.base_url && first.preset.endpoint_mode !== "fixed"
        ? configured.base_url
        : first.format.base_url,
    );
    setModelID(defaultModelID(configured));
  };

  const restoreJournal = (
    recovered: ProviderSetupJournal,
    nextProviders: ProviderConfigRecord[],
  ) => {
    const record = nextProviders.find((item) => (
      item.can_manage && item.provider === recovered.providerKey
    )) ?? null;
    const recoveredModel = recovered.model || defaultModelID(record);
    if (recovered.presetKey === "custom") {
      setCustomProviderName(record?.display_name || recovered.providerDisplayName);
      setCustomBaseURL(record?.base_url || "");
      setCustomModelID(recoveredModel);
      setScene("custom");
      return;
    }
    setSelectedPresetKey(recovered.presetKey);
    setBaseURL(record?.base_url || "");
    setModelID(recoveredModel);
    setScene("credentials");
  };

  const abandonJournal = () => {
    if (ownerScope) {
      removeProviderSetupJournal(ownerScope);
    }
    setJournal(null);
  };

  const recoveryLocked = journal?.outcome === "unknown";

  const markConfigurationEdited = () => {
    if (recoveryLocked) {
      return;
    }
    abandonJournal();
    setFailure(null);
  };

  const selectPreset = (preset: ProviderSetupPreset) => {
    if (recoveryLocked) {
      return;
    }
    abandonJournal();
    setSelectedPresetKey(preset.preset.preset_key);
    setFailure(null);
    setResult(null);
    setApiKey("");
    const configured = findManageablePresetProvider(
      providers,
      preset.preset.preset_key,
    );
    setBaseURL(
      configured?.base_url && preset.preset.endpoint_mode !== "fixed"
        ? configured.base_url
        : preset.format.base_url,
    );
    setModelID(defaultModelID(configured));
  };

  const submitConnection = (
    draft: ProviderConnectionDraft,
    failureScene: ProviderSetupFailureScene,
  ) => {
    if (busy) {
      return;
    }
    const normalized: ProviderConnectionDraft = {
      ...draft,
      apiKey: draft.apiKey.trim(),
      baseURL: draft.baseURL.trim(),
      displayName: draft.displayName.trim(),
      modelID: draft.modelID.trim(),
    };
    const resumesWithoutDraft = journal?.providerKey === normalized.providerKey
      && (journal.stage !== "persist" || journal.outcome === "unknown");
    if (
      !resumesWithoutDraft
      && !normalized.existingProvider?.auth_token_masked?.trim()
      && !normalized.apiKey
    ) {
      setFailure(validationFailure("onboarding.provider_setup_api_key_required"));
      return;
    }
    if (!resumesWithoutDraft && !normalized.baseURL) {
      setFailure(validationFailure("onboarding.provider_setup_base_url_required"));
      return;
    }
    if (!resumesWithoutDraft && normalized.modelRequired && !normalized.modelID) {
      setFailure(validationFailure("onboarding.provider_setup_model_required"));
      return;
    }

    setBusy(true);
    setScene("verify");
    setFailure(null);
    setResult(null);
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    void runProviderSetupWorkflow({
      callbacks: {
        onJournal: setJournal,
        onPhase: setVerifyPhase,
        onProviders: setProviders,
      },
      currentJournal: journal,
      draft: normalized,
      ownerGeneration,
      ownerScope,
    }).then((workflowResult) => {
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      if (workflowResult.kind === "blocked") {
        setFailure(workflowResult.failure);
        setScene(failureScene);
        return;
      }
      setFailure(null);
      setResult(workflowResult.result);
      setScene("ready");
    }).catch((error: unknown) => {
      if (!isAuthOwnerScopeSupersededError(error)) {
        setFailure(failureForUnknownStage(journal?.stage ?? "persist"));
        setScene(failureScene);
      }
    }).finally(() => {
      if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setBusy(false);
      }
    });
  };

  const handleSubmit = () => {
    if (!selected) {
      return;
    }
    submitConnection({
      apiFormat: selected.format.api_format,
      apiKey,
      baseURL: usesBuiltinEndpoint ? selected.format.base_url : baseURL,
      displayName: selected.preset.display_name,
      existingProvider,
      modelID,
      modelRequired,
      modelsPath: selected.format.models_path,
      presetKey: selected.preset.preset_key,
      providerKey: existingProvider?.provider ?? selected.preset.preset_key,
    }, "credentials");
  };

  const handleCustomSubmit = () => {
    const displayName = customProviderName.trim();
    if (!displayName) {
      setFailure(validationFailure("onboarding.provider_setup_custom_name_required"));
      return;
    }
    if (!customSetup || !customApiFormat) {
      setFailure(validationFailure("onboarding.provider_setup_custom_format_required"));
      return;
    }
    submitConnection({
      apiFormat: customApiFormat,
      apiKey: customApiKey,
      baseURL: customBaseURL,
      displayName,
      existingProvider: customExistingProvider,
      modelID: customModelID,
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
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    await persistImportedDefaultSelection(selection);
    assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
    setResult({
      model: selection.model_display_name || selection.model,
      provider: selection.provider_display_name || selection.provider,
    });
    setScene("ready");
  };

  const onBack = () => {
    if (recoveryLocked) {
      return;
    }
    abandonJournal();
    setFailure(null);
    setScene("provider");
  };

  const onStartNewIntent = () => {
    if (!window.confirm(t("onboarding.provider_setup_new_intent_confirm"))) {
      return;
    }
    abandonJournal();
    setCustomProviderKey(createCustomProviderKey());
    setFailure(null);
    setScene("provider");
  };

  const submitLabel = recoveryLocked
    ? journal?.stage === "test"
      ? t("onboarding.provider_setup_reconcile_test_action")
      : journal?.stage === "default"
        ? t("onboarding.provider_setup_reconcile_default_action")
        : t("onboarding.provider_setup_reconcile_action")
    : journal?.stage === "default"
      ? t("onboarding.provider_setup_retry_default")
      : journal?.stage === "test"
        ? t("onboarding.provider_setup_retry_test")
        : t("onboarding.provider_setup_submit");

  return {
    busy,
    canImportFromCCSwitch,
    ccSwitchOpen,
    close: () => {
      if (!busy) {
        onClose();
      }
    },
    credentials: {
      apiKey,
      apiKeyRequired,
      baseURL,
      existingProvider,
      modelID,
      modelRequired,
      onApiKeyChange: (value) => {
        setApiKey(value);
        markConfigurationEdited();
      },
      onBaseURLChange: (value) => {
        setBaseURL(value);
        markConfigurationEdited();
      },
      onModelIDChange: (value) => {
        setModelID(value);
        markConfigurationEdited();
      },
      onSubmit: handleSubmit,
      setup: selected,
    },
    custom: {
      apiFormat: customApiFormat,
      apiKey: customApiKey,
      baseURL: customBaseURL,
      existingProvider: customExistingProvider,
      formats: customSetups,
      modelID: customModelID,
      onApiFormatChange: (value) => {
        const next = customSetups.find((item) => item.format.api_format === value);
        if (next) {
          setCustomApiFormat(next.format.api_format);
          markConfigurationEdited();
        }
      },
      onApiKeyChange: (value) => {
        setCustomApiKey(value);
        markConfigurationEdited();
      },
      onBaseURLChange: (value) => {
        setCustomBaseURL(value);
        markConfigurationEdited();
      },
      onModelIDChange: (value) => {
        setCustomModelID(value);
        markConfigurationEdited();
      },
      onNameChange: (value) => {
        setCustomProviderName(value);
        markConfigurationEdited();
      },
      onSubmit: handleCustomSubmit,
      providerName: customProviderName,
    },
    failure,
    handleCCSwitchSynced,
    loading,
    onBack,
    onContinue: () => {
      if (selected) {
        setFailure(null);
        setScene("credentials");
      }
    },
    onCustom: () => {
      if (recoveryLocked) {
        return;
      }
      abandonJournal();
      setCustomProviderKey(createCustomProviderKey());
      setFailure(null);
      setScene("custom");
    },
    onImportCCSwitch: () => setCCSwitchOpen(true),
    onRetryLoad: () => setLoadGeneration((value) => value + 1),
    onSelectPreset: selectPreset,
    onShowAllChange: setShowAllProviders,
    onStart: () => {
      if (!busy) {
        onClose();
        onStart?.();
      }
    },
    onStartNewIntent,
    providerCatalog: {
      presets,
      providers,
      selectedPresetKey,
      showAll: showAllProviders,
      supportsCustom: customSetups.length > 0,
    },
    recoveryLocked,
    result,
    scene,
    setCCSwitchOpen,
    submitLabel,
    verifyPhase,
  };
}

function failureForUnknownStage(
  stage: ProviderSetupJournal["stage"],
): ProviderSetupFailure {
  if (stage === "persist") {
    return {
      kind: "persist_unknown",
      problemKey: "onboarding.provider_setup_persist_unknown_problem",
    };
  }
  if (stage === "test") {
    return {
      kind: "test_unknown",
      problemKey: "onboarding.provider_setup_test_unknown_problem",
    };
  }
  return {
    kind: "default_unknown",
    problemKey: "onboarding.provider_setup_default_unknown_problem",
  };
}

function validationFailure(
  problemKey: ProviderSetupFailure["problemKey"],
): ProviderSetupFailure {
  return { kind: "validation", problemKey };
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

function createCustomProviderKey(): string {
  const timestamp = Date.now().toString(36);
  const random = Math.random().toString(36).slice(2, 8);
  return `custom-${timestamp}-${random}`;
}
