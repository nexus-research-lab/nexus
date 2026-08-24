import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("Agent and Room form dialogs use plain chrome without internal subtitles", async () => {
  const [agentDialog, agentModel, agentEditor, roomDialog, roomModel] = await Promise.all([
    read("web/src/features/agents/options/dialog/agent-options-dialog.tsx"),
    read("web/src/features/agents/options/dialog/agent-options-dialog-model.ts"),
    read("web/src/features/agents/options/agent-options-editor.tsx"),
    read("web/src/features/conversation/room/members/create-room-dialog.tsx"),
    read("web/src/features/conversation/room/members/create-room-dialog-model.ts"),
  ]);

  assert.match(agentDialog, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.doesNotMatch(agentDialog, /Settings|subtitle=|header\.subtitle/);
  assert.match(agentModel, /getAgentOptionsDialogTitle/);
  assert.doesNotMatch(agentModel, /subtitle|id_prefix|source\.agentId/);
  assert.match(agentEditor, /<UiDialogFooter appearance="plain">/);

  assert.match(roomDialog, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.match(roomDialog, /<UiDialogBody[\s\S]*scrollable/);
  assert.match(roomDialog, /<UiDialogFooter appearance="plain">/);
  assert.doesNotMatch(roomDialog, /MessageCirclePlus|subtitle=|getDialogActionClassName/);
  assert.doesNotMatch(roomModel, /subtitle/);
});

test("Provider dialogs read like settings rows rather than icon cards", async () => {
  const [addModel, modelOptions, capabilitySwitch, deleteUsage, settingsZh] = await Promise.all([
    read("web/src/features/settings/provider-settings/dialogs/provider-settings-add-model-dialog.tsx"),
    read("web/src/features/settings/provider-settings/dialogs/provider-settings-model-options-dialog.tsx"),
    read("web/src/features/settings/provider-settings/components/provider-settings-capability-switch.tsx"),
    read("web/src/features/settings/provider-settings/dialogs/provider-settings-delete-usage-dialog.tsx"),
    read("web/src/shared/i18n/catalog/zh/settings.ts"),
  ]);

  for (const source of [addModel, modelOptions, deleteUsage]) {
    assert.match(source, /<UiDialogHeader[\s\S]*appearance="plain"/);
    assert.match(source, /<UiDialogFooter[\s\S]*appearance="plain"/);
    assert.doesNotMatch(source, /<UiDialogHeader[\s\S]*subtitle=/);
  }
  assert.doesNotMatch(addModel, /ListPlus|surface-radius-md/);
  assert.doesNotMatch(modelOptions, /Brain|Database|Eye|Image|SlidersHorizontal|Wrench|\bicon=/);
  assert.doesNotMatch(capabilitySwitch, /ReactNode|\bicon\b|rounded-\[10px\]/);
  assert.doesNotMatch(deleteUsage, /Trash2|font-mono[\s\S]*agent\.agent_id/);
  assert.match(settingsZh, /"settings\.providers\.force_delete": "仍要删除"/);
  assert.doesNotMatch(settingsZh, /add_model_subtitle|model_options_subtitle|delete_usage_title/);
});

function read(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}
