import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
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
  assert.match(unknown.impact, /无法确认频道是否已断开/);
  assert.match(unknown.impact, /刷新频道信息/);
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
  assert.match(rejected.impact, /wasn't updated/);
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

test("Channel controllers preserve snapshots, lock unknown writes, and reconcile with reads", async () => {
  const [catalog, pairings, connection, login, createDialog, channelApi] = await Promise.all([
    read("src/features/capability/channels/catalog/use-channels-controller.ts"),
    read("src/features/capability/channels/pairings/use-pairings-controller.ts"),
    read("src/features/capability/channels/connection/use-channel-connection-controller.ts"),
    read("src/features/capability/channels/connection/login/use-channel-login-controller.ts"),
    read("src/features/capability/channels/pairings/pairing-create-dialog.tsx"),
    read("src/lib/api/capability/channel-api.ts"),
  ]);

  assert.match(catalog, /Promise\.allSettled/);
  assert.match(catalog, /setChannels\(channelResult\.value\)/);
  assert.doesNotMatch(catalog, /error instanceof Error|error\.message|getErrorMessage/);

  assert.match(pairings, /Promise\.allSettled/);
  assert.match(pairings, /setRecovery\(\{ check: "not_checked", intent, issue \}\)/);
  assert.match(pairings, /const latest = await listPairingsApi\(\)/);
  assert.match(pairings, /reconcilePairingIntent\(recovery\.intent, latest\)/);
  assert.match(pairings, /busy: pendingAction !== null \|\| recovery !== null/);
  assert.doesNotMatch(pairings, /getErrorMessage|error\.message/);
  assert.doesNotMatch(createDialog, /createPairingApi|getErrorMessage|error\.message/);

  assert.match(connection, /const items = await listChannelsApi\(\)/);
  assert.match(connection, /reconcileChannelConnectionIntent/);
  assert.match(connection, /wroteSecrets: Object\.values\(draft\.credentials\)/);
  const connectionRecovery = await read(
    "src/features/capability/channels/connection/channel-connection-recovery.ts",
  );
  assert.doesNotMatch(connectionRecovery, /draft:|credentials: Record|secretValue/);
  assert.match(connection, /if \(!closeBlocked\)[\s\S]*onClose\(\)/);
  assert.doesNotMatch(connection, /getErrorMessage|error\.message/);

  assert.match(login, /const nextView = await getChannelLoginApi/);
  assert.match(login, /nextView\.status !== "verify_code_required"/);
  assert.match(login, /const reconcileLoginStart = useCallback/);
  assert.match(login, /await getCurrentChannelLoginApi\(channelType\)/);
  assert.match(login, /error\.status === 404 \|\| error\.status === 409/);
  assert.match(login, /refreshed \? "unproven" : "failed"/);
  assert.match(login, /startCanRetry[\s\S]*recovery\.issue\.effect === "not_applied"/);
  assert.match(login, /if \(startCanRetry\)[\s\S]*void startLogin\(\)/);
  assert.match(login, /else if \(recovery\.kind === "start"\)[\s\S]*void reconcileLoginStart\(\)/);
  const startReconciliation = login.slice(
    login.indexOf("const reconcileLoginStart"),
    login.indexOf("const submitVerifyCode"),
  );
  assert.doesNotMatch(startReconciliation, /startChannelLoginApi|startLogin\(/);
  assert.doesNotMatch(login, /getErrorMessage|error\.message/);

  assert.match(channelApi, /export async function getCurrentChannelLoginApi/);
  assert.match(
    channelApi,
    /getCurrentChannelLoginApi[\s\S]*\/login`[\s\S]*method: "GET"/,
  );
});

test("Channel login and account views hide raw provider diagnostics and QR payload text", async () => {
  const [loginModel, loginPanel, qrCode, sharedQrCode, accounts, deleteCopy, fields, card, connectionDialog, catalogModel] = await Promise.all([
    read("src/features/capability/channels/connection/login/channel-login-model.ts"),
    read("src/features/capability/channels/connection/login/channel-login-panel.tsx"),
    read("src/features/capability/channels/connection/login/login-qr-code.tsx"),
    read("src/shared/ui/display/qr-code.tsx"),
    read("src/features/capability/channels/connection/channel-accounts-panel.tsx"),
    read("src/features/capability/channels/connection/view/channel-connect-dialog-model.ts"),
    read("src/features/capability/channels/connection/view/channel-connection-fields.tsx"),
    read("src/features/capability/channels/catalog/channel-card.tsx"),
    read("src/features/capability/channels/connection/channel-connect-dialog.tsx"),
    read("src/features/capability/channels/catalog/channel-catalog-model.ts"),
  ]);

  assert.doesNotMatch(loginModel, /view\.error|view\.output|view\.command/);
  assert.doesNotMatch(loginModel, /view\.verify_code_hint|label: status/);
  assert.doesNotMatch(loginPanel, /model\.error|model\.output/);
  assert.match(qrCode, /showPayload=\{false\}/);
  assert.match(qrCode, /channel_login_qr_missing_impact/);
  assert.match(qrCode, /channel_login_qr_missing_next_step/);
  assert.match(qrCode, /channel_login_qr_failed_impact/);
  assert.match(qrCode, /channel_login_qr_failed_next_step/);
  assert.match(sharedQrCode, /\? "loading" : "idle"/);
  assert.match(sharedQrCode, /generation\.status === "loading"/);
  assert.match(sharedQrCode, /generation\.status === "ready"/);
  assert.doesNotMatch(accounts, /title=\{account\.last_error\}|\{account\.last_error\}/);
  assert.match(accounts, /channel_account_error_message/);
  assert.match(accounts, /channel_account_error_impact/);
  assert.doesNotMatch(fields, /window\.open/);
  assert.match(fields, /rel="noopener noreferrer"/);
  assert.doesNotMatch(card, /window\.open/);
  assert.match(card, /rel="noopener noreferrer"/);
  assert.match(connectionDialog, /channel_connection_error_impact/);
  assert.doesNotMatch(connectionDialog, /\{controller\.currentItem\.last_error\}/);
  assert.match(catalogModel, /Boolean\(item\.last_error\)/);
  assert.match(catalogModel, /channel_connection_error_badge/);
  assert.match(deleteCopy, /配置、已连接账号和配对/);
  assert.match(deleteCopy, /账号及使用它的配对会被删除/);

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

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
