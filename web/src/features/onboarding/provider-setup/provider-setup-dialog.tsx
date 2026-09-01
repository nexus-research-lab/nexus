/**
 * INPUT: Owner 作用域、Provider 精确 key/version、连接测试与默认偏好命令。
 * OUTPUT: 可恢复的保存、测试、默认选择三阶段单栏连接向导及精简失败提示。
 * POS: 首次 Provider 配置编排边界；写前 journal 不保存密钥、Base URL、请求正文或 HTTP 身份。
 */
"use client";

import {
  type Dispatch,
  type SetStateAction,
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
  ExternalLink,
  Loader2,
  Settings2,
} from "lucide-react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { getDefaultAgentRuntimeKind, setUserPreferences } from "@/config/runtime-options";
import { resolveAuthOwnerScope } from "@/app/auth/auth-owner-scope";
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
import { projectMutationFailure } from "@/lib/error-message";
import { useAuth } from "@/shared/auth/auth-context";
import {
  assertAuthOwnerScopeGenerationCurrent,
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  isAuthOwnerScopeSupersededError,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
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
  UpdateProviderConfigPayload,
} from "@/types/capability/provider";
import type { UserPreferences } from "@/types/settings/preferences";

import {
  findManageablePresetProvider,
  listCustomProviderSetupFormats,
  listProviderSetupPresets,
  providerSetupModelIsRequired,
  selectInitialProviderSetupPreset,
  type ProviderSetupPreset,
} from "./provider-setup-model";
import { ProviderSetupFailureView } from "./provider-setup-failure";
import {
  fingerprintProviderSetup,
  preferencesUseProviderSelection,
  readProviderSetupJournal,
  reconcileProviderPersist,
  reconcileProviderTest,
  removeProviderSetupJournal,
  writeProviderSetupJournal,
  type ProviderSetupJournal,
} from "./provider-setup-recovery";

interface ProviderSetupDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onStart?: () => void;
}

type SetupScene = "provider" | "credentials" | "custom" | "verify" | "ready";
type JourneyPhase = "connect" | "discover" | "start";
type SetupFailureKind =
  | "default_not_applied"
  | "default_unknown"
  | "journal_after_save"
  | "journal_before_submit"
  | "persist_not_applied"
  | "persist_unknown"
  | "read"
  | "test_failed"
  | "test_not_applied"
  | "test_unknown"
  | "validation";

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
  const { status: authStatus } = useAuth();
  const ownerScope = authStatus ? resolveAuthOwnerScope(authStatus) : null;
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
  const [errorKind, setErrorKind] = useState<SetupFailureKind>("read");
  const [result, setResult] = useState<SetupResult | null>(null);
  const [journal, setJournal] = useState<ProviderSetupJournal | null>(null);

  useEffect(() => {
    if (!ownerScope) {
      return undefined;
    }
    return subscribeAuthOwnerScopeGeneration(() => {
      removeProviderSetupJournal(ownerScope);
      setJournal(null);
    });
  }, [ownerScope]);

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
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const journalRead = ownerScope
      ? readProviderSetupJournal(ownerScope)
      : { journal: null, status: "unavailable" as const };
    const recoveredJournal = journalRead.journal;
    setScene("provider");
    setLoading(true);
    setBusy(false);
    setCCSwitchOpen(false);
    setError(null);
    setResult(null);
    setJournal(recoveredJournal);
    setApiKey("");
    setBaseUrl("");
    setModelId("");
    setCustomProviderKey(recoveredJournal?.providerKey ?? createCustomProviderKey());
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
        recoveredJournal?.apiFormat
        ?? selectInitialCustomAPIFormat(nextCustomSetups),
      );
      if (recoveredJournal) {
        restoreProviderSetupJournal({
          journal: recoveredJournal,
          providers: nextProviders,
          setBaseUrl,
          setCustomBaseUrl,
          setCustomModelId,
          setCustomProviderName,
          setModelId,
          setScene,
          setSelectedPresetKey,
        });
        if (recoveredJournal.outcome === "unknown") {
          setErrorKind(failureKindForUnknownStage(recoveredJournal.stage));
          setError(t(failureMessageKeyForUnknownStage(recoveredJournal.stage)));
        }
        return;
      }
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
      if (!cancelled && isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        void loadError;
        setErrorKind("read");
        setError(t("onboarding.provider_setup_load_failed"));
      }
    }).finally(() => {
      if (!cancelled && isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [isOpen, ownerScope, runtimeKind, t]);

  if (!isOpen) {
    return null;
  }

  const selectPreset = (preset: ProviderSetupPreset) => {
    abandonProviderSetupJournal(ownerScope, setJournal);
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
    if (journal?.stage === "persist" && journal.outcome === "unknown") {
      return;
    }
    abandonProviderSetupJournal(ownerScope, setJournal);
    setError(null);
    setScene("provider");
  };

  const markConfigurationEdited = () => {
    if (journal?.stage === "persist" && journal.outcome === "unknown") {
      return;
    }
    abandonProviderSetupJournal(ownerScope, setJournal);
    setError(null);
  };

  const startNewIntent = () => {
    if (!window.confirm(t("onboarding.provider_setup_new_intent_confirm"))) {
      return;
    }
    abandonProviderSetupJournal(ownerScope, setJournal);
    setCustomProviderKey(createCustomProviderKey());
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
    const resumesWithoutDraft = journal?.providerKey === draft.providerKey
      && (journal.stage !== "persist" || journal.outcome === "unknown");
    if (!resumesWithoutDraft && requiresAPIKey && !normalizedApiKey) {
      setErrorKind("validation");
      setError(t("onboarding.provider_setup_api_key_required"));
      return;
    }
    if (!resumesWithoutDraft && !normalizedBaseURL) {
      setErrorKind("validation");
      setError(t("onboarding.provider_setup_base_url_required"));
      return;
    }
    if (!resumesWithoutDraft && draft.modelRequired && !normalizedModelID) {
      setErrorKind("validation");
      setError(t("onboarding.provider_setup_model_required"));
      return;
    }

    const normalizedDraft: ProviderConnectionDraft = {
      ...draft,
      apiKey: normalizedApiKey,
      baseURL: normalizedBaseURL,
      displayName: draft.displayName.trim(),
      modelID: normalizedModelID,
    };
    setBusy(true);
    setScene("verify");
    setError(null);
    setResult(null);
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    void runProviderSetup(normalizedDraft, failureScene, ownerGeneration)
      .catch((setupError: unknown) => {
        if (!isAuthOwnerScopeSupersededError(setupError)) {
          setSetupFailure(
            "persist_unknown",
            t("onboarding.provider_setup_persist_unknown_problem"),
            failureScene,
          );
        }
      })
      .finally(() => {
        if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          setBusy(false);
        }
      });
  };

  const runProviderSetup = async (
    draft: ProviderConnectionDraft,
    failureScene: "credentials" | "custom",
    ownerGeneration: number,
  ) => {
    assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
    let activeJournal = journal;
    if (activeJournal && activeJournal.providerKey !== draft.providerKey) {
      setSetupFailure(
        "persist_unknown",
        t("onboarding.provider_setup_persist_unknown_problem"),
        failureScene,
      );
      return;
    }
    if (!activeJournal) {
      try {
        activeJournal = await createProviderSetupJournal(ownerScope, draft);
      } catch (setupError) {
        if (isAuthOwnerScopeSupersededError(setupError)) {
          throw setupError;
        }
        setSetupFailure(
          "journal_before_submit",
          t("onboarding.provider_setup_journal_before_problem"),
          failureScene,
        );
        return;
      }
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      if (!activeJournal || !storeJournalBeforeEffect(activeJournal, setJournal)) {
        setSetupFailure(
          "journal_before_submit",
          t("onboarding.provider_setup_journal_before_problem"),
          failureScene,
        );
        return;
      }
    }

    if (activeJournal.stage === "persist") {
      setVerifyPhase(0);
      const persisted = await ensureProviderPersisted(
        activeJournal,
        draft,
        failureScene,
        ownerGeneration,
      );
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      if (!persisted) {
        return;
      }
      activeJournal = persisted;
    }
    if (activeJournal.stage === "test") {
      setVerifyPhase(1);
      const tested = await ensureProviderTested(
        activeJournal,
        failureScene,
        ownerGeneration,
      );
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      if (!tested) {
        return;
      }
      activeJournal = tested;
    }
    if (activeJournal.stage === "default") {
      setVerifyPhase(2);
      await ensureDefaultSelection(activeJournal, failureScene, ownerGeneration);
    }
  };

  const ensureProviderPersisted = async (
    activeJournal: ProviderSetupJournal,
    draft: ProviderConnectionDraft,
    failureScene: "credentials" | "custom",
    ownerGeneration: number,
  ): Promise<ProviderSetupJournal | null> => {
    if (activeJournal.outcome === "unknown") {
      return reconcilePersistStage(activeJournal, failureScene, ownerGeneration);
    }
    const inFlight = { ...activeJournal, outcome: "unknown" as const };
    if (!storeJournalBeforeEffect(inFlight, setJournal)) {
      setSetupFailure(
        "journal_before_submit",
        t("onboarding.provider_setup_journal_before_problem"),
        failureScene,
      );
      return null;
    }
    let record: ProviderConfigRecord;
    try {
      const payload = providerSetupPayload(draft);
      record = draft.existingProvider
        ? await updateProviderConfigApi(draft.existingProvider.provider, payload, {
          expectedVersion: activeJournal.baselineConfigurationVersion ?? undefined,
        })
        : await createProviderConfigApi({
          ...payload,
          auth_token: draft.apiKey,
          provider: draft.providerKey,
          provider_kind: "llm",
          visibility: "private",
        });
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      const failure = projectMutationFailure(
        setupError,
        t("onboarding.provider_setup_persist_unknown_problem"),
      );
      if (failure.effect === "not_applied") {
        const ready = { ...inFlight, outcome: "ready" as const };
        writeProviderSetupJournal(ready);
        setJournal(ready);
        setSetupFailure(
          "persist_not_applied",
          t("onboarding.provider_setup_persist_not_applied_problem"),
          failureScene,
        );
        return null;
      }
      if (failure.effect === "committed") {
        return reconcilePersistStage(
          inFlight,
          failureScene,
          ownerGeneration,
          true,
        );
      }
      return reconcilePersistStage(inFlight, failureScene, ownerGeneration);
    }
    return advanceJournalAfterPersist(inFlight, record, failureScene);
  };

  const reconcilePersistStage = async (
    activeJournal: ProviderSetupJournal,
    failureScene: "credentials" | "custom",
    ownerGeneration: number,
    commitConfirmed = false,
  ): Promise<ProviderSetupJournal | null> => {
    try {
      const latestProviders = await listProviderConfigsApi();
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      setProviders(latestProviders);
      const outcome = await reconcileProviderPersist(
        activeJournal,
        latestProviders,
        commitConfirmed,
      );
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      if (outcome.kind === "applied") {
        return advanceJournalAfterPersist(activeJournal, outcome.record, failureScene);
      }
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      // The exact read is the only recovery evidence; keep the write lock if it is unavailable.
    }
    setJournal(activeJournal);
    setSetupFailure(
      "persist_unknown",
      t("onboarding.provider_setup_persist_unknown_problem"),
      failureScene,
    );
    return null;
  };

  const advanceJournalAfterPersist = (
    activeJournal: ProviderSetupJournal,
    record: ProviderConfigRecord,
    failureScene: "credentials" | "custom",
  ): ProviderSetupJournal | null => {
    if (!validConfigurationVersion(record.configuration_version)) {
      setSetupFailure(
        "journal_after_save",
        t("onboarding.provider_setup_journal_after_problem"),
        failureScene,
      );
      return null;
    }
    const next: ProviderSetupJournal = {
      ...activeJournal,
      configurationVersion: record.configuration_version,
      outcome: "ready",
      providerDisplayName: record.display_name || activeJournal.providerDisplayName,
      stage: "test",
      testBaselineAt: record.last_test_at?.trim() || null,
    };
    if (!storeJournalBeforeEffect(next, setJournal)) {
      setSetupFailure(
        "journal_after_save",
        t("onboarding.provider_setup_journal_after_problem"),
        failureScene,
      );
      return null;
    }
    return next;
  };

  const ensureProviderTested = async (
    activeJournal: ProviderSetupJournal,
    failureScene: "credentials" | "custom",
    ownerGeneration: number,
  ): Promise<ProviderSetupJournal | null> => {
    if (activeJournal.outcome === "unknown") {
      return reconcileTestStage(activeJournal, failureScene, ownerGeneration);
    }
    const inFlight = { ...activeJournal, outcome: "unknown" as const };
    if (!storeJournalBeforeEffect(inFlight, setJournal)) {
      setSetupFailure(
        "journal_after_save",
        t("onboarding.provider_setup_journal_after_problem"),
        failureScene,
      );
      return null;
    }
    try {
      const options = { expectedVersion: activeJournal.configurationVersion ?? undefined };
      const testResult = activeJournal.model
        ? await testProviderModelApi(activeJournal.providerKey, activeJournal.model, options)
        : await testProviderConfigApi(activeJournal.providerKey, options);
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      const testedJournal = {
        ...inFlight,
        configurationVersion: testResult.configuration_version,
        model: testResult.model?.trim() || activeJournal.model,
        testBaselineAt: testResult.tested_at?.trim() || activeJournal.testBaselineAt,
      };
      if (!testResult.success) {
        const ready = { ...testedJournal, outcome: "ready" as const };
        writeProviderSetupJournal(ready);
        setJournal(ready);
        await refreshProvidersWithoutReplacingFailure(ownerGeneration);
        setSetupFailure(
          "test_failed",
          t("onboarding.provider_setup_test_failed_problem"),
          failureScene,
        );
        return null;
      }
      return advanceJournalAfterTest(testedJournal, failureScene);
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      const failure = projectMutationFailure(
        setupError,
        t("onboarding.provider_setup_test_unknown_problem"),
      );
      if (failure.effect === "not_applied") {
        await refreshProvidersWithoutReplacingFailure(ownerGeneration);
        const ready = {
          ...inFlight,
          outcome: "ready" as const,
        };
        writeProviderSetupJournal(ready);
        setJournal(ready);
        setSetupFailure(
          "test_not_applied",
          t("onboarding.provider_setup_test_not_applied_problem"),
          failureScene,
        );
        return null;
      }
      return reconcileTestStage(inFlight, failureScene, ownerGeneration);
    }
  };

  const reconcileTestStage = async (
    activeJournal: ProviderSetupJournal,
    failureScene: "credentials" | "custom",
    ownerGeneration: number,
  ): Promise<ProviderSetupJournal | null> => {
    try {
      const latestProviders = await listProviderConfigsApi();
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      setProviders(latestProviders);
      const outcome = reconcileProviderTest(activeJournal, latestProviders);
      if (outcome.kind === "passed") {
        return advanceJournalAfterTest({
          ...activeJournal,
          configurationVersion: outcome.record.configuration_version,
          testBaselineAt: outcome.record.last_test_at?.trim() || activeJournal.testBaselineAt,
        }, failureScene);
      }
      if (outcome.kind === "failed") {
        const ready = {
          ...activeJournal,
          configurationVersion: outcome.record.configuration_version,
          outcome: "ready" as const,
          testBaselineAt: outcome.record.last_test_at?.trim() || activeJournal.testBaselineAt,
        };
        writeProviderSetupJournal(ready);
        setJournal(ready);
        setSetupFailure(
          "test_failed",
          t("onboarding.provider_setup_test_failed_problem"),
          failureScene,
        );
        return null;
      }
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      // Keep the exact test stage locked until a later Provider read can prove it.
    }
    setJournal(activeJournal);
    setSetupFailure(
      "test_unknown",
      t("onboarding.provider_setup_test_unknown_problem"),
      failureScene,
    );
    return null;
  };

  const advanceJournalAfterTest = (
    activeJournal: ProviderSetupJournal,
    failureScene: "credentials" | "custom",
  ): ProviderSetupJournal | null => {
    if (!activeJournal.model || !validConfigurationVersion(activeJournal.configurationVersion)) {
      setSetupFailure(
        "test_unknown",
        t("onboarding.provider_setup_test_unknown_problem"),
        failureScene,
      );
      return null;
    }
    const next: ProviderSetupJournal = {
      ...activeJournal,
      outcome: "ready",
      stage: "default",
    };
    if (!storeJournalBeforeEffect(next, setJournal)) {
      setSetupFailure(
        "journal_after_save",
        t("onboarding.provider_setup_journal_after_problem"),
        failureScene,
      );
      return null;
    }
    return next;
  };

  const ensureDefaultSelection = async (
    activeJournal: ProviderSetupJournal,
    failureScene: "credentials" | "custom",
    ownerGeneration: number,
  ) => {
    if (activeJournal.outcome === "unknown") {
      await reconcileDefaultStage(activeJournal, failureScene, ownerGeneration);
      return;
    }
    let currentPreferences;
    try {
      currentPreferences = await getUserPreferencesApi();
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      setSetupFailure(
        "default_not_applied",
        t("onboarding.provider_setup_default_not_applied_problem"),
        failureScene,
      );
      return;
    }
    if (!validConfigurationVersion(currentPreferences.version)) {
      setSetupFailure(
        "default_not_applied",
        t("onboarding.provider_setup_default_not_applied_problem"),
        failureScene,
      );
      return;
    }
    const inFlight = { ...activeJournal, outcome: "unknown" as const };
    if (!storeJournalBeforeEffect(inFlight, setJournal)) {
      setSetupFailure(
        "journal_after_save",
        t("onboarding.provider_setup_journal_after_problem"),
        failureScene,
      );
      return;
    }
    try {
      const savedPreferences = await updateDefaultModelSelections(
        currentPreferences,
        activeJournal.providerKey,
        activeJournal.model,
      );
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      finishProviderSetup(inFlight, savedPreferences);
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      const failure = projectMutationFailure(
        setupError,
        t("onboarding.provider_setup_default_unknown_problem"),
      );
      if (failure.effect === "not_applied") {
        const ready = { ...inFlight, outcome: "ready" as const };
        writeProviderSetupJournal(ready);
        setJournal(ready);
        setSetupFailure(
          "default_not_applied",
          t("onboarding.provider_setup_default_not_applied_problem"),
          failureScene,
        );
        return;
      }
      await reconcileDefaultStage(inFlight, failureScene, ownerGeneration);
    }
  };

  const reconcileDefaultStage = async (
    activeJournal: ProviderSetupJournal,
    failureScene: "credentials" | "custom",
    ownerGeneration: number,
  ) => {
    try {
      const currentPreferences = await getUserPreferencesApi();
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      if (preferencesUseProviderSelection(
        currentPreferences,
        activeJournal.providerKey,
        activeJournal.model,
      )) {
        finishProviderSetup(activeJournal, currentPreferences);
        return;
      }
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      // A failed exact preference read cannot release an unknown default-selection stage.
    }
    setJournal(activeJournal);
    setSetupFailure(
      "default_unknown",
      t("onboarding.provider_setup_default_unknown_problem"),
      failureScene,
    );
  };

  const finishProviderSetup = (
    activeJournal: ProviderSetupJournal,
    savedPreferences: Awaited<ReturnType<typeof getUserPreferencesApi>>,
  ) => {
    setUserPreferences(savedPreferences);
    invalidateProviderAvailability();
    const complete = { ...activeJournal, outcome: "ready" as const, stage: "complete" as const };
    writeProviderSetupJournal(complete);
    removeProviderSetupJournal(activeJournal.ownerScope);
    setJournal(null);
    setError(null);
    setResult({
      model: activeJournal.model,
      provider: activeJournal.providerDisplayName,
    });
    setScene("ready");
  };

  const refreshProvidersWithoutReplacingFailure = async (ownerGeneration: number) => {
    try {
      const latest = await listProviderConfigsApi();
      assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
      setProviders(latest);
      return latest;
    } catch (setupError) {
      if (isAuthOwnerScopeSupersededError(setupError)) {
        throw setupError;
      }
      return providers;
    }
  };

  const setSetupFailure = (
    kind: SetupFailureKind,
    message: string,
    failureScene: "credentials" | "custom",
  ) => {
    setErrorKind(kind);
    setError(message);
    setScene(failureScene);
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
      setErrorKind("validation");
      setError(t("onboarding.provider_setup_custom_name_required"));
      return;
    }
    if (!customSetup || !customApiFormat) {
      setErrorKind("validation");
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

  const persistenceLocked = journal?.stage === "persist"
    && journal.outcome === "unknown";
  const submitLabel = journal?.stage === "default"
    ? t("onboarding.provider_setup_retry_default")
    : journal?.stage === "test"
      ? t("onboarding.provider_setup_retry_test")
      : persistenceLocked
        ? t("onboarding.provider_setup_reconcile_action")
        : t("onboarding.provider_setup_submit");

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
                        errorKind={errorKind}
                        loading={loading}
                        onContinue={() => {
                          if (selected) {
                            setError(null);
                            setScene("credentials");
                          }
                        }}
                        onCustom={() => {
                          abandonProviderSetupJournal(ownerScope, setJournal);
                          setCustomProviderKey(createCustomProviderKey());
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
                        errorKind={errorKind}
                        existingProvider={existingProvider}
                        modelId={modelId}
                        modelRequired={modelRequired}
                        locked={persistenceLocked}
                        onApiKeyChange={(value) => {
                          setApiKey(value);
                          markConfigurationEdited();
                        }}
                        onBack={handleBack}
                        onBaseUrlChange={(value) => {
                          setBaseUrl(value);
                          markConfigurationEdited();
                        }}
                        onModelIDChange={(value) => {
                          setModelId(value);
                          markConfigurationEdited();
                        }}
                        onStartNewIntent={startNewIntent}
                        onSubmit={handleSubmit}
                        setup={selected}
                        submitLabel={submitLabel}
                    />
                  ) : null}
                  {scene === "custom" ? (
                    <CustomProviderScene
                        apiFormat={customApiFormat}
                        apiKey={customApiKey}
                        baseUrl={customBaseUrl}
                        error={error}
                        errorKind={errorKind}
                        existingProvider={customExistingProvider}
                        formats={customSetups}
                        locked={persistenceLocked}
                        modelId={customModelId}
                        onApiFormatChange={(value) => {
                          const nextSetup = customSetups.find(
                            (item) => item.format.api_format === value,
                          );
                          if (nextSetup) {
                            setCustomApiFormat(nextSetup.format.api_format);
                            markConfigurationEdited();
                          }
                        }}
                        onApiKeyChange={(value) => {
                          setCustomApiKey(value);
                          markConfigurationEdited();
                        }}
                        onBack={handleBack}
                        onBaseUrlChange={(value) => {
                          setCustomBaseUrl(value);
                          markConfigurationEdited();
                        }}
                        onModelIDChange={(value) => {
                          setCustomModelId(value);
                          markConfigurationEdited();
                        }}
                        onNameChange={(value) => {
                          setCustomProviderName(value);
                          markConfigurationEdited();
                        }}
                        onStartNewIntent={startNewIntent}
                        onSubmit={handleCustomSubmit}
                        providerName={customProviderName}
                        submitLabel={submitLabel}
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

function ProviderSetupFailure({
  kind,
}: {
  kind: SetupFailureKind;
}) {
  const { t } = useI18n();
  let recovery: { impact: string; nextStep: string; problem: string };
  switch (kind) {
    case "read":
      recovery = {
        impact: t("state.read_failure_impact"),
        nextStep: t("state.retry_next_step"),
        problem: t("onboarding.provider_setup_load_failed"),
      };
      break;
    case "validation":
      recovery = {
        impact: t("state.validation_failure_impact"),
        nextStep: t("state.validation_failure_next_step"),
        problem: t("onboarding.provider_setup_validation_problem"),
      };
      break;
    case "journal_before_submit":
      recovery = {
        impact: t("onboarding.provider_setup_journal_before_impact"),
        nextStep: t("onboarding.provider_setup_journal_before_next"),
        problem: t("onboarding.provider_setup_journal_before_problem"),
      };
      break;
    case "journal_after_save":
      recovery = {
        impact: t("onboarding.provider_setup_journal_after_impact"),
        nextStep: t("onboarding.provider_setup_journal_after_next"),
        problem: t("onboarding.provider_setup_journal_after_problem"),
      };
      break;
    case "persist_not_applied":
      recovery = {
        impact: t("onboarding.provider_setup_persist_not_applied_impact"),
        nextStep: t("onboarding.provider_setup_persist_not_applied_next"),
        problem: t("onboarding.provider_setup_persist_not_applied_problem"),
      };
      break;
    case "persist_unknown":
      recovery = {
        impact: t("onboarding.provider_setup_persist_unknown_impact"),
        nextStep: t("onboarding.provider_setup_persist_unknown_next"),
        problem: t("onboarding.provider_setup_persist_unknown_problem"),
      };
      break;
    case "test_failed":
      recovery = {
        impact: t("onboarding.provider_setup_test_failed_impact"),
        nextStep: t("onboarding.provider_setup_test_failed_next"),
        problem: t("onboarding.provider_setup_test_failed_problem"),
      };
      break;
    case "test_not_applied":
      recovery = {
        impact: t("onboarding.provider_setup_test_not_applied_impact"),
        nextStep: t("onboarding.provider_setup_test_not_applied_next"),
        problem: t("onboarding.provider_setup_test_not_applied_problem"),
      };
      break;
    case "test_unknown":
      recovery = {
        impact: t("onboarding.provider_setup_test_unknown_impact"),
        nextStep: t("onboarding.provider_setup_test_unknown_next"),
        problem: t("onboarding.provider_setup_test_unknown_problem"),
      };
      break;
    case "default_not_applied":
      recovery = {
        impact: t("onboarding.provider_setup_default_not_applied_impact"),
        nextStep: t("onboarding.provider_setup_default_not_applied_next"),
        problem: t("onboarding.provider_setup_default_not_applied_problem"),
      };
      break;
    case "default_unknown":
      recovery = {
        impact: t("onboarding.provider_setup_default_unknown_impact"),
        nextStep: t("onboarding.provider_setup_default_unknown_next"),
        problem: t("onboarding.provider_setup_default_unknown_problem"),
      };
      break;
  }
  return (
    <ProviderSetupFailureView
      impact={recovery.impact}
      nextStep={recovery.nextStep}
      problem={recovery.problem}
      tone={kind.endsWith("unknown") || kind === "journal_after_save" ? "warning" : "danger"}
    />
  );
}

function ProviderScene({
  error,
  errorKind,
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
  errorKind: SetupFailureKind;
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
        {!loading && error ? (
          <ProviderSetupFailure kind={errorKind} />
        ) : null}
        {!loading && !error && presets.length === 0 ? (
          <div className={getDialogNoteClassName("danger")} role="status">
            {t("onboarding.provider_setup_provider_empty")}
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
  errorKind,
  existingProvider,
  locked,
  modelId,
  modelRequired,
  onApiKeyChange,
  onBack,
  onBaseUrlChange,
  onModelIDChange,
  onStartNewIntent,
  onSubmit,
  setup,
  submitLabel,
}: {
  apiKey: string;
  apiKeyRequired: boolean;
  baseUrl: string;
  error: string | null;
  errorKind: SetupFailureKind;
  existingProvider: ProviderConfigRecord | null;
  locked: boolean;
  modelId: string;
  modelRequired: boolean;
  onApiKeyChange: (value: string) => void;
  onBack: () => void;
  onBaseUrlChange: (value: string) => void;
  onModelIDChange: (value: string) => void;
  onStartNewIntent: () => void;
  onSubmit: () => void;
  setup: ProviderSetupPreset;
  submitLabel: string;
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
          required={apiKeyRequired && !locked}
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
            readOnly={locked}
            required={apiKeyRequired && !locked}
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
            required={!locked}
          >
            <UiInput
              autoCapitalize="off"
              autoCorrect="off"
              controlSize="md"
              id="provider-setup-base-url"
              onChange={(event) => onBaseUrlChange(event.target.value)}
              placeholder={setup.format.base_url_placeholder || t("onboarding.provider_setup_base_url_placeholder")}
              readOnly={locked}
              required={!locked}
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
            required={!locked}
          >
            <UiInput
              autoCapitalize="off"
              autoCorrect="off"
              controlSize="md"
              id="provider-setup-model-id"
              onChange={(event) => onModelIDChange(event.target.value)}
              placeholder={t("onboarding.provider_setup_model_placeholder")}
              readOnly={locked}
              required={!locked}
              spellCheck={false}
              type="text"
              value={modelId}
            />
          </UiField>
        ) : null}

        {error ? <ProviderSetupFailure kind={errorKind} /> : null}
      </div>

      <div className="flex shrink-0 items-center justify-end gap-2 border-t border-(--divider-subtle-color) pb-5 pt-3">
        <UiButton
          onClick={locked ? onStartNewIntent : onBack}
          size="sm"
          type="button"
          variant="text"
        >
          {locked ? null : <ChevronLeft className="h-3.5 w-3.5" />}
          {locked
            ? t("onboarding.provider_setup_new_intent_action")
            : t("onboarding.provider_setup_back")}
        </UiButton>
        <UiButton size="sm" tone="primary" type="submit" variant="solid">
          {submitLabel}
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
  errorKind,
  existingProvider,
  formats,
  locked,
  modelId,
  onApiFormatChange,
  onApiKeyChange,
  onBack,
  onBaseUrlChange,
  onModelIDChange,
  onNameChange,
  onStartNewIntent,
  onSubmit,
  providerName,
  submitLabel,
}: {
  apiFormat: ProviderApiFormat | "";
  apiKey: string;
  baseUrl: string;
  error: string | null;
  errorKind: SetupFailureKind;
  existingProvider: ProviderConfigRecord | null;
  formats: ProviderSetupPreset[];
  locked: boolean;
  modelId: string;
  onApiFormatChange: (value: string) => void;
  onApiKeyChange: (value: string) => void;
  onBack: () => void;
  onBaseUrlChange: (value: string) => void;
  onModelIDChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onStartNewIntent: () => void;
  onSubmit: () => void;
  providerName: string;
  submitLabel: string;
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
            required={!locked}
          >
            <UiInput
              autoCapitalize="off"
              autoCorrect="off"
              controlSize="md"
              id="provider-setup-custom-name"
              onChange={(event) => onNameChange(event.target.value)}
              placeholder={t("onboarding.provider_setup_custom_name_placeholder")}
              readOnly={locked}
              required={!locked}
              spellCheck={false}
              type="text"
              value={providerName}
            />
          </UiField>
          <UiField
            htmlFor="provider-setup-custom-format"
            label={t("onboarding.provider_setup_custom_format")}
            required={!locked}
          >
            <UiSelectMenu
              ariaLabel={t("onboarding.provider_setup_custom_format")}
              className="w-full"
              disabled={locked}
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
          required={!locked && !existingProvider?.auth_token_masked?.trim()}
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
            readOnly={locked}
            required={!locked && !existingProvider?.auth_token_masked?.trim()}
            spellCheck={false}
            type="password"
            value={apiKey}
          />
        </UiField>

        <UiField
          htmlFor="provider-setup-custom-base-url"
          label={t("onboarding.provider_setup_base_url")}
          required={!locked}
        >
          <UiInput
            autoCapitalize="off"
            autoCorrect="off"
            controlSize="md"
            id="provider-setup-custom-base-url"
            onChange={(event) => onBaseUrlChange(event.target.value)}
            placeholder={t("onboarding.provider_setup_base_url_placeholder")}
            readOnly={locked}
            required={!locked}
            spellCheck={false}
            type="url"
            value={baseUrl}
          />
        </UiField>

        <UiField
          htmlFor="provider-setup-custom-model-id"
          label={t("onboarding.provider_setup_model")}
          required={!locked}
        >
          <UiInput
            autoCapitalize="off"
            autoCorrect="off"
            controlSize="md"
            id="provider-setup-custom-model-id"
            onChange={(event) => onModelIDChange(event.target.value)}
            placeholder={t("onboarding.provider_setup_model_placeholder")}
            readOnly={locked}
            required={!locked}
            spellCheck={false}
            type="text"
            value={modelId}
          />
        </UiField>

        {error ? <ProviderSetupFailure kind={errorKind} /> : null}
      </div>

      <div className="flex shrink-0 items-center justify-end gap-2 border-t border-(--divider-subtle-color) pb-5 pt-3">
        <UiButton
          onClick={locked ? onStartNewIntent : onBack}
          size="sm"
          type="button"
          variant="text"
        >
          {locked ? null : <ChevronLeft className="h-3.5 w-3.5" />}
          {locked
            ? t("onboarding.provider_setup_new_intent_action")
            : t("onboarding.provider_setup_back")}
        </UiButton>
        <UiButton size="sm" tone="primary" type="submit" variant="solid">
          {submitLabel}
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
      <h3 className="text-lg font-semibold tracking-[-0.02em] text-(--text-strong)">
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

async function createProviderSetupJournal(
  ownerScope: string | null,
  draft: ProviderConnectionDraft,
): Promise<ProviderSetupJournal | null> {
  const normalizedOwnerScope = ownerScope?.trim() ?? "";
  const baselineConfigurationVersion = draft.existingProvider?.configuration_version ?? null;
  if (
    !normalizedOwnerScope
    || draft.existingProvider && !validConfigurationVersion(baselineConfigurationVersion)
  ) {
    return null;
  }
  const configurationFingerprint = await fingerprintProviderSetup({
    apiFormat: draft.apiFormat,
    baseURL: draft.baseURL,
    displayName: draft.displayName,
    enabled: true,
    modelsPath: draft.modelsPath,
    presetKey: draft.presetKey,
    providerKind: "llm",
  });
  return {
    apiFormat: draft.apiFormat,
    baselineConfigurationVersion,
    configurationFingerprint,
    configurationVersion: null,
    credentialMode: draft.apiKey ? "replace" : "preserve",
    model: draft.modelID,
    outcome: "ready",
    ownerScope: normalizedOwnerScope,
    preferencesBaselineVersion: null,
    presetKey: draft.presetKey,
    providerDisplayName: draft.displayName,
    providerKey: draft.providerKey,
    providerWasExisting: Boolean(draft.existingProvider),
    stage: "persist",
    testBaselineAt: draft.existingProvider?.last_test_at?.trim() || null,
    version: 1,
  };
}

function providerSetupPayload(
  draft: ProviderConnectionDraft,
): UpdateProviderConfigPayload {
  return {
    api_format: draft.apiFormat,
    base_url: draft.baseURL,
    display_name: draft.displayName,
    enabled: true,
    models_path: draft.modelsPath,
    preset_key: draft.presetKey,
    provider_kind: "llm",
    ...(draft.apiKey ? { auth_token: draft.apiKey } : {}),
  };
}

function storeJournalBeforeEffect(
  next: ProviderSetupJournal,
  setJournal: Dispatch<SetStateAction<ProviderSetupJournal | null>>,
): boolean {
  if (!writeProviderSetupJournal(next)) {
    return false;
  }
  setJournal(next);
  return true;
}

function abandonProviderSetupJournal(
  ownerScope: string | null,
  setJournal: Dispatch<SetStateAction<ProviderSetupJournal | null>>,
): void {
  if (ownerScope) {
    removeProviderSetupJournal(ownerScope);
  }
  setJournal(null);
}

function validConfigurationVersion(value: unknown): value is number {
  return typeof value === "number"
    && Number.isSafeInteger(value)
    && value > 0;
}

function restoreProviderSetupJournal({
  journal,
  providers,
  setBaseUrl,
  setCustomBaseUrl,
  setCustomModelId,
  setCustomProviderName,
  setModelId,
  setScene,
  setSelectedPresetKey,
}: {
  journal: ProviderSetupJournal;
  providers: readonly ProviderConfigRecord[];
  setBaseUrl: Dispatch<SetStateAction<string>>;
  setCustomBaseUrl: Dispatch<SetStateAction<string>>;
  setCustomModelId: Dispatch<SetStateAction<string>>;
  setCustomProviderName: Dispatch<SetStateAction<string>>;
  setModelId: Dispatch<SetStateAction<string>>;
  setScene: Dispatch<SetStateAction<SetupScene>>;
  setSelectedPresetKey: Dispatch<SetStateAction<string>>;
}): void {
  const record = providers.find((item) => (
    item.can_manage && item.provider === journal.providerKey
  )) ?? null;
  const model = journal.model || defaultModelID(record);
  if (journal.presetKey === "custom") {
    setCustomProviderName(record?.display_name || journal.providerDisplayName);
    setCustomBaseUrl(record?.base_url || "");
    setCustomModelId(model);
    setScene("custom");
    return;
  }
  setSelectedPresetKey(journal.presetKey);
  setBaseUrl(record?.base_url || "");
  setModelId(model);
  setScene("credentials");
}

function failureKindForUnknownStage(
  stage: ProviderSetupJournal["stage"],
): SetupFailureKind {
  if (stage === "persist") {
    return "persist_unknown";
  }
  if (stage === "test") {
    return "test_unknown";
  }
  return "default_unknown";
}

function failureMessageKeyForUnknownStage(
  stage: ProviderSetupJournal["stage"],
): "onboarding.provider_setup_default_unknown_problem"
  | "onboarding.provider_setup_persist_unknown_problem"
  | "onboarding.provider_setup_test_unknown_problem" {
  if (stage === "persist") {
    return "onboarding.provider_setup_persist_unknown_problem";
  }
  if (stage === "test") {
    return "onboarding.provider_setup_test_unknown_problem";
  }
  return "onboarding.provider_setup_default_unknown_problem";
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
  const savedPreferences = await updateDefaultModelSelections(
    currentPreferences,
    provider,
    model,
  );
  setUserPreferences(savedPreferences);
  invalidateProviderAvailability();
}

async function updateDefaultModelSelections(
  currentPreferences: UserPreferences,
  provider: string,
  model: string,
): Promise<UserPreferences> {
  if (!validConfigurationVersion(currentPreferences.version)) {
    throw new Error("Preferences version is unavailable");
  }
  return updateUserPreferencesApi({
    default_agent_options: {
      ...currentPreferences.default_agent_options,
      model,
      provider,
    },
    default_background_model_selection: { model, provider },
  }, { expectedVersion: currentPreferences.version });
}
