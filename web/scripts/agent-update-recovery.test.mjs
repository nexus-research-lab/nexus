import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("Agent settings keep uncertain writes locked until authoritative state changes", async () => {
  const command = await read(
    "src/features/agents/options/editor/use-agent-options-save-command.ts",
  );
  const feedback = await read(
    "src/features/agents/options/editor/use-agent-save-feedback.ts",
  );

  assert.match(command, /projectMutationFailure\(error, fallbackError\)/);
  assert.match(command, /failure\.effect !== "not_applied"/);
  assert.match(
    command,
    /sourceScopeKeyRef\.current === expected\.sourceScopeKey/,
  );
  assert.match(command, /!repeatBlocked/);
  assert.match(feedback, /current\?\.blocksRepeat[\s\S]*current[\s\S]*null/);
});

test("Agent settings failure feedback always states result, impact, and next step", async () => {
  const actions = await read(
    "src/features/agents/options/components/agent-options-editor-actions.tsx",
  );
  const controller = await read(
    "src/features/agents/options/editor/use-agent-options-editor-controller.ts",
  );

  assert.match(actions, /feedback\.message/);
  assert.match(actions, /feedback\.impact/);
  assert.match(actions, /feedback\.nextStep/);
  assert.match(actions, /aria-live="polite"/);
  for (const effect of ["accepted", "committed", "not_applied", "unknown"]) {
    assert.match(controller, new RegExp(`${effect}:[\\s\\S]*impact:[\\s\\S]*message:[\\s\\S]*nextStep:`));
  }
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
