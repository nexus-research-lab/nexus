import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("Pairing and custom MCP forms keep optional protocol detail out of primary chrome", async () => {
  const [pairingDialog, customMcpDialog, capabilityZh] = await Promise.all([
    read("web/src/features/capability/channels/pairings/pairing-create-dialog.tsx"),
    read("web/src/features/capability/connectors/custom/custom-mcp-dialog.tsx"),
    read("web/src/shared/i18n/catalog/zh/capability.ts"),
  ]);

  for (const source of [pairingDialog, customMcpDialog]) {
    assert.match(source, /<UiDialogHeader[\s\S]*appearance="plain"/);
    assert.match(source, /<UiDialogFooter[\s\S]*appearance="plain"/);
    assert.doesNotMatch(source, /<UiDialogHeader[\s\S]*subtitle=/);
  }
  assert.doesNotMatch(pairingDialog, /ShieldCheck|icon=|rounded-\[12px\]/);
  assert.match(pairingDialog, /<details[\s\S]*账号、话题与初始状态/);
  assert.doesNotMatch(customMcpDialog, /\bCable\b|className="h-\[min\(82dvh,760px\)\]/);
  assert.doesNotMatch(capabilityZh, /custom_mcp_dialog_description/);
});

test("Connector capability detail reads as content instead of a status card", async () => {
  const source = await read("web/src/features/capability/connectors/detail/connector-feature-dialog.tsx");

  assert.match(source, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.match(source, /<details[\s\S]*OAuth scopes/);
  assert.doesNotMatch(source, /UiPanel|UiBadge|icon=|subtitle=|<Check/);
});

test("Scheduled destructive actions use product decision dialogs without internal IDs", async () => {
  const [directory, historyDialog, historyActions] = await Promise.all([
    read("web/src/features/capability/scheduled/scheduled-tasks-directory.tsx"),
    read("web/src/features/capability/scheduled/history/scheduled-task-run-history-dialog.tsx"),
    read("web/src/features/capability/scheduled/history/use-scheduled-task-run-history-actions.ts"),
  ]);

  assert.match(directory, /<ConfirmDialog[\s\S]*title="删除任务"/);
  assert.match(historyDialog, /<ConfirmDialog[\s\S]*title="释放运行占用"/);
  assert.match(historyDialog, /message="这次运行会标记为已取消，任务随后可以重新运行。"/);
  assert.doesNotMatch(`${directory}\n${historyActions}`, /window\.confirm/);
  assert.doesNotMatch(historyDialog, /message=\{[^}]*run_id/);
});

function read(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}
