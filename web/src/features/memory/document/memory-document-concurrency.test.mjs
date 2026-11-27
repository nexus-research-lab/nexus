// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("a successful save advances only the submitted baseline and keeps newer typing", async () => {
  const {
    classifyMemorySaveReconciliation,
    mergeSavedMemoryDocument,
  } = await import("@/features/memory/document/use-memory-document-state.ts");
  const current = {
    command: "save",
    commandError: null,
    content: "old",
    draft: "newer typing",
    editing: true,
    isLoading: false,
    resourceError: null,
    revision: "sha256:old",
    saveIssue: null,
    scopeKey: "agent:memory/topic.md",
  };

  const merged = mergeSavedMemoryDocument(
    current,
    "submitted draft",
    "submitted draft",
    "sha256:submitted",
  );
  assert.equal(merged.content, "submitted draft");
  assert.equal(merged.revision, "sha256:submitted");
  assert.equal(merged.draft, "newer typing");
  assert.equal(merged.editing, true);

  const issue = {
    attemptedDraft: "submitted draft",
    expectedRevision: "sha256:old",
    kind: "outcome_unknown",
    reconciliationFailed: false,
  };
  assert.equal(classifyMemorySaveReconciliation(issue, {
    content: "submitted draft",
    path: "memory/topic.md",
    revision: "sha256:submitted",
  }), "saved");
  assert.equal(classifyMemorySaveReconciliation(issue, {
    content: "old",
    path: "memory/topic.md",
    revision: "sha256:old",
  }), "not_applied");
  assert.equal(classifyMemorySaveReconciliation(issue, {
    content: "agent update",
    path: "memory/topic.md",
    revision: "sha256:agent",
  }), "conflict");
});
