import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("channel authorization does not expose server result text", async () => {
  const [presenter, dialog] = await Promise.all([
    read("src/features/capability/channels/authorization/channel-authorization-presenter.tsx"),
    read("src/features/capability/channels/authorization/channel-authorization-dialog.tsx"),
  ]);

  assert.doesNotMatch(presenter, /result\.message/);
  assert.doesNotMatch(dialog, /failure\.message/);
  assert.match(dialog, /failure\.title/);
  assert.match(dialog, /failure\.impact/);
  assert.match(dialog, /failure\.nextStep/);
  assert.match(dialog, /aria-atomic="true"[\s\S]*aria-live="polite"/);
});

test("negative authorization ACK locks writes until read-only reconciliation", async () => {
  const [model, presenter, dialog, zh] = await Promise.all([
    read("src/features/capability/channels/authorization/channel-authorization-model.ts"),
    read("src/features/capability/channels/authorization/channel-authorization-presenter.tsx"),
    read("src/features/capability/channels/authorization/channel-authorization-dialog.tsx"),
    read("src/shared/i18n/catalog/zh/capability.ts"),
  ]);

  assert.match(model, /writeLocked: delivery === "rejected"/);
  assert.match(presenter, /error\?\.writeLocked/);
  assert.match(dialog, /disabled=\{busy \|\| writeLocked\}/);
  assert.match(dialog, /disabled=\{!code\.trim\(\) \|\| busy \|\| expiry\.expired \|\| writeLocked\}/);
  assert.match(zh, /无法确认验证码提交结果/);
  assert.match(zh, /回到频道页查看最新连接状态/);
  assert.match(zh, /确认前不要重复提交或取消/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
