// @vitest-environment node

import assert from "node:assert/strict";

import path from "node:path";
import { test } from "vitest";

const recoveryModule = import("@/features/conversation/shared/editor/text/text-file-editor-recovery.ts");

test("workspace text reconciliation preserves newer typing and distinguishes exact outcomes", async () => {
  const {
    classifyTextFileSaveReconciliation,
    mergeSavedTextFile,
  } = await recoveryModule;
  const issue = {
    attemptedDraft: "submitted",
    expectedRevision: "sha256:base",
    kind: "outcome_unknown",
    reconciliationFailed: false,
  };

  assert.equal(classifyTextFileSaveReconciliation(issue, {
    content: "submitted",
    path: "notes.md",
    revision: "sha256:submitted",
  }), "intent_present");
  assert.equal(classifyTextFileSaveReconciliation(issue, {
    content: "base",
    path: "notes.md",
    revision: "sha256:base",
  }), "retry_ready");
  assert.equal(classifyTextFileSaveReconciliation(issue, {
    content: "other writer",
    path: "notes.md",
    revision: "sha256:other",
  }), "conflict");

  assert.deepEqual(mergeSavedTextFile({
    draftContent: "newer typing",
    isEditing: true,
    revision: "sha256:base",
    savedContent: "base",
  }, "submitted", {
    content: "submitted",
    path: "notes.md",
    revision: "sha256:submitted",
  }), {
    draftContent: "newer typing",
    isEditing: true,
    revision: "sha256:submitted",
    savedContent: "submitted",
  });
});

test("external live updates never overwrite a dirty or outcome-unknown draft", async () => {
  const { resolveTextFileLiveUpdateIntent } = await recoveryModule;
  const liveState = {
    agent_id: "agent-a",
    path: "notes.md",
    status: "updated",
    version: 4,
    source: "agent",
    live_content: "external",
    content_revision: "sha256:external",
    updated_at: Date.now(),
  };
  const base = {
    agentId: "agent-a",
    consumed: { scopeKey: "scope", version: 3 },
    hasLoadedContent: true,
    isEditing: false,
    isSaving: false,
    liveState,
    path: "notes.md",
    revision: "sha256:base",
    saveIssue: null,
    scopeKey: "scope",
  };

  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: true,
  }), { kind: "conflict", version: 4 });
  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: false,
  }), {
    content: "external",
    kind: "apply",
    revision: "sha256:external",
    version: 4,
  });
  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: true,
    saveIssue: {
      attemptedDraft: "draft",
      expectedRevision: "sha256:base",
      kind: "outcome_unknown",
      reconciliationFailed: false,
    },
  }), { kind: "ignore" });
  assert.deepEqual(resolveTextFileLiveUpdateIntent({
    ...base,
    isDirty: false,
    revision: "sha256:external",
  }), { kind: "consume", version: 4 });
});
