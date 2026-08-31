import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("workspace preview failures keep the original file and provide recovery", async () => {
  const sources = await Promise.all([
    read("src/features/conversation/shared/editor/document/document-preview-view.tsx"),
    read("src/features/conversation/shared/editor/presentation/presentation-file-preview.tsx"),
    read("src/features/conversation/shared/editor/spreadsheet/spreadsheet-file-preview.tsx"),
    read("src/features/conversation/shared/editor/media/media-file-preview.tsx"),
  ]);
  const officeFailure = await read(
    "src/features/conversation/shared/editor/office-preview-fallbacks.tsx",
  );

  assert.match(officeFailure, /impact=/);
  assert.match(officeFailure, /nextStep=/);
  assert.match(officeFailure, /primaryAction=/);
  assert.match(officeFailure, /urgency="polite"/);
  for (const source of sources) {
    assert.match(source, /PreviewFailure|OfficePreviewFailureState/);
    assert.doesNotMatch(source, /status\.message|error\.message/);
  }
});

test("attachment and Mermaid render failures do not expose parser or file errors", async () => {
  const attachmentDialog = await read(
    "src/features/conversation/shared/composer/attachments/composer-attachment-preview-dialog.tsx",
  );
  const attachmentController = await read(
    "src/features/conversation/shared/composer/attachments/use-composer-attachments.ts",
  );
  const mermaidState = await read(
    "src/shared/ui/markdown/mermaid/use-mermaid-svg.ts",
  );
  const mermaidView = await read(
    "src/shared/ui/markdown/mermaid/mermaid-view-parts.tsx",
  );

  assert.match(attachmentDialog, /attachment_preview_failed_impact/);
  assert.match(attachmentDialog, /attachment_preview_failed_next_step/);
  assert.match(attachmentDialog, /urgency="polite"/);
  assert.doesNotMatch(attachmentController, /error instanceof Error[\s\S]*error\.message/);
  assert.match(mermaidState, /"invalid_syntax" \| "render_failed"/);
  assert.doesNotMatch(mermaidState, /error\.message/);
  assert.match(mermaidView, /render_failed_impact/);
  assert.match(mermaidView, /render_failed_next_step/);
  assert.doesNotMatch(mermaidView, /<pre[^>]*>\{error\}/);
});

test("scheduled resource reads preserve same-scope snapshots and offer explicit retry", async () => {
  const resource = await read(
    "src/features/capability/scheduled/dialog/resources/use-dialog-resource.ts",
  );
  const failureView = await read(
    "src/features/capability/scheduled/dialog/form/task-basics-advanced.tsx",
  );

  assert.match(resource, /retry: \(\) => void/);
  assert.match(resource, /current\.key === requestKey/);
  assert.match(resource, /\{ \.\.\.current, error: fallbackError, loading: false \}/);
  assert.doesNotMatch(resource, /getErrorMessage|error\.message/);
  assert.match(failureView, /scheduled_dialog_resource_load_impact/);
  assert.match(failureView, /scheduled_dialog_resource_load_next_step/);
  assert.match(failureView, /primaryAction=/);
});

test("Agent option supporting reads keep drafts and last connector snapshots", async () => {
  const connectors = await read(
    "src/features/agents/options/editor/use-agent-connectors.ts",
  );
  const profileTemplate = await read(
    "src/features/agents/options/editor/use-agent-profile-template.ts",
  );
  const advancedView = await read(
    "src/features/agents/options/components/agent-options-advanced-tab.tsx",
  );
  const identityView = await read(
    "src/features/agents/options/components/identity/agent-options-identity-tab.tsx",
  );

  assert.match(connectors, /items: current\.items/);
  assert.doesNotMatch(connectors, /error\.message/);
  assert.doesNotMatch(profileTemplate, /error\.message/);
  assert.match(advancedView, /connector_load_failed_stale_impact/);
  assert.match(advancedView, /connector_load_failed_empty_impact/);
  assert.match(advancedView, /onRetryConnectors/);
  assert.match(identityView, /profile_template_load_failed_impact/);
  assert.match(identityView, /profile_template_load_failed_next_step/);
  assert.match(identityView, /onRetryProfileTemplate/);
});

test("Skill source reads use safe copy and uncertain writes reconcile by reading", async () => {
  const sources = await read(
    "src/features/capability/skills/controller/use-external-skill-sources.ts",
  );
  const catalog = await read(
    "src/features/capability/skills/controller/use-skill-catalog.ts",
  );

  assert.match(sources, /skill_sources_load_failed_title/);
  assert.match(sources, /impact: t\("state\.read_failure_impact"\)/);
  assert.match(sources, /nextStep: t\("state\.retry_next_step"\)/);
  assert.match(sources, /mutationLockedRef/);
  assert.match(sources, /skill_source_unknown_next_step/);
  assert.doesNotMatch(sources, /error\.message|getErrorMessage/);
  assert.doesNotMatch(catalog, /error\.message|getErrorMessage/);
});

test("Memory command errors use the same problem, impact, and recovery surface", async () => {
  const memoryPanel = await read(
    "src/features/memory/document/memory-document-panel.tsx",
  );

  assert.doesNotMatch(memoryPanel, />\s*\{commandError\}\s*<\/div>/);
  assert.match(memoryPanel, /description=\{commandError\}/);
  assert.match(memoryPanel, /feedback\.unconfirmed_impact/);
  assert.match(memoryPanel, /feedback\.unconfirmed_next_step/);
  assert.match(memoryPanel, /state\.reload_check/);
});

async function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
