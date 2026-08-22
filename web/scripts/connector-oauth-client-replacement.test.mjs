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

test("Feishu Docs replace-app action only appears for a persisted OAuth client", async () => {
  const { canReplaceConnectorOauthClient } = await server.ssrLoadModule(
    "/src/features/capability/connectors/detail/connector-detail-model.ts",
  );
  const detail = {
    connector_id: "feishu-docx",
    oauth_client_id: "persisted-client",
  };

  assert.equal(canReplaceConnectorOauthClient(detail), true);
  assert.equal(
    canReplaceConnectorOauthClient({ ...detail, oauth_client_id: null }),
    false,
  );
  assert.equal(
    canReplaceConnectorOauthClient({
      ...detail,
      connector_id: "github",
    }),
    false,
  );
});

test("Feishu Docs presents official QR as the primary choose-or-create flow", async () => {
  const {
    getFeishuDeviceAuthPresentation,
    shouldAutoOpenFeishuUserAuthorization,
  } = await server.ssrLoadModule(
    "/src/features/capability/connectors/auth/feishu/feishu-app-connection-model.ts",
  );

  const appSelection = getFeishuDeviceAuthPresentation("app_selection");
  assert.equal(appSelection.title, "选择飞书应用");
  assert.equal("subtitle" in appSelection, false);
  assert.equal(appSelection.showQRCode, true);
  assert.equal(appSelection.actionLabel, "打开飞书");

  const userAuthorization = getFeishuDeviceAuthPresentation(
    "user_authorization",
  );
  assert.equal(userAuthorization.title, "连接飞书云文档");
  assert.equal(userAuthorization.showQRCode, false);
  assert.equal(userAuthorization.actionLabel, "继续飞书授权");

  const appSelectionSession = {
    connector_id: "feishu-docx",
    device_code: "app-selection",
    user_code: "choose-app",
    verification_uri: "https://open.feishu.test/choose-app",
    expires_in: 600,
    interval: 1,
    stage: "app_selection",
  };
  const userAuthorizationSession = {
    ...appSelectionSession,
    device_code: "user-authorization",
    user_code: "authorize-user",
    verification_uri: "https://accounts.feishu.test/authorize-user",
    stage: "user_authorization",
  };
  assert.equal(
    shouldAutoOpenFeishuUserAuthorization(appSelectionSession),
    false,
  );
  assert.equal(
    shouldAutoOpenFeishuUserAuthorization(userAuthorizationSession),
    true,
  );
});

test("Feishu Docs opens a Web window only when user authorization begins", async () => {
  const { FeishuWebAuthorizationWindow } = await server.ssrLoadModule(
    "/src/features/capability/connectors/auth/feishu/feishu-web-authorization-window.ts",
  );
  let openCount = 0;
  let focusCount = 0;
  let closeCount = 0;
  let navigationCount = 0;
  let navigationScheduleCount = 0;
  let loadingRenderCount = 0;
  let currentHref = "";
  const popup = {
    closed: false,
    close: () => {
      closeCount += 1;
      popup.closed = true;
    },
    focus: () => {
      focusCount += 1;
    },
    location: {
      get href() {
        return currentHref;
      },
      set href(value) {
        navigationCount += 1;
        currentHref = value;
      },
    },
    opener: {},
  };
  const authorizationWindow = new FeishuWebAuthorizationWindow(
    () => {
      openCount += 1;
      return popup;
    },
    (navigate) => {
      navigationScheduleCount += 1;
      navigate();
    },
    () => {
      loadingRenderCount += 1;
    },
  );

  assert.equal(openCount, 0);
  assert.equal(
    authorizationWindow.open("https://accounts.feishu.test/authorize"),
    true,
  );
  assert.equal(openCount, 1);
  assert.equal(loadingRenderCount, 1);
  assert.equal(navigationScheduleCount, 1);
  assert.equal(popup.opener, null);
  assert.equal(currentHref, "https://accounts.feishu.test/authorize");
  assert.equal(navigationCount, 1);
  assert.equal(
    authorizationWindow.open("https://accounts.feishu.test/authorize"),
    true,
  );
  assert.equal(navigationCount, 1);
  assert.equal(navigationScheduleCount, 1);
  assert.equal(focusCount, 2);

  authorizationWindow.close();
  assert.equal(closeCount, 1);
});

test("Feishu Docs manual credentials are a complete-pair fallback", async () => {
  const { feishuManualCredentialsComplete } = await server.ssrLoadModule(
    "/src/features/capability/connectors/auth/feishu/feishu-app-connection-model.ts",
  );

  assert.equal(feishuManualCredentialsComplete("", ""), false);
  assert.equal(feishuManualCredentialsComplete("cli_a", ""), false);
  assert.equal(feishuManualCredentialsComplete("", "secret"), false);
  assert.equal(
    feishuManualCredentialsComplete(" cli_a ", " secret "),
    true,
  );
});

test("Device auth closes immediately when connection succeeds", async () => {
  const { ConnectorDeviceAuthPoller } = await server.ssrLoadModule(
    "/src/features/capability/connectors/auth/device-flow/connector-device-auth-poller.ts",
  );
  const events = [];
  let finishRefresh;
  const refresh = new Promise((resolve) => {
    finishRefresh = resolve;
  });
  const poller = new ConnectorDeviceAuthPoller(
    {
      connector_id: "feishu-docx",
      device_code: "device-code",
      user_code: "user-code",
      verification_uri: "https://accounts.feishu.test/device",
      expires_in: 600,
      interval: 1,
      stage: "user_authorization",
    },
    {
      onClose: () => events.push("closed"),
      onConnected: async () => {
        events.push("refresh-started");
        await refresh;
        events.push("refresh-finished");
      },
      onError: (message) => events.push(`error:${message}`),
      onMessage: () => events.push("connected-message"),
      onNext: () => events.push("next"),
    },
    async () => ({ status: "connected" }),
  );

  const polling = poller.poll();
  await Promise.resolve();
  assert.deepEqual(events, [
    "connected-message",
    "closed",
    "refresh-started",
  ]);

  finishRefresh();
  await polling;
  assert.deepEqual(events, [
    "connected-message",
    "closed",
    "refresh-started",
    "refresh-finished",
  ]);
});
