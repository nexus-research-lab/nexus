// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

for (const heartbeatTimeout of [false, true]) {
test(heartbeatTimeout ? "heartbeat timeout reconnects without waiting for a close event" : "clean server close reconnects the shared WebSocket", async () => {
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
    const { WebSocketClient } = await import("@/lib/websocket/socket-client.ts");
    const states = [];
    const client = new WebSocketClient(
      {
        heartbeatInterval: heartbeatTimeout ? 10 : 0,
        heartbeatTimeout: 10,
        reconnect: true,
        reconnectDelay: 1,
        url: "ws://127.0.0.1/test",
      },
      { onStateChange: (state) => states.push(state) },
    );

    client.connect();
    sockets[0].open();
    if (!heartbeatTimeout) {
      sockets[0].serverClose();
      assert.equal(states.at(-1), "reconnecting");
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
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
}

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
    const { SharedWebSocketRegistry } = await import("@/lib/websocket/shared-socket-channel.ts");
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

test("exhausted retries keep probing until recovery and disconnect cancels probes", async () => {
  const originalWebSocket = globalThis.WebSocket;
  const originalWindow = globalThis.window;
  const sockets = [];
  const timers = new Map();
  let nextTimer = 0;
  class FakeWebSocket {
    static OPEN = 1;
    constructor() { sockets.push(this); }
    close() {}
    send() {}
  }
  globalThis.WebSocket = FakeWebSocket;
  globalThis.window = {
    dispatchEvent() {},
    setTimeout(callback, delay) { timers.set(++nextTimer, { callback, delay }); return nextTimer; },
    clearTimeout(id) { timers.delete(id); },
  };
  try {
    const { WebSocketClient } = await import("@/lib/websocket/socket-client.ts");
    const states = [];
    const client = new WebSocketClient({ url: "ws://localhost/test", heartbeatInterval: 0, maxReconnectAttempts: 1, reconnectDelay: 1 }, { onStateChange: state => states.push(state) });
    const fire = () => {
      const [id, timer] = timers.entries().next().value;
      timers.delete(id);
      timer.callback();
    };
    client.connect();
    sockets.at(-1).onclose({ code: 1006 });
    fire();
    sockets.at(-1).onclose({ code: 1006 });
    assert.equal(states.at(-1), "failed");
    assert.equal(timers.values().next().value.delay, 30_000);
    fire();
    sockets.at(-1).onclose({ code: 1006 });
    assert.equal(timers.values().next().value.delay, 30_000);
    fire();
    sockets.at(-1).readyState = FakeWebSocket.OPEN;
    sockets.at(-1).onopen();
    assert.equal(states.at(-1), "connected");
    sockets.at(-1).onclose({ code: 1006 });
    client.disconnect();
    assert.equal(timers.size, 0);
    client.connect();
    sockets.at(-1).onclose({ code: 4401 });
    assert.equal(timers.size, 0, "authentication failures must not probe");
    client.disconnect();
  } finally {
    globalThis.WebSocket = originalWebSocket;
    globalThis.window = originalWindow;
  }
});
