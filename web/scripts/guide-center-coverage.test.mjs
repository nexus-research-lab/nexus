import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readSource = (relativePath) => readFile(
  new URL(`../${relativePath}`, import.meta.url),
  "utf8",
);

test("guide center keeps essential tours and advanced feature entry points", async () => {
  const [model, navigation, dialog, zh, en] = await Promise.all([
    readSource("src/features/onboarding/guide-center/guide-center-model.ts"),
    readSource("src/features/onboarding/guide-center/use-guide-center-navigation.ts"),
    readSource("src/features/onboarding/guide-center/guide-center-dialog.tsx"),
    readSource("src/shared/i18n/catalog/zh/navigation.ts"),
    readSource("src/shared/i18n/catalog/en/navigation.ts"),
  ]);

  for (const feature of [
    "automations",
    "connectors",
    "goal",
    "providers",
    "subagents",
    "thread",
    "workgraph",
  ]) {
    assert.match(model, new RegExp(`${feature}: "advanced-${feature}"`));
    assert.match(zh, new RegExp(`"guide_center\\.${feature}_title"`));
    assert.match(en, new RegExp(`"guide_center\\.${feature}_title"`));
  }

  assert.match(navigation, /AppRouteBuilders\.workGraphDistillations\(\)/);
  assert.match(navigation, /AppRouteBuilders\.scheduledTasks\(\)/);
  assert.match(navigation, /AppRouteBuilders\.connectors\(\)/);
  assert.match(navigation, /AppRouteBuilders\.settings\("providers"\)/);
  assert.match(dialog, /GUIDE_CENTER_SECTIONS/);
  assert.match(dialog, /overflow-y-auto/);
});
