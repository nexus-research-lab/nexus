import assert from "node:assert/strict";
import { webcrypto } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { importLeafTypeScriptModule } from "./import-leaf-typescript-module.mjs";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

if (!globalThis.crypto) {
  globalThis.crypto = webcrypto;
}

const recovery = await importLeafTypeScriptModule(
  webRoot,
  "src/features/onboarding/provider-setup/provider-setup-recovery.ts",
);

test("create reconciliation requires exact key, version, and non-secret intent fingerprint", async () => {
  const journal = await providerJournal({ providerWasExisting: false });
  const matching = providerRecord({
    configuration_version: 1,
    provider: journal.providerKey,
  });
  assert.equal(
    (await recovery.reconcileProviderPersist(journal, [matching])).kind,
    "applied",
  );
  assert.equal(
    (await recovery.reconcileProviderPersist(journal, [
      { ...matching, provider: "same-name-different-key" },
    ])).kind,
    "unproven",
  );
  assert.equal(
    (await recovery.reconcileProviderPersist(journal, [
      { ...matching, base_url: "https://other.example/v1" },
    ])).kind,
    "unproven",
  );
  assert.equal(
    (await recovery.reconcileProviderPersist(journal, [
      { ...matching, configuration_version: 2 },
    ])).kind,
    "unproven",
  );
});

test("existing Provider reconciliation respects CAS and cannot prove a lost secret rotation", async () => {
  const preserved = await providerJournal({
    baselineConfigurationVersion: 7,
    credentialMode: "preserve",
    providerWasExisting: true,
  });
  const matching = providerRecord({
    configuration_version: 8,
    provider: preserved.providerKey,
  });
  assert.equal(
    (await recovery.reconcileProviderPersist(preserved, [matching])).kind,
    "applied",
  );
  assert.equal(
    (await recovery.reconcileProviderPersist(preserved, [
      { ...matching, configuration_version: 9 },
    ])).kind,
    "unproven",
  );

  const rotated = { ...preserved, credentialMode: "replace" };
  assert.equal(
    (await recovery.reconcileProviderPersist(rotated, [matching])).kind,
    "unproven",
  );
  assert.equal(
    (await recovery.reconcileProviderPersist(rotated, [matching], true)).kind,
    "applied",
    "a server-confirmed committed effect can prove the otherwise unreadable secret change",
  );
});

test("test reconciliation requires a new authoritative test timestamp and exact next version", async () => {
  const journal = await providerJournal({
    configurationVersion: 4,
    stage: "test",
    testBaselineAt: "2026-08-28T00:00:00Z",
  });
  const staleSuccess = providerRecord({
    configuration_version: 5,
    last_test_at: journal.testBaselineAt,
    last_test_status: "success",
    provider: journal.providerKey,
  });
  assert.equal(
    recovery.reconcileProviderTest(journal, [staleSuccess]).kind,
    "unproven",
    "an ordinary version advance cannot reuse an old success result",
  );
  assert.equal(
    recovery.reconcileProviderTest(journal, [{
      ...staleSuccess,
      last_test_at: "2026-08-28T00:00:01Z",
    }]).kind,
    "passed",
  );
  assert.equal(
    recovery.reconcileProviderTest(journal, [{
      ...staleSuccess,
      configuration_version: 6,
      last_test_at: "2026-08-28T00:00:01Z",
    }]).kind,
    "unproven",
  );
});

test("journal payload contains no credential, endpoint, request body, or transport identity", async () => {
  const source = await read(
    "src/features/onboarding/provider-setup/provider-setup-recovery.ts",
  );
  const journalShape = source.match(
    /export interface ProviderSetupJournal \{([\s\S]*?)\n\}/,
  )?.[1] ?? "";
  assert.doesNotMatch(
    journalShape,
    /apiKey|authToken|baseURL|base_url|requestBody|requestId|transportRequestId|httpId/i,
  );

  const storage = memoryStorage();
  globalThis.window = { localStorage: storage };
  const journal = await providerJournal();
  assert.equal(recovery.writeProviderSetupJournal(journal), true);
  const serialized = storage.values().join("\n");
  assert.doesNotMatch(
    serialized,
    /secret-value|https:\/\/api\.example|raw_body|transport_request_id|http_id/i,
  );

  Object.defineProperty(globalThis.window, "localStorage", {
    configurable: true,
    get() {
      throw new Error("storage unavailable");
    },
  });
  assert.equal(
    recovery.writeProviderSetupJournal(journal),
    false,
    "unavailable recovery storage must fail closed before a mutation",
  );
  delete globalThis.window;
});

test("the dialog has only staged save, test, and default recovery paths", async () => {
  const dialog = await read(
    "src/features/onboarding/provider-setup/provider-setup-dialog.tsx",
  );
  assert.match(dialog, /activeJournal\.stage === "persist"/);
  assert.match(dialog, /activeJournal\.stage === "test"/);
  assert.match(dialog, /activeJournal\.stage === "default"/);
  assert.match(dialog, /storeJournalBeforeEffect\(inFlight/);
  assert.match(dialog, /expectedVersion: activeJournal\.baselineConfigurationVersion/);
  assert.match(dialog, /expectedVersion: activeJournal\.configurationVersion/);
  assert.match(dialog, /subscribeAuthOwnerScopeGeneration/);
  assert.doesNotMatch(dialog, /function persistAndTest|persistAndTest\(/);
  assert.doesNotMatch(dialog, /testResult\.error|last_test_error/);
});

async function providerJournal(overrides = {}) {
  const fingerprint = await recovery.fingerprintProviderSetup({
    apiFormat: "chat_completions",
    baseURL: "https://api.example.test/v1",
    displayName: "Example",
    enabled: true,
    modelsPath: "/models",
    presetKey: "custom",
    providerKind: "llm",
  });
  return {
    apiFormat: "chat_completions",
    baselineConfigurationVersion: null,
    configurationFingerprint: fingerprint,
    configurationVersion: null,
    credentialMode: "replace",
    model: "model-a",
    outcome: "unknown",
    ownerScope: "user-id:owner-a",
    preferencesBaselineVersion: null,
    presetKey: "custom",
    providerDisplayName: "Example",
    providerKey: "custom-exact-key",
    providerWasExisting: false,
    stage: "persist",
    testBaselineAt: null,
    version: 1,
    ...overrides,
  };
}

function providerRecord(overrides = {}) {
  return {
    agent_runtime_supported: true,
    api_format: "chat_completions",
    auth_token_masked: "***",
    base_url: "https://api.example.test/v1",
    can_manage: true,
    configuration_version: 1,
    display_name: "Example",
    enabled: true,
    id: "provider-id",
    last_test_at: null,
    last_test_error: "",
    last_test_status: "",
    models: [{
      capabilities_auto: {},
      capabilities_override: {},
      category: "chat",
      display_name: "model-a",
      enabled: true,
      id: "model-id",
      is_default: true,
      model_id: "model-a",
      provider_id: "provider-id",
      provider_options: {},
    }],
    models_path: "/models",
    owner_user_id: "owner-a",
    preset_key: "custom",
    provider: "custom-exact-key",
    provider_kind: "llm",
    usage_count: 0,
    used_by_agents: [],
    visibility: "private",
    ...overrides,
  };
}

function memoryStorage() {
  const data = new Map();
  return {
    clear: () => data.clear(),
    getItem: (key) => data.get(key) ?? null,
    key: (index) => [...data.keys()][index] ?? null,
    get length() {
      return data.size;
    },
    removeItem: (key) => data.delete(key),
    setItem: (key, value) => data.set(key, String(value)),
    values: () => [...data.values()],
  };
}

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
