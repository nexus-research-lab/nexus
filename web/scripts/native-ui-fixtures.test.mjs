// INPUT: The native fixture server, a local upstream sentinel and real HTTP/WS clients.
// OUTPUT: Evidence that App reads are served locally and mutations cannot reach an upstream.
// POS: Native QA transport contract; never connects to a running product service.

import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createServer } from "node:http";
import test from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { fileURLToPath } from "node:url";
import WebSocket from "ws";

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return server.address().port;
}

test("native App fixtures reject writes and never forward HTTP or WebSocket to a backend", { timeout: 20_000 }, async (t) => {
  let forwarded = 0;
  const upstream = createServer((_request, response) => { forwarded += 1; response.end("unexpected upstream"); });
  upstream.on("upgrade", (_request, socket) => { forwarded += 1; socket.destroy(); });
  const upstreamPort = await listen(upstream);
  const probe = createServer();
  const port = await listen(probe);
  await new Promise((resolve) => probe.close(resolve));
  const child = spawn(process.execPath, ["browser-tests/native-ui-server.mjs"], {
    cwd: fileURLToPath(new URL("..", import.meta.url)),
    env: { ...process.env, NEXUS_UI_TEST_PORT: String(port), NEXUS_UI_TEST_SURFACE: "app-shell",
      VITE_BACKEND_PORT: String(upstreamPort), VITE_CONTROL_PORT: String(upstreamPort) },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let log = "";
  child.stdout.on("data", (data) => { log += data; });
  child.stderr.on("data", (data) => { log += data; });
  t.after(async () => {
    if (child.exitCode === null) {
      const closed = once(child, "exit");
      child.kill("SIGTERM");
      const timer = setTimeout(() => child.kill("SIGKILL"), 5_000);
      await closed;
      clearTimeout(timer);
    }
    await new Promise((resolve) => upstream.close(resolve));
  });
  const origin = `http://127.0.0.1:${port}`;
  let ready = false;
  for (let attempt = 0; attempt < 80; attempt += 1) {
    assert.equal(child.exitCode, null, log);
    try { ready = (await fetch(`${origin}/qa/health`)).ok; } catch { /* Server is still starting. */ }
    if (ready) break;
    await delay(100);
  }
  assert.ok(ready, log);
  const snapshot = await (await fetch(`${origin}/nexus/v1/launcher/bootstrap`)).json();
  assert.equal(snapshot.data.agents[0].id, "qa-main");
  for (const [method, path] of [["POST", "/nexus/v1/launcher/bootstrap"], ["POST", "/auth/v1/logout"], ["GET", "/nexus/v1/not-a-fixture"]]) {
    assert.equal((await fetch(origin + path, { method })).status, 503);
  }
  const socket = new WebSocket(`ws://127.0.0.1:${port}/nexus/v1/chat/ws`);
  t.after(() => socket.terminate());
  await once(socket, "open");
  socket.send(JSON.stringify({ type: "subscribe_app_events" }));
  const pong = once(socket, "message");
  socket.send(JSON.stringify({ type: "ping" }));
  assert.deepEqual(JSON.parse((await pong)[0].toString()), { event_type: "pong" });
  const closed = once(socket, "close");
  socket.send(JSON.stringify({ type: "send_message" }));
  assert.equal((await closed)[0], 1008);
  for (const malformed of ["null", "{"]) {
    const invalid = new WebSocket(`ws://127.0.0.1:${port}/nexus/v1/chat/ws`);
    t.after(() => invalid.terminate());
    await once(invalid, "open");
    const rejected = once(invalid, "close");
    invalid.send(malformed);
    assert.equal((await rejected)[0], 1008);
  }
  const health = await (await fetch(`${origin}/qa/health`)).json();
  assert.equal(health.socketConnections, 3);
  assert.deepEqual(health.socketMessages, ["subscribe_app_events", "ping"]);
  assert.equal(health.rejected.length, 6);
  assert.equal(forwarded, 0, "Vite's development proxy must not reach the local upstream sentinel");
});
