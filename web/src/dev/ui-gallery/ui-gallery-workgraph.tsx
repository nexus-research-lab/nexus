// INPUT: Local immutable Execution fixture and Gallery locale.
// OUTPUT: Real canvas with node/edge inspectors and observable workspace-file actions.
// POS: Development-only product fixture; no service reads, execution commands or synthetic component copies.

import { useState } from "react";
import { ExecutionWorkGraphCanvas } from "@/features/conversation/shared/execution/execution-workgraph-canvas";
import { NamedWorkGraphSketch } from "@/features/conversation/shared/execution/named-workgraph-sketch";
import type { Locale } from "@/shared/i18n/messages";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ExecutionView } from "@/types/conversation/execution";
import { galleryText } from "./ui-gallery-copy";

const EXECUTION: ExecutionView = {
  id: "gallery-execution", session_key: "dm:gallery", scope_kind: "dm", status: "active", version: 1,
  objective: "Inspect the current execution", created_at: "2026-09-05T00:00:00Z", updated_at: "2026-09-05T00:00:00Z",
  progress: { total: 2, required: 2, accepted: 1, running: 1, blocked: 0, submitted: 0, ready: 0, waiting: 0, changes_requested: 0, failed: 0, cancelled: 0 },
  graph: {
    runtime_node_total: 3, runtime_edge_total: 1, runtime_nodes_truncated: false, runtime_edges_truncated: false,
    nodes: [
      { id: "draft", kind: "tool", visibility: "primary", work_item_id: "", agent_id: "author", name: "Draft report", description: "Prepare a reviewable report with the source evidence.", position: 0, run_status: "succeeded",
        runs: [{ id: "draft-run", status: "succeeded", result_summary: "Report is ready for review.", artifacts: [{ type: "workspace_file_artifact", path: "reports/review.md", label: "Review report", scope: "agent_workspace", workspace_agent_id: "author" }] }] },
      { id: "review", kind: "tool", visibility: "primary", work_item_id: "", agent_id: "reviewer", name: "Review report", description: "Verify the report without changing the source execution.", position: 1, run_status: "running" },
      { id: "evidence", parent_node_id: "draft", kind: "tool", visibility: "detail", work_item_id: "", name: "Read evidence", result_summary: "The source references remain attached to the current draft.", position: 2, run_status: "succeeded" },
    ],
    edges: [{ id: "draft-review", kind: "dependency", source_node_id: "draft", target_node_id: "review", source_node_run_id: "draft-run", target_node_run_id: "review-run", created_at: "2026-09-05T00:00:00Z" }],
  },
};

export function WorkGraphGallery({ locale }: { locale: Locale }) {
  const [openedFile, setOpenedFile] = useState("");
  return (
    <section className="min-w-0 space-y-3" data-gallery-workgraph>
      <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
        {galleryText(locale, "WorkGraph 真实视图", "WorkGraph view")}
      </h2>
      <div className="flex h-[28rem] min-w-0">
        <ExecutionWorkGraphCanvas
          currentId="review" directory={{ author: { id: "author", name: "Author", avatar: null }, reviewer: { id: "reviewer", name: "Reviewer", avatar: null } }}
          execution={EXECUTION} nodePresentation="summary" taskRuns={[]}
          onOpenWorkspaceFile={(path, agentId) => setOpenedFile(`${agentId}:${path}`)}
        />
      </div>
      <output className={getUiTypographyClassName({ role: "caption", tone: "soft" })} data-gallery-workgraph-file>{openedFile}</output>
      <NamedWorkGraphSketch dependencies={[{ logical_key: "review", depends_on_logical_key: "draft", kind: "hard" }]} nodes={[
        { logical_key: "draft", role: "key", kind: "produce", subject: "Draft report", objective: "Preserve the source evidence.", deliverable: "Reviewable report", required: true, position: 0 },
        { logical_key: "review", role: "key", kind: "verify", subject: "Review report", objective: "Verify the report and its sources.", deliverable: "Verified report", required: true, terminal: true, position: 1 },
      ]} />
    </section>
  );
}
