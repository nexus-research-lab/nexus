// Explicit external-provider acceptance. Never loads the application's state/database env.
import { readFileSync } from "node:fs";
import { parseEnv } from "node:util";
import { spawn } from "node:child_process";

const source = parseEnv(readFileSync(process.env.NEXUS_SANDBOX_LIVE_ENV_FILE || ".env", "utf8"));
const token = source.ANTHROPIC_AUTH_TOKEN || source.ANTHROPIC_API_KEY;
const model = source.ANTHROPIC_MODEL || source.DEFAULT_MODEL;
if (!token || !source.ANTHROPIC_BASE_URL || !model) {
  throw new Error("The selected .env must provide a third-party token, base URL and model.");
}
const runtime = process.env.NEXUS_SANDBOX_LIVE_RUNTIME || "both";
if (!["both", "nxs", "claude"].includes(runtime)) throw new Error("Unsupported runtime selection.");
if (process.platform !== "darwin") throw new Error("This acceptance probe requires native macOS.");
const env = {
  ...process.env,
  GOWORK: "off",
  NEXUS_SANDBOX_LIVE_PROVIDER: "1",
  NEXUS_SANDBOX_LIVE_RUNTIME: runtime,
  NEXUS_SANDBOX_LIVE_TOKEN: token,
  NEXUS_SANDBOX_LIVE_BASE_URL: source.ANTHROPIC_BASE_URL,
  NEXUS_SANDBOX_LIVE_MODEL: model,
};
const child = spawn("go", ["test", "./internal/runtime/clientopts", "-run", "^TestDesktopSandboxLiveProvider$", "-count=1", "-v", "-timeout=12m"], {
  env, timeout: 13 * 60_000, stdio: ["ignore", "pipe", "pipe"],
});
for (const output of [child.stdout, child.stderr]) {
  let pending = "";
  output.setEncoding("utf8");
  output.on("data", (chunk) => {
    pending += chunk;
    let newline;
    while ((newline = pending.indexOf("\n")) !== -1) {
      process.stdout.write(pending.slice(0, newline + 1).replaceAll(token, "[redacted]"));
      pending = pending.slice(newline + 1);
    }
  });
  output.on("end", () => {
    if (pending) process.stdout.write(pending.replaceAll(token, "[redacted]"));
  });
}
child.on("error", (error) => console.error(error.code || "live acceptance runner failed"));
child.on("close", (code) => { process.exitCode = code ?? 1; });
