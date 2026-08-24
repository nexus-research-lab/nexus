import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("connection dialogs use plain chrome without generated implementation prose", async () => {
  const [
    credential,
    oauth,
    device,
    feishu,
    channelAuthorization,
    channelConnect,
    channelGuide,
  ] = await Promise.all([
    read("web/src/features/capability/connectors/auth/connector-credential-dialog.tsx"),
    read("web/src/features/capability/connectors/auth/connector-oauth-client-dialog.tsx"),
    read("web/src/features/capability/connectors/auth/device-flow/connector-device-auth-dialog.tsx"),
    read("web/src/features/capability/connectors/auth/feishu/feishu-app-connection-dialog.tsx"),
    read("web/src/features/capability/channels/authorization/channel-authorization-dialog.tsx"),
    read("web/src/features/capability/channels/connection/channel-connect-dialog.tsx"),
    read("web/src/features/capability/channels/connection/channel-guide.tsx"),
  ]);

  for (const source of [credential, oauth, device, feishu, channelAuthorization, channelConnect]) {
    assert.match(source, /<UiDialogHeader[\s\S]*?appearance="plain"/);
    assert.doesNotMatch(source, /<UiDialogHeader[\s\S]*?\bsubtitle=/);
  }
  assert.doesNotMatch(credential, /Agent 运行时|挂载对应 MCP|KeyRound|<UiPanel/);
  assert.doesNotMatch(device, /Github|QrCode|径向|Nexus 已取得|<UiPanel/);
  assert.doesNotMatch(feishu, /KeyRound|ScanLine|历史 App ID|该阶段不再|<UiPanel/);
  assert.doesNotMatch(channelAuthorization, /ShieldCheck|TriangleAlert|radial-gradient|安全提交|安全连接 Channel/);
  assert.match(channelGuide, /<details[\s\S]*连接说明/);
  assert.match(channelGuide, /runtimeNote[\s\S]*border-t/);
  assert.match(channelConnect, /<UiDialogBody className="space-y-5 px-5"/);
});

test("generic internal errors fall back to the current operation", async () => {
  const [errorMessage, conversationZh] = await Promise.all([
    read("web/src/lib/error-message.ts"),
    read("web/src/shared/i18n/catalog/zh/conversation.ts"),
  ]);

  assert.match(errorMessage, /INTERNAL_ERROR_PLACEHOLDERS/);
  assert.match(errorMessage, /"服务内部错误"/);
  assert.match(errorMessage, /!INTERNAL_ERROR_PLACEHOLDERS\.has\(message\)/);
  assert.match(conversationZh, /"execution\.workflow_schedule_failed": "暂时无法保存，请重试。"/);
});

function read(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}
