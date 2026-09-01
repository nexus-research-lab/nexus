import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("Channel mutation copy uses machine evidence and never raw error text", async () => {
  const { buildChannelOperationIssue } = await server.ssrLoadModule(
    "/src/features/capability/channels/channel-operation-recovery.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const { enMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/en/index.ts",
  );

  const unknown = buildChannelOperationIssue(
    new ApiTransportError(
      "provider failed token=raw-secret request_id=internal",
      "network",
      "unknown",
    ),
    "channel_delete",
    translator(zhMessages),
  );
  assert.equal(unknown.effect, "unknown");
  assert.match(unknown.title, /无法确认/);
  assert.match(unknown.impact, /配置、账号和配对状态待核对/);
  assert.match(unknown.impact, /其他频道不受影响/);
  assert.match(unknown.nextStep, /不要重复/);
  assert.doesNotMatch(unknown.impact, /可能[^。；]*也可能/);
  assert.doesNotMatch(JSON.stringify(unknown), /raw-secret|request_id|provider failed/);

  const rejected = buildChannelOperationIssue(
    new ApiRequestError("SQL conflict row=private", 409, {
      version: 1,
      code: "channel.pairing_conflict",
      category: "conflict",
      effect: "not_applied",
    }),
    "pairing_update",
    translator(enMessages),
  );
  assert.equal(rejected.effect, "not_applied");
  assert.match(rejected.title, /did not finish/);
  assert.match(rejected.impact, /remain unchanged/);
  assert.match(rejected.nextStep, /try again/);
  assert.doesNotMatch(JSON.stringify(rejected), /SQL|private|pairing_conflict/);
});

test("Pairing reconciliation proves only an observable requested end state", async () => {
  const { reconcilePairingIntent } = await server.ssrLoadModule(
    "/src/features/capability/channels/pairings/pairing-recovery.ts",
  );
  const base = pairing({ pairing_id: "pair-1", status: "pending" });

  assert.equal(reconcilePairingIntent({
    kind: "update",
    pairingId: "pair-1",
    patch: { status: "active" },
  }, [base]), "unproven");
  assert.equal(reconcilePairingIntent({
    kind: "update",
    pairingId: "pair-1",
    patch: { status: "active" },
  }, [{ ...base, status: "active" }]), "applied");
  assert.equal(reconcilePairingIntent({
    kind: "delete",
    pairingId: "pair-1",
  }, []), "applied");

  const createIntent = {
    kind: "create",
    payload: {
      agent_id: "agent-1",
      channel_type: "feishu",
      chat_type: "dm",
      external_name: "客户 A",
      external_ref: "external-a",
      status: "active",
    },
  };
  assert.equal(reconcilePairingIntent(createIntent, [base]), "unproven");
  assert.equal(reconcilePairingIntent(createIntent, [pairing({
    agent_id: "agent-1",
    external_name: "客户 A",
    external_ref: "external-a",
    status: "active",
  })]), "applied");
});

test("Channel reconciliation does not pretend an opaque secret rotation is proven", async () => {
  const { reconcileChannelConnectionIntent } = await server.ssrLoadModule(
    "/src/features/capability/channels/connection/channel-connection-recovery.ts",
  );
  const channel = channelView({ has_credentials: true });
  const saveIntent = {
    agentId: "agent-1",
    baseHadCredentials: true,
    channelType: "telegram",
    kind: "save",
    publicConfig: { region: "global" },
    wroteSecrets: true,
  };
  assert.equal(
    reconcileChannelConnectionIntent(saveIntent, [channel]),
    "unproven",
  );
  assert.equal(
    reconcileChannelConnectionIntent({
      ...saveIntent,
      baseHadCredentials: false,
    }, [channel]),
    "applied",
  );
  assert.equal(reconcileChannelConnectionIntent({
    channelType: "telegram",
    kind: "delete-channel",
  }, [{ ...channel, configured: false }]), "applied");
  assert.equal(reconcileChannelConnectionIntent({
    accountId: "account-a",
    channelType: "telegram",
    kind: "delete-account",
  }, [{ ...channel, accounts: [] }]), "applied");
});

test("Channel login model hides raw provider diagnostics and unknown states", async () => {
  const { buildChannelLoginPanelModel } = await server.ssrLoadModule(
    "/src/features/capability/channels/connection/login/channel-login-model.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const rawProviderHint = "provider says request_id=secret-internal";
  const unknownStatus = "provider_future_state";
  const unknown = buildChannelLoginPanelModel({
    account_id: "",
    login_id: "internal-login-id",
    qr_payload: "opaque-login-token",
    status: unknownStatus,
    user_id: "",
    verify_code_hint: rawProviderHint,
  }, translator(zhMessages));
  assert.equal(unknown.status.label, zhMessages["capability.channel_login_status_pending"]);
  assert.doesNotMatch(JSON.stringify(unknown.status), new RegExp(unknownStatus));
  assert.equal(
    unknown.failure.title,
    zhMessages["capability.channel_login_unknown_status_title"],
  );
  assert.ok(unknown.failure.impact);
  assert.ok(unknown.failure.nextStep);
  assert.doesNotMatch(JSON.stringify(unknown.failure), new RegExp(unknownStatus));

  const verify = buildChannelLoginPanelModel({
    account_id: "",
    login_id: "internal-login-id",
    qr_payload: "opaque-login-token",
    status: "verify_code_required",
    user_id: "",
    verify_code_hint: rawProviderHint,
  }, translator(zhMessages));
  assert.equal(
    verify.verifyCodeHint,
    zhMessages["capability.channel_login_verify_code_hint"],
  );
  assert.doesNotMatch(verify.verifyCodeHint, /provider|request_id|secret-internal/);
});

function translator(messages) {
  return (key, params = {}) => Object.entries(params).reduce(
    (value, [name, replacement]) => value.replaceAll(
      `{${name}}`,
      String(replacement),
    ),
    messages[key] ?? key,
  );
}

function pairing(overrides = {}) {
  return {
    account_id: "",
    agent_id: "agent-1",
    channel_type: "feishu",
    chat_type: "dm",
    created_at: "2026-01-01T00:00:00Z",
    external_name: "",
    external_ref: "external-b",
    pairing_id: "pair-2",
    session_key: "session-hidden",
    source: "manual",
    status: "pending",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function channelView(overrides = {}) {
  return {
    accounts: [{ account_id: "account-a", status: "connected" }],
    agent_id: "agent-1",
    bot_label: "Bot",
    capabilities: [],
    channel_type: "telegram",
    configured: true,
    connection_state: "connected",
    credential_fields: [],
    description: "Telegram",
    has_credentials: false,
    public_config: { region: "global" },
    runtime_status: "ready",
    stats: { paired_group_count: 0, paired_user_count: 0, pending_count: 0 },
    status: "connected",
    supports_group: true,
    supports_oauth_link: false,
    supports_qr_code: false,
    title: "Telegram",
    ...overrides,
  };
}
