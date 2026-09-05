// INPUT: Real Execution canvas and an isolated local graph fixture.
// OUTPUT: Exact node/edge selection, independent file action and close behavior regression evidence.
// POS: DOM integration tests; browser tests separately verify inverse zoom and opaque inspector geometry.

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { WorkGraphGallery } from "@/dev/ui-gallery/ui-gallery-workgraph";
import { I18nProvider } from "@/shared/i18n/i18n-provider";

describe("Execution graph inspectors", () => {
  it("switches exact identities and retains the artifact's workspace action", () => {
    const { container } = render(<I18nProvider><WorkGraphGallery locale="en" /></I18nProvider>);
    const draft = container.querySelector<HTMLButtonElement>('[data-execution-graph-node-id="draft"]')!;
    fireEvent.click(draft);
    const inspector = screen.getByRole("complementary", { name: /Draft report/ });
    expect(inspector.dataset.executionSelectedNodeDetail).toBe("draft");
    expect(within(inspector).getByRole("heading", { level: 3 }).textContent).toBe("Draft report");
    fireEvent.click(within(inspector).getByRole("button", { name: /^review\.md/ }));
    expect(container.querySelector("[data-gallery-workgraph-file]")?.textContent).toBe("author:reports/review.md");
    expect(inspector.isConnected).toBe(true);

    fireEvent.click(container.querySelector('[data-execution-edge-hit-target="draft-review"]')!);
    expect(inspector.isConnected).toBe(false);
    const edge = screen.getByRole("complementary");
    expect(edge.dataset.executionSelectedEdgeDetail).toBe("draft-review");
    expect(edge.textContent).toContain("draft-run");
    expect(edge.textContent).toContain("review-run");
    fireEvent.click(within(edge).getByRole("button", { name: /close relation details|关闭关系详情/i }));
    expect(screen.queryByRole("complementary")).toBeNull();

    fireEvent.click(draft);
    fireEvent.keyDown(draft, { key: "Escape" });
    expect(screen.queryByRole("complementary")).toBeNull();
  });
});
