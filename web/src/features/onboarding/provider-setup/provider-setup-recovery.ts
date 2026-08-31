/**
 * INPUT: Auth owner scope, exact Provider key/version, non-secret setup stage facts and later authoritative reads.
 * OUTPUT: Owner-scoped durable stage journal plus conservative Provider/test/default reconciliation results.
 * POS: Provider setup recovery boundary; it never stores credentials, request bodies, Base URLs or transport IDs.
 */

import type {
  ProviderApiFormat,
  ProviderConfigRecord,
} from "@/types/capability/provider";
import type { UserPreferences } from "@/types/settings/preferences";

const PROVIDER_SETUP_JOURNAL_PREFIX = "nexus.provider_setup.journal.v1";
const MAX_JOURNAL_STRING_LENGTH = 512;

export type ProviderSetupJournalStage =
  | "complete"
  | "default"
  | "persist"
  | "test";

export interface ProviderSetupJournal {
  apiFormat: ProviderApiFormat;
  baselineConfigurationVersion: number | null;
  configurationFingerprint: string;
  configurationVersion: number | null;
  credentialMode: "preserve" | "replace";
  model: string;
  outcome: "ready" | "unknown";
  ownerScope: string;
  preferencesBaselineVersion: number | null;
  presetKey: string;
  providerDisplayName: string;
  providerKey: string;
  providerWasExisting: boolean;
  stage: ProviderSetupJournalStage;
  testBaselineAt: string | null;
  version: 1;
}

export interface ProviderSetupFingerprintInput {
  apiFormat: ProviderApiFormat;
  baseURL: string;
  displayName: string;
  enabled: boolean;
  modelsPath: string;
  presetKey: string;
  providerKind: "llm";
}

export type ProviderSetupJournalRead =
  | { journal: ProviderSetupJournal | null; status: "available" }
  | { journal: null; status: "unavailable" };

export type ProviderPersistReconciliation =
  | { kind: "applied"; record: ProviderConfigRecord }
  | { kind: "unproven"; record: ProviderConfigRecord | null };

export type ProviderTestReconciliation =
  | { kind: "failed"; record: ProviderConfigRecord }
  | { kind: "passed"; record: ProviderConfigRecord }
  | { kind: "unproven"; record: ProviderConfigRecord | null };

const JOURNAL_FIELDS = [
  "apiFormat",
  "baselineConfigurationVersion",
  "configurationFingerprint",
  "configurationVersion",
  "credentialMode",
  "model",
  "outcome",
  "ownerScope",
  "preferencesBaselineVersion",
  "presetKey",
  "providerDisplayName",
  "providerKey",
  "providerWasExisting",
  "stage",
  "testBaselineAt",
  "version",
] as const;

export function readProviderSetupJournal(
  ownerScope: string,
): ProviderSetupJournalRead {
  const storage = getStorage();
  const storageKey = journalStorageKey(ownerScope);
  if (!storage || !storageKey) {
    return { journal: null, status: "unavailable" };
  }
  try {
    const raw = storage.getItem(storageKey);
    if (!raw) {
      return { journal: null, status: "available" };
    }
    const journal = parseJournal(JSON.parse(raw), ownerScope);
    if (!journal || journal.stage === "complete") {
      storage.removeItem(storageKey);
      return { journal: null, status: "available" };
    }
    return { journal, status: "available" };
  } catch {
    try {
      storage.removeItem(storageKey);
    } catch {
      return { journal: null, status: "unavailable" };
    }
    return { journal: null, status: "available" };
  }
}

/** The write is verified before any next-stage side effect is allowed. */
export function writeProviderSetupJournal(
  journal: ProviderSetupJournal,
): boolean {
  const storage = getStorage();
  const storageKey = journalStorageKey(journal.ownerScope);
  const sanitized = sanitizeJournal(journal, journal.ownerScope);
  if (!storage || !storageKey || !sanitized) {
    return false;
  }
  try {
    const serialized = JSON.stringify(sanitized);
    storage.setItem(storageKey, serialized);
    return storage.getItem(storageKey) === serialized;
  } catch {
    return false;
  }
}

export function removeProviderSetupJournal(ownerScope: string): boolean {
  const storage = getStorage();
  const storageKey = journalStorageKey(ownerScope);
  if (!storage || !storageKey) {
    return false;
  }
  try {
    storage.removeItem(storageKey);
    return storage.getItem(storageKey) === null;
  } catch {
    return false;
  }
}

export async function fingerprintProviderSetup(
  input: ProviderSetupFingerprintInput,
): Promise<string> {
  if (
    typeof crypto === "undefined"
    || !crypto.subtle
    || typeof TextEncoder === "undefined"
  ) {
    throw new Error("Provider setup fingerprint is unavailable");
  }
  const canonical = JSON.stringify({
    api_format: input.apiFormat.trim(),
    base_url: input.baseURL.trim(),
    display_name: input.displayName.trim(),
    enabled: input.enabled,
    models_path: input.modelsPath.trim(),
    preset_key: input.presetKey.trim(),
    provider_kind: input.providerKind,
  });
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(canonical),
  );
  return Array.from(new Uint8Array(digest), (value) => (
    value.toString(16).padStart(2, "0")
  )).join("");
}

export async function reconcileProviderPersist(
  journal: ProviderSetupJournal,
  providers: readonly ProviderConfigRecord[],
  commitConfirmed = false,
): Promise<ProviderPersistReconciliation> {
  const record = findExactProvider(providers, journal.providerKey);
  if (!record) {
    return { kind: "unproven", record: null };
  }
  const baseline = journal.baselineConfigurationVersion;
  const expectedVersion = journal.providerWasExisting
    ? baseline === null ? null : baseline + 1
    : 1;
  if (expectedVersion === null || record.configuration_version !== expectedVersion) {
    return { kind: "unproven", record };
  }
  const fingerprint = await fingerprintProviderSetup({
    apiFormat: record.api_format,
    baseURL: record.base_url,
    displayName: record.display_name,
    enabled: record.enabled,
    modelsPath: record.models_path,
    presetKey: record.preset_key,
    providerKind: "llm",
  });
  const credentialCanBeProven = !journal.providerWasExisting
    || journal.credentialMode === "preserve"
    || commitConfirmed;
  return credentialCanBeProven
    && fingerprint === journal.configurationFingerprint
    ? { kind: "applied", record }
    : { kind: "unproven", record };
}

export function reconcileProviderTest(
  journal: ProviderSetupJournal,
  providers: readonly ProviderConfigRecord[],
): ProviderTestReconciliation {
  const record = findExactProvider(providers, journal.providerKey);
  const testedVersion = journal.configurationVersion;
  if (
    !record
    || testedVersion === null
    || record.configuration_version !== testedVersion + 1
    || !testTimestampAdvanced(journal.testBaselineAt, record.last_test_at)
  ) {
    return { kind: "unproven", record };
  }
  if (record.last_test_status === "success") {
    const model = journal.model.trim();
    const modelReady = model
      ? record.models.some((item) => item.model_id === model && item.enabled)
      : record.models.some((item) => item.enabled);
    return modelReady
      ? { kind: "passed", record }
      : { kind: "unproven", record };
  }
  return record.last_test_status === "failed"
    ? { kind: "failed", record }
    : { kind: "unproven", record };
}

export function preferencesUseProviderSelection(
  preferences: UserPreferences,
  provider: string,
  model: string,
): boolean {
  const selectionMatches = (selection: { provider?: string; model?: string } | undefined) => (
    selection?.provider?.trim() === provider.trim()
    && selection?.model?.trim() === model.trim()
  );
  return selectionMatches(preferences.default_agent_options)
    && selectionMatches(preferences.default_background_model_selection);
}

function findExactProvider(
  providers: readonly ProviderConfigRecord[],
  providerKey: string,
): ProviderConfigRecord | null {
  return providers.find((record) => (
    record.can_manage && record.provider === providerKey
  )) ?? null;
}

function getStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

function journalStorageKey(ownerScope: string): string | null {
  const normalized = ownerScope.trim();
  if (!normalized || normalized.length > MAX_JOURNAL_STRING_LENGTH) {
    return null;
  }
  return `${PROVIDER_SETUP_JOURNAL_PREFIX}:${normalized.length}:${normalized}`;
}

function parseJournal(value: unknown, ownerScope: string): ProviderSetupJournal | null {
  return sanitizeJournal(value, ownerScope);
}

function sanitizeJournal(
  value: unknown,
  ownerScope: string,
): ProviderSetupJournal | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  const journal = value as Partial<ProviderSetupJournal>;
  const valid = Object.keys(value).every((key) => (
    JOURNAL_FIELDS.includes(key as (typeof JOURNAL_FIELDS)[number])
  ))
    && journal.version === 1
    && journal.ownerScope === ownerScope
    && validString(journal.ownerScope)
    && validString(journal.providerKey)
    && validString(journal.presetKey)
    && validString(journal.providerDisplayName)
    && validString(journal.model, true)
    && validFingerprint(journal.configurationFingerprint)
    && validAPIFormat(journal.apiFormat)
    && typeof journal.providerWasExisting === "boolean"
    && (journal.credentialMode === "preserve" || journal.credentialMode === "replace")
    && (journal.outcome === "ready" || journal.outcome === "unknown")
    && validStage(journal.stage)
    && validOptionalTimestamp(journal.testBaselineAt)
    && validVersion(journal.baselineConfigurationVersion)
    && validVersion(journal.configurationVersion)
    && validVersion(journal.preferencesBaselineVersion)
    && validJournalStageState(journal);
  if (!valid) {
    return null;
  }
  return {
    apiFormat: journal.apiFormat as ProviderApiFormat,
    baselineConfigurationVersion: journal.baselineConfigurationVersion as number | null,
    configurationFingerprint: journal.configurationFingerprint as string,
    configurationVersion: journal.configurationVersion as number | null,
    credentialMode: journal.credentialMode as "preserve" | "replace",
    model: journal.model as string,
    outcome: journal.outcome as "ready" | "unknown",
    ownerScope: journal.ownerScope as string,
    preferencesBaselineVersion: journal.preferencesBaselineVersion as number | null,
    presetKey: journal.presetKey as string,
    providerDisplayName: journal.providerDisplayName as string,
    providerKey: journal.providerKey as string,
    providerWasExisting: journal.providerWasExisting as boolean,
    stage: journal.stage as ProviderSetupJournalStage,
    testBaselineAt: journal.testBaselineAt as string | null,
    version: 1,
  };
}

function validJournalStageState(
  journal: Partial<ProviderSetupJournal>,
): boolean {
  if (journal.providerWasExisting) {
    if (!validVersion(journal.baselineConfigurationVersion)
      || journal.baselineConfigurationVersion === null) {
      return false;
    }
  } else if (
    journal.baselineConfigurationVersion !== null
    || journal.credentialMode !== "replace"
  ) {
    return false;
  }
  if (journal.stage === "persist") {
    return journal.configurationVersion === null
      && journal.preferencesBaselineVersion === null;
  }
  if (journal.configurationVersion === null) {
    return false;
  }
  if (journal.stage === "test" && journal.preferencesBaselineVersion !== null) {
    return false;
  }
  if (
    journal.stage === "default"
    && journal.outcome === "unknown"
    && journal.preferencesBaselineVersion === null
  ) {
    return false;
  }
  return journal.stage === "test" || Boolean(journal.model?.trim());
}

function validString(value: unknown, allowEmpty = false): value is string {
  return typeof value === "string"
    && value.length <= MAX_JOURNAL_STRING_LENGTH
    && (allowEmpty || value.trim().length > 0);
}

function validVersion(value: unknown): value is number | null {
  return value === null
    || typeof value === "number" && Number.isSafeInteger(value) && value > 0;
}

function validFingerprint(value: unknown): value is string {
  return typeof value === "string" && /^[a-f0-9]{64}$/.test(value);
}

function validOptionalTimestamp(value: unknown): value is string | null {
  return value === null
    || typeof value === "string"
      && value.length > 0
      && value.length <= MAX_JOURNAL_STRING_LENGTH;
}

function testTimestampAdvanced(
  baseline: string | null,
  current: string | null | undefined,
): boolean {
  const normalizedCurrent = current?.trim() ?? "";
  if (!normalizedCurrent) {
    return false;
  }
  return normalizedCurrent !== (baseline?.trim() ?? "");
}

function validStage(value: unknown): value is ProviderSetupJournalStage {
  return value === "persist"
    || value === "test"
    || value === "default"
    || value === "complete";
}

function validAPIFormat(value: unknown): value is ProviderApiFormat {
  return value === "anthropic_messages"
    || value === "chat_completions"
    || value === "responses";
}
