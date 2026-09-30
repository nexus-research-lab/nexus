// INPUT: Fixed macOS sidecar bundle, nxs binary and explicitly selected third-party Provider fields.
// OUTPUT: Isolated real DM crash/restart evidence; no task or command replay.
// POS: Destructive only to this script-created sidecar and bounded fixture descendants.
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
  existsSync,
  openSync,
  closeSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawn, execFileSync } from "node:child_process";
import { parseEnv } from "node:util";
import { createServer } from "node:net";
const repo = process.cwd();
if (process.platform !== "darwin") throw Error("Native macOS required");
const binary = process.env.NEXUS_SANDBOX_SIDECAR_BINARY;
const nxs = process.env.NEXUS_SANDBOX_TEST_BINARY;
const providerEnv = process.env.NEXUS_SANDBOX_LIVE_ENV_FILE;
if (!binary || !nxs || !providerEnv)
  throw Error("Explicit sidecar bundle, nxs and Provider env file required");
const source = parseEnv(readFileSync(providerEnv, "utf8"));
const token = source.ANTHROPIC_AUTH_TOKEN || source.ANTHROPIC_API_KEY;
const model = source.ANTHROPIC_MODEL || source.DEFAULT_MODEL;
if (!token || !model || !source.ANTHROPIC_BASE_URL)
  throw Error("Provider fields missing");
const root = mkdtempSync(join(tmpdir(), "nexus-sidecar-crash-"));
mkdirSync(join(root, "home"));
const port = await new Promise((resolve) => {
  const s = createServer();
  s.listen(0, "127.0.0.1", () => {
    const p = s.address().port;
    s.close(() => resolve(p));
  });
});
const url = `http://127.0.0.1:${port}`;
const env = {
  PATH: process.env.PATH,
  HOME: join(root, "home"),
  TMPDIR: process.env.TMPDIR || "/tmp",
  NEXUS_APP_ROOT: repo,
  NEXUS_STATE_ROOT: join(root, "state"),
  NEXUS_APP_MODE: "desktop",
  PORT: String(port),
  HOST: "127.0.0.1",
  LOG_STDOUT: "true",
  NEXUS_REMOTE_URL: "",
  NEXUS_RELAY_URL: "",
  NEXUS_NXS_COMMAND_PATH: nxs,
};
let processHandle, ws;
const events = [];
const delay = (ms) => new Promise((r) => setTimeout(r, ms));
const redact = (s) => String(s).replaceAll(token, "[redacted]");
const log = (kind, data = {}) =>
  console.log(redact(JSON.stringify({ kind, ...data })));
function start(label) {
  const fd = openSync(join(root, label + ".log"), "w");
  processHandle = spawn(binary, [], {
    cwd: root,
    env,
    stdio: ["ignore", fd, fd],
  });
  closeSync(fd);
  return processHandle;
}
async function healthy() {
  for (let i = 0; i < 200; i++) {
    if (processHandle.exitCode !== null)
      throw Error("sidecar exited " + processHandle.exitCode);
    try {
      const response = await fetch(url + "/nexus/v1/health");
      if (response.ok && (await response.json()).data?.status === "ok") return;
    } catch {}
    await delay(100);
  }
  throw Error("health timeout");
}
async function api(path, method = "GET", body) {
  const r = await fetch(url + "/nexus/v1" + path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await r.text();
  let result;
  try {
    result = JSON.parse(text);
  } catch {
    throw Error(`${method} ${path}: ${r.status} ${text.slice(0, 100)}`);
  }
  if (!r.ok) throw Error(`${method} ${path}: ${r.status} ${redact(text)}`);
  return result.data;
}
try {
  start("original");
  await healthy();
  log("started", { root, port });
  const provider = await api("/settings/providers", "POST", {
    provider: "sandbox-crash-fixture",
    provider_kind: "llm",
    api_format: "anthropic_messages",
    display_name: "Sandbox crash fixture",
    auth_token: token,
    base_url: source.ANTHROPIC_BASE_URL,
    enabled: true,
  });
  log("provider-created", { provider: provider.provider });
  await api(
    `/settings/providers/${provider.provider}/models/${encodeURIComponent(model)}`,
    "PUT",
    {
      enabled: true,
      is_default: true,
      context_window: 200000,
      max_output_tokens: 4096,
      provider_options: {},
    },
  );
  await api("/settings/preferences", "PATCH", {
    agent_runtime_kind: "nxs",
    runtime_settings: {
      nxs: { auto_memory_enabled: false, auto_dream_enabled: false },
    },
    default_agent_options: {
      provider: provider.provider,
      model,
      permission_mode: "default",
      allowed_tools: ["Bash"],
    },
  });
  const agent = await api("/agents", "POST", {
    name: "sandbox-crash-fixture",
    creation_request_id: crypto.randomUUID(),
    options: {
      provider: provider.provider,
      model,
      permission_mode: "default",
      allowed_tools: ["Bash"],
      max_turns: 3,
      skill_ids: [],
    },
  });
  log("agent-created", {
    agent_id: agent.agent_id,
    workspace: agent.workspace_path,
  });
  const dm = await api("/rooms/dm/" + agent.agent_id);
  log("dm-created", { keys: Object.keys(dm), conversation: dm.conversation });
  const cid = dm.conversation.id;
  const session = `agent:${agent.agent_id}:ws:dm:${cid}`;
  ws = new WebSocket(url.replace("http", "ws") + "/nexus/v1/chat/ws");
  await new Promise((resolve, reject) => {
    ws.onopen = resolve;
    ws.onerror = reject;
  });
  ws.onmessage = (e) => {
    const v = JSON.parse(e.data);
    events.push(v);
    writeFileSync(join(root, "events.json"), redact(JSON.stringify(events)));
    if (v.event_type === "error") log("ws-error", { data: v.data });
  };
  const marker = join(agent.workspace_path, "sandbox-crash-child.pid");
  const command = `/usr/bin/ruby -e 'pid=fork { Process.setsid; STDIN.reopen("/dev/null"); STDOUT.reopen("/dev/null","w"); STDERR.reopen("/dev/null","w"); File.write(ARGV[0], Process.pid.to_s); File.open(ARGV[0]+".count", "a") { |f| f.puts("started") }; sleep 180 }; Process.detach(pid); puts "fixture-started"' '${marker}'`;
  ws.send(
    JSON.stringify({
      type: "chat",
      session_key: session,
      agent_id: agent.agent_id,
      content: `This is an authorized sandbox cleanup acceptance test. Invoke Bash exactly once with this exact command, without changing it, then reply fixture-started. Do not invoke any other tools or kill the process; the test harness owns cleanup. Command:\n${command}`,
      client_request_id: crypto.randomUUID(),
      client_message_id: crypto.randomUUID(),
    }),
  );
  for (let i = 0; i < 1200 && !existsSync(marker); i++) {
    if (processHandle.exitCode !== null)
      throw Error("sidecar exited before marker");
    await delay(100);
  }
  if (!existsSync(marker)) throw Error("model command marker timeout");
  const pid = Number(readFileSync(marker, "utf8"));
  process.kill(pid, 0);
  log("detached-child-alive", { pid, session });
  writeFileSync(
    join(root, "fixture.json"),
    JSON.stringify({ root, port, pid, session, marker }),
  );
  log("ready-for-crash", { root });
  const database = join(root, "state/app/data/nexus.db");
  function snapshot() {
    return JSON.parse(
      execFileSync(
        "python3",
        [
          "-c",
          `import sqlite3,json,sys
c=sqlite3.connect('file:'+sys.argv[1]+'?mode=ro',uri=True);c.row_factory=sqlite3.Row;c.execute('BEGIN')
print(json.dumps({t:[dict(r) for r in c.execute('select * from '+t+' where session_key=?',(sys.argv[2],))] for t in ['sandbox_process_launches','sandbox_policy_receipts','sandbox_scratch_recoveries']}))`,
          database,
          session,
        ],
        { encoding: "utf8" },
      ),
    );
  }
  const before = snapshot();
  writeFileSync(join(root, "before.json"), JSON.stringify(before, null, 2));
  if (!before.sandbox_process_launches.some((r) => r.phase === "released"))
    throw Error("no released original process");
  if (!before.sandbox_policy_receipts.some((r) => r.phase === "confirmed"))
    throw Error("no confirmed original policy");
  const count = readFileSync(marker + ".count", "utf8");
  const original = processHandle;
  const exited = new Promise((resolve) =>
    original.once("exit", (code, signal) => resolve({ code, signal })),
  );
  original.kill("SIGKILL");
  log("original-host-killed", await exited);
  ws.close();
  ws = null;
  await delay(200);
  process.kill(pid, 0);
  log("descendant-survived-host", { pid });
  start("restarted");
  await healthy();
  const after = snapshot();
  writeFileSync(join(root, "after.json"), JSON.stringify(after, null, 2));
  const originalIDs = before.sandbox_process_launches
    .map((r) => r.launch_id)
    .sort();
  if (
    JSON.stringify(originalIDs) !==
    JSON.stringify(
      after.sandbox_process_launches.map((r) => r.launch_id).sort(),
    )
  )
    throw Error("recovery launched another task");
  if (
    after.sandbox_process_launches.some(
      (r) => r.phase !== "reaped" || r.resource_phase !== "complete",
    )
  )
    throw Error("process/resource recovery incomplete");
  if (after.sandbox_policy_receipts.some((r) => r.phase !== "reconciled"))
    throw Error("policy recovery incomplete");
  if (
    !after.sandbox_scratch_recoveries.length ||
    after.sandbox_scratch_recoveries.some((r) => r.phase !== "complete")
  )
    throw Error("scratch recovery incomplete");
  let alive = true;
  try {
    process.kill(pid, 0);
  } catch (e) {
    if (e.code === "ESRCH") alive = false;
    else throw e;
  }
  if (alive) throw Error("detached fixture still alive after recovery");
  await delay(2000);
  if (readFileSync(marker + ".count", "utf8") !== count)
    throw Error("command replayed");
  if (
    JSON.stringify(
      snapshot()
        .sandbox_process_launches.map((r) => r.launch_id)
        .sort(),
    ) !== JSON.stringify(originalIDs)
  )
    throw Error("new runtime appeared after recovery");
  log("PASS-default-crash-recovery", {
    root,
    processes: originalIDs.length,
    policies: after.sandbox_policy_receipts.length,
    scratch: after.sandbox_scratch_recoveries.length,
    noReplay: true,
    detachedChildGone: true,
  });
} catch (e) {
  log("failure", { error: redact(e.stack) });
  process.exitCode = 1;
} finally {
  if (ws) ws.close();
  if (processHandle?.exitCode === null && processHandle?.signalCode === null) {
    const exited = new Promise((resolve) =>
      processHandle.once("exit", resolve),
    );
    processHandle.kill("SIGTERM");
    const timer = setTimeout(() => processHandle.kill("SIGKILL"), 15000);
    await exited;
    clearTimeout(timer);
  }
  log("fixture-stopped", { root });
}
