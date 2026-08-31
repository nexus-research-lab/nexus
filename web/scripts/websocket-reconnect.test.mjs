import assert from "node:assert/strict";
import fs from "node:fs/promises";
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

test("clean server close reconnects the shared WebSocket", async () => {
  const sockets = [];
  const originalWebSocket = globalThis.WebSocket;
  const originalWindow = globalThis.window;

  class FakeWebSocket {
    static OPEN = 1;

    constructor() {
      this.readyState = 0;
      sockets.push(this);
    }

    open() {
      this.readyState = FakeWebSocket.OPEN;
      this.onopen?.();
    }

    serverClose() {
      this.readyState = 3;
      this.onclose?.({ code: 1000, reason: "idle timeout", wasClean: true });
    }

    close() {
      this.readyState = 3;
    }

    send() {}
  }

  globalThis.WebSocket = FakeWebSocket;
  globalThis.window = globalThis;
  try {
    const { WebSocketClient } = await server.ssrLoadModule(
      "/src/lib/websocket/socket-client.ts",
    );
    const states = [];
    const client = new WebSocketClient(
      {
        heartbeatInterval: 0,
        reconnect: true,
        reconnectDelay: 1,
        url: "ws://127.0.0.1/test",
      },
      { onStateChange: (state) => states.push(state) },
    );

    client.connect();
    sockets[0].open();
    sockets[0].serverClose();

    assert.equal(states.at(-1), "reconnecting");
    await new Promise((resolve) => setTimeout(resolve, 10));
    assert.equal(sockets.length, 2);
    sockets[1].open();
    assert.equal(states.at(-1), "connected");
    client.disconnect();
  } finally {
    globalThis.WebSocket = originalWebSocket;
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("owner reset disposes old channels and the same key creates a fresh handshake", async () => {
  const sockets = [];
  const originalWebSocket = globalThis.WebSocket;
  const originalWindow = globalThis.window;

  class FakeWebSocket {
    static OPEN = 1;

    constructor() {
      this.readyState = 0;
      sockets.push(this);
    }

    open() {
      this.readyState = FakeWebSocket.OPEN;
      this.onopen?.();
    }

    close() {
      this.readyState = 3;
    }

    send() {}
  }

  globalThis.WebSocket = FakeWebSocket;
  globalThis.window = globalThis;
  try {
    const { SharedWebSocketRegistry } = await server.ssrLoadModule(
      "/src/lib/websocket/shared-socket-channel.ts",
    );
    const registry = new SharedWebSocketRegistry();
    const config = {
      heartbeatInterval: 0,
      heartbeatTimeout: 0,
      maxReconnectAttempts: 1,
      maxReconnectDelay: 1,
      protocols: [],
      reconnect: false,
      reconnectDelay: 1,
      url: "ws://127.0.0.1/owner-fence",
    };
    const key = "owner-fence-key";
    let requestCallbackCalled = false;
    const oldChannel = registry.acquire(key, config);
    const oldSubscriberId = oldChannel.subscribe({
      setError: () => {},
      setState: () => {},
    });
    oldChannel.acquireSessionBinding({}, {
      type: "bind_session",
      session_key: "owner-a-session",
    });
    oldChannel.acquireRequestTransportLease({
      clientRequestId: "owner-a-request",
      onAccepted: () => {
        requestCallbackCalled = true;
      },
      onRejected: () => {
        requestCallbackCalled = true;
      },
      sessionBinding: {
        type: "bind_session",
        session_key: "owner-a-session",
      },
    });
    oldChannel.connect();
    sockets[0].open();
    assert.equal(oldChannel.hasConsumers(), true);

    registry.resetOwnerScope();
    assert.equal(oldChannel.hasConsumers(), false);
    assert.equal(requestCallbackCalled, false);
    assert.equal(sockets[0].readyState, 3);

    const newChannel = registry.acquire(key, config);
    assert.notStrictEqual(newChannel, oldChannel);
    newChannel.connect();
    sockets[1].open();
    assert.equal(sockets.length, 2);

    // 旧 React cleanup 迟到时只能处理旧对象，不能移除同 key 的新通道。
    oldChannel.unsubscribe(oldSubscriberId);
    registry.release(key, oldChannel);
    await new Promise((resolve) => setTimeout(resolve, 320));
    assert.strictEqual(registry.acquire(key, config), newChannel);
    registry.resetOwnerScope();
  } finally {
    globalThis.WebSocket = originalWebSocket;
    if (originalWindow === undefined) {
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
  }
});

test("home directory uses shared event-driven refresh without fixed polling", async () => {
  const [directoryResource, launcherPage, notificationSocket] = await Promise.all([
    fs.readFile(
      path.join(webRoot, "src/features/home/home-directory-resource.ts"),
      "utf8",
    ),
    fs.readFile(
      path.join(webRoot, "src/pages/launcher/launcher-page.tsx"),
      "utf8",
    ),
    fs.readFile(
      path.join(
        webRoot,
        "src/features/home/notifications/use-chat-notification-socket.ts",
      ),
      "utf8",
    ),
  ]);

  assert.match(directoryResource, /function refreshHomeDirectoryIfStale/);
  assert.match(directoryResource, /subscribeRoomDirectoryUpdates\(refreshHomeDirectory\)/);
  assert.doesNotMatch(directoryResource, /setInterval/);
  assert.match(launcherPage, /useHomeDirectory\(\)/);
  assert.doesNotMatch(
    launcherPage,
    /getLauncherBootstrapApi|subscribeRoomDirectoryUpdates/,
  );
  assert.match(notificationSocket, /if \(hasConnectedRef\.current\)/);
  assert.match(notificationSocket, /notifyRoomDirectoryUpdated\(\)/);
});
