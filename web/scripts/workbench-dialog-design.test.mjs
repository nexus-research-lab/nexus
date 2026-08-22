import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

test("remaining compact forms and guide center use plain chrome", async () => {
  const [goal, contacts, guide, guideController, ccSwitch] = await Promise.all([
    read("web/src/features/conversation/shared/goal/goal-draft-form.tsx"),
    read("web/src/features/contacts/agent-communication-view.tsx"),
    read("web/src/features/onboarding/guide-center/guide-center-dialog.tsx"),
    read("web/src/features/onboarding/guide-center/use-guide-center-controller.ts"),
    read("web/src/features/provider-imports/cc-switch/provider-ccswitch-dialog.tsx"),
  ]);

  assert.match(goal, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.doesNotMatch(goal, /\bTarget\b|icon=/);
  assert.match(contacts, /function AddContactDialog[\s\S]*appearance="plain"/);
  assert.doesNotMatch(contacts, /icon=\{<UserRoundPlus/);
  assert.match(guide, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.doesNotMatch(`${guide}\n${guideController}`, /guide_center_description|describedBy/);
  assert.match(ccSwitch, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.match(ccSwitch, /<UiDialogFooter[\s\S]*appearance="plain"/);
  assert.doesNotMatch(ccSwitch, /icon=\{<RefreshCw|className="h-\[500px\]/);

  const navigationZh = await read("web/src/shared/i18n/catalog/zh/navigation.ts");
  assert.doesNotMatch(navigationZh, /一句话就能直达|你的工作入口/);
});

test("session navigator preview is a lightweight non-modal card", async () => {
  const source = await read(
    "web/src/features/conversation/shared/session-navigator/conversation-session-navigator.tsx",
  );

  assert.match(source, /data-session-navigator-preview="true"/);
  assert.doesNotMatch(source, /UiDialog(?:Shell|Header|Body)|MessageSquareText|role="dialog"/);
});

test("provider setup is a single-column connection flow without brand theater", async () => {
  const [dialog, zh, en] = await Promise.all([
    read("web/src/features/onboarding/provider-setup/provider-setup-dialog.tsx"),
    read("web/src/shared/i18n/catalog/zh/navigation.ts"),
    read("web/src/shared/i18n/catalog/en/navigation.ts"),
  ]);

  assert.match(dialog, /<UiDialogHeader[\s\S]*appearance="plain"/);
  assert.doesNotMatch(dialog, /NexusPresence|nexus-mascot|\/logo\.webp|FeatureItem|provider_setup_features_/);
  assert.doesNotMatch(dialog, /md:grid-cols-\[176px|MessageSquare|UsersRound|Puzzle|Clock3/);
  assert.match(zh, /"onboarding\.provider_setup_title": "连接模型服务"/);
  assert.doesNotMatch(zh, /我会完成余下设置|"onboarding\.provider_setup_features_/);
  assert.doesNotMatch(en, /I'll handle the rest|"onboarding\.provider_setup_features_/);
});

test("custom dialog overlays stay compact and avoid repeated metadata", async () => {
  const [history, mobileSwitcher, modelControl, memoryOverlay, iconPicker] = await Promise.all([
    read("web/src/features/conversation/room/surface/history/room-history-menu.tsx"),
    read("web/src/features/conversation/room/surface/mobile/room-mobile-conversation-switcher.tsx"),
    read("web/src/features/conversation/shared/composer/components/footer/composer-room-model-control.tsx"),
    read("web/src/features/conversation/shared/message/item/view/assistant/assistant-message-stats.tsx"),
    read("web/src/shared/ui/icon-picker/icon-picker-popover.tsx"),
  ]);

  assert.doesNotMatch(history, /room\.conversation_count|room\.history_empty_hint|\bClock3\b/);
  assert.doesNotMatch(mobileSwitcher, /room\.conversation_count|MessageSquare|Clock3/);
  assert.match(modelControl, /UiActionMenuContent[\s\S]*density="compact"/);
  assert.match(memoryOverlay, /<h3[\s\S]*<ul/);
  assert.doesNotMatch(iconPicker, /\{maxIcons\}\s*<\/span>/);
});

function read(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}
