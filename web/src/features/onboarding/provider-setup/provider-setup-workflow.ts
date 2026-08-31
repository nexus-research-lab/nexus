/**
 * INPUT: Provider 连接草稿、owner generation、当前恢复 journal 与可替换的领域服务。
 * OUTPUT: save → test → default 的串行结果；未知写入只执行精确读取对账，不重放副作用。
 * POS: Provider 初始化的副作用编排边界；React 只消费阶段、journal、目录和最终结果。
 */

import { setUserPreferences } from "@/config/runtime-options";
import { invalidateProviderAvailability } from "@/hooks/capability/use-provider-availability";
import {
  createProviderConfigApi,
  listProviderConfigsApi,
  testProviderConfigApi,
  testProviderModelApi,
  updateProviderConfigApi,
} from "@/lib/api/settings/provider-api";
import {
  getUserPreferencesApi,
  updateUserPreferencesApi,
} from "@/lib/api/settings/preferences-api";
import { projectMutationFailure } from "@/lib/error-message";
import {
  assertAuthOwnerScopeGenerationCurrent,
  isAuthOwnerScopeSupersededError,
} from "@/shared/auth/auth-owner-generation";
import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderTestResult,
  UpdateProviderConfigPayload,
} from "@/types/capability/provider";
import type {
  UpdateUserPreferencesParams,
  UserPreferences,
} from "@/types/settings/preferences";

import {
  fingerprintProviderSetup,
  preferencesUseProviderSelection,
  reconcileProviderPersist,
  reconcileProviderTest,
  removeProviderSetupJournal,
  writeProviderSetupJournal,
  type ProviderSetupJournal,
} from "./provider-setup-recovery";

export type ProviderSetupFailureScene = "credentials" | "custom";
export type ProviderSetupFailureKind =
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

export interface ProviderSetupFailure {
  kind: ProviderSetupFailureKind;
  problemKey: TranslationKey;
}

export interface ProviderConnectionDraft {
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

export interface ProviderSetupResult {
  model: string;
  provider: string;
}

export interface ProviderSetupWorkflowServices {
  createProvider: typeof createProviderConfigApi;
  getPreferences: typeof getUserPreferencesApi;
  listProviders: typeof listProviderConfigsApi;
  testProvider: typeof testProviderConfigApi;
  testProviderModel: typeof testProviderModelApi;
  updatePreferences: (
    params: UpdateUserPreferencesParams,
    options?: { expectedVersion?: number },
  ) => Promise<UserPreferences>;
  updateProvider: typeof updateProviderConfigApi;
}

export interface ProviderSetupWorkflowCallbacks {
  onJournal: (journal: ProviderSetupJournal | null) => void;
  onPhase: (phase: 0 | 1 | 2) => void;
  onProviders: (providers: ProviderConfigRecord[]) => void;
}

export type ProviderSetupWorkflowResult =
  | {
    failure: ProviderSetupFailure;
    kind: "blocked";
  }
  | {
    kind: "complete";
    preferences: UserPreferences;
    result: ProviderSetupResult;
  };

const DEFAULT_SERVICES: ProviderSetupWorkflowServices = {
  createProvider: createProviderConfigApi,
  getPreferences: getUserPreferencesApi,
  listProviders: listProviderConfigsApi,
  testProvider: testProviderConfigApi,
  testProviderModel: testProviderModelApi,
  updatePreferences: updateUserPreferencesApi,
  updateProvider: updateProviderConfigApi,
};

interface RunProviderSetupWorkflowInput {
  callbacks: ProviderSetupWorkflowCallbacks;
  currentJournal: ProviderSetupJournal | null;
  draft: ProviderConnectionDraft;
  ownerGeneration: number;
  ownerScope: string | null;
  services?: ProviderSetupWorkflowServices;
}

interface WorkflowContext {
  callbacks: ProviderSetupWorkflowCallbacks;
  draft: ProviderConnectionDraft;
  ownerGeneration: number;
  services: ProviderSetupWorkflowServices;
}

/** One invocation advances only stages whose previous effect is proven. */
export async function runProviderSetupWorkflow({
  callbacks,
  currentJournal,
  draft,
  ownerGeneration,
  ownerScope,
  services = DEFAULT_SERVICES,
}: RunProviderSetupWorkflowInput): Promise<ProviderSetupWorkflowResult> {
  assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
  const context: WorkflowContext = {
    callbacks,
    draft,
    ownerGeneration,
    services,
  };
  let journal = currentJournal;
  if (journal && journal.providerKey !== draft.providerKey) {
    return blocked("persist_unknown");
  }
  if (!journal) {
    try {
      journal = await createProviderSetupJournal(ownerScope, draft);
    } catch (error) {
      if (isAuthOwnerScopeSupersededError(error)) {
        throw error;
      }
      return blocked("journal_before_submit");
    }
    assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
    if (!journal || !storeJournal(journal, callbacks)) {
      return blocked("journal_before_submit");
    }
  }

  if (journal.stage === "persist") {
    callbacks.onPhase(0);
    const persisted = await ensureProviderPersisted(context, journal);
    assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
    if (persisted.kind === "blocked") {
      return persisted;
    }
    journal = persisted.journal;
  }
  if (journal.stage === "test") {
    callbacks.onPhase(1);
    const tested = await ensureProviderTested(context, journal);
    assertAuthOwnerScopeGenerationCurrent(ownerGeneration);
    if (tested.kind === "blocked") {
      return tested;
    }
    journal = tested.journal;
  }
  if (journal.stage !== "default") {
    return blocked("default_unknown");
  }
  callbacks.onPhase(2);
  return ensureDefaultSelection(context, journal);
}

type StageResult =
  | { failure: ProviderSetupFailure; kind: "blocked" }
  | { journal: ProviderSetupJournal; kind: "advanced" };

async function ensureProviderPersisted(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
): Promise<StageResult> {
  if (journal.outcome === "unknown") {
    return reconcilePersistStage(context, journal);
  }
  const inFlight = { ...journal, outcome: "unknown" as const };
  if (!storeJournal(inFlight, context.callbacks)) {
    return blocked("journal_before_submit");
  }
  try {
    const payload = providerSetupPayload(context.draft);
    const record = context.draft.existingProvider
      ? await context.services.updateProvider(
        context.draft.existingProvider.provider,
        payload,
        { expectedVersion: journal.baselineConfigurationVersion ?? undefined },
      )
      : await context.services.createProvider({
        ...payload,
        auth_token: context.draft.apiKey,
        provider: context.draft.providerKey,
        provider_kind: "llm",
        visibility: "private",
      });
    assertCurrent(context);
    const confirmed = await reconcileProviderPersist(inFlight, [record], true);
    if (confirmed.kind === "applied") {
      return advanceAfterPersist(context, inFlight, confirmed.record);
    }
    return reconcilePersistStage(context, inFlight);
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
    const failure = projectMutationFailure(
      error,
      "Provider setup persistence result is unknown",
    );
    if (failure.effect === "not_applied") {
      storeKnownStage(inFlight, context.callbacks);
      return blocked("persist_not_applied");
    }
    return reconcilePersistStage(
      context,
      inFlight,
      failure.effect === "committed",
    );
  }
}

async function reconcilePersistStage(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
  commitConfirmed = false,
): Promise<StageResult> {
  try {
    const providers = await context.services.listProviders();
    assertCurrent(context);
    context.callbacks.onProviders(providers);
    const outcome = await reconcileProviderPersist(
      journal,
      providers,
      commitConfirmed,
    );
    assertCurrent(context);
    if (outcome.kind === "applied") {
      return advanceAfterPersist(context, journal, outcome.record);
    }
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
  }
  context.callbacks.onJournal(journal);
  return blocked("persist_unknown");
}

async function advanceAfterPersist(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
  record: ProviderConfigRecord,
): Promise<StageResult> {
  if (!validConfigurationVersion(record.configuration_version)) {
    return blocked("journal_after_save");
  }
  const next: ProviderSetupJournal = {
    ...journal,
    configurationVersion: record.configuration_version,
    outcome: "ready",
    providerDisplayName: record.display_name || journal.providerDisplayName,
    stage: "test",
    testBaselineAt: record.last_test_at?.trim() || null,
  };
  return storeJournal(next, context.callbacks)
    ? { journal: next, kind: "advanced" }
    : blocked("journal_after_save");
}

async function ensureProviderTested(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
): Promise<StageResult> {
  if (journal.outcome === "unknown") {
    return reconcileTestStage(context, journal);
  }
  const inFlight = { ...journal, outcome: "unknown" as const };
  if (!storeJournal(inFlight, context.callbacks)) {
    return blocked("journal_after_save");
  }
  try {
    const options = { expectedVersion: journal.configurationVersion ?? undefined };
    const result = journal.model
      ? await context.services.testProviderModel(journal.providerKey, journal.model, options)
      : await context.services.testProvider(journal.providerKey, options);
    assertCurrent(context);
    if (!testResultMatchesJournal(inFlight, result)) {
      return reconcileTestStage(context, inFlight);
    }
    const testedJournal: ProviderSetupJournal = {
      ...inFlight,
      configurationVersion: result.configuration_version,
      model: result.model?.trim() || journal.model,
      testBaselineAt: result.tested_at?.trim() || journal.testBaselineAt,
    };
    if (!result.success) {
      const ready = { ...testedJournal, outcome: "ready" as const };
      storeKnownStage(ready, context.callbacks);
      await refreshProviders(context);
      return blocked("test_failed");
    }
    return advanceAfterTest(context, testedJournal);
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
    const failure = projectMutationFailure(
      error,
      "Provider connection test result is unknown",
    );
    if (failure.effect === "not_applied") {
      await refreshProviders(context);
      storeKnownStage(inFlight, context.callbacks);
      return blocked("test_not_applied");
    }
    return reconcileTestStage(context, inFlight);
  }
}

async function reconcileTestStage(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
): Promise<StageResult> {
  try {
    const providers = await context.services.listProviders();
    assertCurrent(context);
    context.callbacks.onProviders(providers);
    const outcome = reconcileProviderTest(journal, providers);
    if (outcome.kind === "passed") {
      const model = journal.model || defaultModelID(outcome.record);
      return advanceAfterTest(context, {
        ...journal,
        configurationVersion: outcome.record.configuration_version,
        model,
        testBaselineAt: outcome.record.last_test_at?.trim() || journal.testBaselineAt,
      });
    }
    if (outcome.kind === "failed") {
      const ready: ProviderSetupJournal = {
        ...journal,
        configurationVersion: outcome.record.configuration_version,
        outcome: "ready",
        testBaselineAt: outcome.record.last_test_at?.trim() || journal.testBaselineAt,
      };
      storeKnownStage(ready, context.callbacks);
      return blocked("test_failed");
    }
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
  }
  context.callbacks.onJournal(journal);
  return blocked("test_unknown");
}

function advanceAfterTest(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
): StageResult {
  if (!journal.model.trim() || !validConfigurationVersion(journal.configurationVersion)) {
    return blocked("test_unknown");
  }
  const next: ProviderSetupJournal = {
    ...journal,
    outcome: "ready",
    preferencesBaselineVersion: null,
    stage: "default",
  };
  return storeJournal(next, context.callbacks)
    ? { journal: next, kind: "advanced" }
    : blocked("journal_after_save");
}

async function ensureDefaultSelection(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
): Promise<ProviderSetupWorkflowResult> {
  if (journal.outcome === "unknown") {
    return reconcileDefaultStage(context, journal);
  }
  let current: UserPreferences;
  try {
    current = await context.services.getPreferences();
    assertCurrent(context);
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
    return blocked("default_not_applied");
  }
  if (!validConfigurationVersion(current.version)) {
    return blocked("default_not_applied");
  }
  if (preferencesUseProviderSelection(current, journal.providerKey, journal.model)) {
    return completeWorkflow(context, journal, current);
  }
  const inFlight = {
    ...journal,
    outcome: "unknown" as const,
    preferencesBaselineVersion: current.version,
  };
  if (!storeJournal(inFlight, context.callbacks)) {
    return blocked("journal_after_save");
  }
  try {
    const saved = await updateDefaultModelSelections(
      context.services,
      current,
      journal.providerKey,
      journal.model,
    );
    assertCurrent(context);
    if (
      saved.version !== current.version + 1
      || !preferencesUseProviderSelection(saved, journal.providerKey, journal.model)
    ) {
      return reconcileDefaultStage(context, inFlight);
    }
    return completeWorkflow(context, inFlight, saved);
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
    const failure = projectMutationFailure(
      error,
      "Default model update result is unknown",
    );
    if (failure.effect === "not_applied") {
      storeKnownStage(inFlight, context.callbacks);
      return blocked("default_not_applied");
    }
    return reconcileDefaultStage(context, inFlight);
  }
}

async function reconcileDefaultStage(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
): Promise<ProviderSetupWorkflowResult> {
  try {
    const current = await context.services.getPreferences();
    assertCurrent(context);
    const baseline = journal.preferencesBaselineVersion;
    if (
      baseline !== null
      && current.version === baseline + 1
      && preferencesUseProviderSelection(current, journal.providerKey, journal.model)
    ) {
      return completeWorkflow(context, journal, current);
    }
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
  }
  context.callbacks.onJournal(journal);
  return blocked("default_unknown");
}

function completeWorkflow(
  context: WorkflowContext,
  journal: ProviderSetupJournal,
  preferences: UserPreferences,
): ProviderSetupWorkflowResult {
  setUserPreferences(preferences);
  invalidateProviderAvailability();
  writeProviderSetupJournal({
    ...journal,
    outcome: "ready",
    stage: "complete",
  });
  removeProviderSetupJournal(journal.ownerScope);
  context.callbacks.onJournal(null);
  return {
    kind: "complete",
    preferences,
    result: {
      model: journal.model,
      provider: journal.providerDisplayName,
    },
  };
}

export async function persistImportedDefaultSelection(
  selection: { model: string; provider: string },
  services: Pick<ProviderSetupWorkflowServices, "getPreferences" | "updatePreferences"> = DEFAULT_SERVICES,
): Promise<UserPreferences> {
  const current = await services.getPreferences();
  const saved = await updateDefaultModelSelections(
    services,
    current,
    selection.provider,
    selection.model,
  );
  if (
    !validConfigurationVersion(current.version)
    || saved.version !== current.version + 1
    || !preferencesUseProviderSelection(saved, selection.provider, selection.model)
  ) {
    throw new Error("Imported default model result is unconfirmed");
  }
  setUserPreferences(saved);
  invalidateProviderAvailability();
  return saved;
}

async function createProviderSetupJournal(
  ownerScope: string | null,
  draft: ProviderConnectionDraft,
): Promise<ProviderSetupJournal | null> {
  const normalizedOwnerScope = ownerScope?.trim() ?? "";
  const baseline = draft.existingProvider?.configuration_version ?? null;
  if (
    !normalizedOwnerScope
    || draft.existingProvider && !validConfigurationVersion(baseline)
  ) {
    return null;
  }
  return {
    apiFormat: draft.apiFormat,
    baselineConfigurationVersion: baseline,
    configurationFingerprint: await fingerprintProviderSetup({
      apiFormat: draft.apiFormat,
      baseURL: draft.baseURL,
      displayName: draft.displayName,
      enabled: true,
      modelsPath: draft.modelsPath,
      presetKey: draft.presetKey,
      providerKind: "llm",
    }),
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

function storeJournal(
  journal: ProviderSetupJournal,
  callbacks: ProviderSetupWorkflowCallbacks,
): boolean {
  if (!writeProviderSetupJournal(journal)) {
    return false;
  }
  callbacks.onJournal(journal);
  return true;
}

function storeKnownStage(
  journal: ProviderSetupJournal,
  callbacks: ProviderSetupWorkflowCallbacks,
): void {
  const ready = { ...journal, outcome: "ready" as const };
  writeProviderSetupJournal(ready);
  callbacks.onJournal(ready);
}

async function refreshProviders(context: WorkflowContext): Promise<void> {
  try {
    const providers = await context.services.listProviders();
    assertCurrent(context);
    context.callbacks.onProviders(providers);
  } catch (error) {
    if (isAuthOwnerScopeSupersededError(error)) {
      throw error;
    }
  }
}

function testResultMatchesJournal(
  journal: ProviderSetupJournal,
  result: ProviderTestResult,
): boolean {
  const baselineVersion = journal.configurationVersion;
  const testedAt = result.tested_at?.trim() ?? "";
  const resultModel = result.model?.trim() ?? "";
  return baselineVersion !== null
    && result.provider.trim() === journal.providerKey
    && result.configuration_version === baselineVersion + 1
    && Boolean(testedAt)
    && testedAt !== (journal.testBaselineAt?.trim() ?? "")
    && (!journal.model || !resultModel || resultModel === journal.model);
}

async function updateDefaultModelSelections(
  services: Pick<ProviderSetupWorkflowServices, "updatePreferences">,
  current: UserPreferences,
  provider: string,
  model: string,
): Promise<UserPreferences> {
  if (!validConfigurationVersion(current.version)) {
    throw new Error("Preferences version is unavailable");
  }
  return services.updatePreferences({
    default_agent_options: {
      ...current.default_agent_options,
      model,
      provider,
    },
    default_background_model_selection: { model, provider },
  }, { expectedVersion: current.version });
}

function assertCurrent(context: WorkflowContext): void {
  assertAuthOwnerScopeGenerationCurrent(context.ownerGeneration);
}

function validConfigurationVersion(value: unknown): value is number {
  return typeof value === "number"
    && Number.isSafeInteger(value)
    && value > 0;
}

function defaultModelID(provider: ProviderConfigRecord): string {
  return provider.models.find((model) => model.is_default && model.enabled)?.model_id
    ?? provider.models.find((model) => model.enabled)?.model_id
    ?? "";
}

function blocked(kind: ProviderSetupFailureKind): ProviderSetupWorkflowResult & {
  kind: "blocked";
} {
  return {
    failure: {
      kind,
      problemKey: failureProblemKey(kind),
    },
    kind: "blocked",
  };
}

function failureProblemKey(kind: ProviderSetupFailureKind): TranslationKey {
  switch (kind) {
    case "journal_before_submit":
      return "onboarding.provider_setup_journal_before_problem";
    case "journal_after_save":
      return "onboarding.provider_setup_journal_after_problem";
    case "persist_not_applied":
      return "onboarding.provider_setup_persist_not_applied_problem";
    case "persist_unknown":
      return "onboarding.provider_setup_persist_unknown_problem";
    case "test_failed":
      return "onboarding.provider_setup_test_failed_problem";
    case "test_not_applied":
      return "onboarding.provider_setup_test_not_applied_problem";
    case "test_unknown":
      return "onboarding.provider_setup_test_unknown_problem";
    case "default_not_applied":
      return "onboarding.provider_setup_default_not_applied_problem";
    case "default_unknown":
      return "onboarding.provider_setup_default_unknown_problem";
    case "read":
      return "onboarding.provider_setup_load_failed";
    case "validation":
      return "onboarding.provider_setup_api_key_required";
  }
}
