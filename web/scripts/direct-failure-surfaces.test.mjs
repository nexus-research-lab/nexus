import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

async function source(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

test("direct conversation failure surfaces use the complete recovery contract", async () => {
  const [subagent, metadataEditor, distillation, artifact, generativeUI] = await Promise.all([
    source("src/features/conversation/shared/subagent/thread/subagent-task-thread-view.tsx"),
    source("src/features/conversation/shared/execution/workgraph-metadata-editor-dialog.tsx"),
    source("src/features/conversation/shared/execution/workgraph-distillation-dialog.tsx"),
    source("src/features/conversation/shared/message/blocks/artifact/workgraph/workgraph-artifact-block.tsx"),
    source("src/features/conversation/shared/message/blocks/tool/generative-ui-block.tsx"),
  ]);

  for (const item of [subagent, metadataEditor, distillation, artifact, generativeUI]) {
    assert.match(item, /<UiResourceState/);
    assert.match(item, /impact=/);
    assert.match(item, /nextStep=/);
  }
  assert.match(metadataEditor, /projectMutationFailure/);
  assert.match(metadataEditor, /failure\.effect !== "not_applied"/);
  assert.match(metadataEditor, /onRetryProjection/);
  assert.match(distillation, /projectMutationFailure/);
  assert.match(generativeUI, /missingWidgetCode/);
});

test("CC Switch keeps read, write, and committed refresh failures separate", async () => {
  const dialog = await source(
    "src/features/provider-imports/cc-switch/provider-ccswitch-dialog.tsx",
  );

  assert.match(dialog, /kind: "read"/);
  assert.match(dialog, /kind: "sync"/);
  assert.match(dialog, /kind: "committed_refresh"/);
  assert.match(dialog, /projectMutationFailure/);
  assert.match(dialog, /controlsLocked/);
  assert.match(dialog, /handleRefreshAfterSync/);
  assert.match(dialog, /<CCSwitchFailureState/);
  assert.match(dialog, /impact=/);
  assert.match(dialog, /nextStep=/);
});

test("Launcher directory read failures preserve explicit impact and recovery", async () => {
  const [launcher, staleNotice] = await Promise.all([
    source("src/pages/launcher/launcher-page.tsx"),
    source("src/features/home/home-directory-refresh-error-notice.tsx"),
  ]);

  assert.match(launcher, /<UiResourceState/);
  assert.match(launcher, /directory_load_failed_impact/);
  assert.match(launcher, /directory_load_failed_next_step/);
  assert.match(staleNotice, /directory_refresh_failed_impact/);
  assert.match(staleNotice, /directory_refresh_failed_next_step/);
});
