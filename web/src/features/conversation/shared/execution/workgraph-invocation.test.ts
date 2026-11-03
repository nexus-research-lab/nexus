/**
 * INPUT: Typed parameter definitions, saved template and confirmation snapshots.
 * OUTPUT: Copy examples preserve types/defaults; receipt reconciliation includes parameter semantics.
 * POS: Reuse and save-confirmation behavior tests without invoking models.
 */
import { describe, expect, it } from "vitest";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";
import { workgraphInvocationExample } from "./workgraph-invocation";
import { savedWorkGraphMatchesPreview } from "./workgraph-save-state";

const workflow: WorkGraphWorkflow = { id: "template", slash_name: "report", title: "Report", source_execution_id: "source", source_session_key: "session", objective: "Verify", nodes: [], version: 1, created_at: "", updated_at: "" };
describe("typed template reuse", () => {
  it("copies typed defaults and required values while leaving optional inputs absent", () => {
    const example = workgraphInvocationExample({ ...workflow, parameters: [
      { name: "count", type: "integer", required: true, minimum: 2.2 },
      { name: "format", type: "string", required: true, enum: ["md", "txt"] },
      { name: "verbose", type: "boolean", default: false },
      { name: "links", type: "string_list", default: ["fixture:1"] },
      { name: "optional", type: "string" },
    ] });
    expect(JSON.parse(example.slice("/report ".length))).toEqual({ parameters: { count: 3, format: "md", verbose: false, links: ["fixture:1"] }, request: "" });
    expect(workgraphInvocationExample(workflow)).toBe("/report ");
  });
  it("does not confirm a lost save receipt when parameter contracts differ", () => {
    const saved = { ...workflow, parameters: [{ name: "topic", type: "string" as const, required: true }] };
    const preview = { ...saved, preview_id: "draft", expires_at: "", parameters: [{ name: "topic", type: "string" as const, required: false }] };
    expect(savedWorkGraphMatchesPreview(saved, preview)).toBe(false);
  });
});
