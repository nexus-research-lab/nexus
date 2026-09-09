/**
 * INPUT: Real DM/Thread process groups, tool results and streaming transitions.
 * OUTPUT: Independent two-level disclosure without streaming-driven expansion.
 * POS: Assistant process interaction regression; no disclosure components mocked.
 */
import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ContentBlock } from "@/types/conversation/message/content";
import { ThinkingBlock } from "../../../blocks/thinking-block";
import { AssistantToolRuns } from "./assistant-dm-tool-runs";
import type { AssistantActivityState, AssistantContentEnvironment, AssistantPermissionState } from "./assistant-message-model";

const activity: AssistantActivityState = {
  emptyStreamStatus: null, label: null, showCursor: false,
  standalone: false, state: null, toolUseSummary: null,
};
const permissions: AssistantPermissionState = {
  all: [], matchedByToolUseId: new Map(), owner: "composer", unmatched: [],
};
const content: ContentBlock[] = [
  { type: "thinking", thinking: "Inspect the source" },
  { type: "tool_use", id: "read-a", name: "Read", input: { file_path: "a.md" } },
  { type: "tool_result", tool_use_id: "read-a", content: "RESULT_A" },
  { type: "tool_use", id: "read-b", name: "Read", input: { file_path: "b.md" } },
  { type: "tool_result", tool_use_id: "read-b", content: "RESULT_B" },
];

function view(mode: AssistantContentEnvironment["mode"], live: boolean) {
  return <I18nProvider><AssistantToolRuns
    activity={{ ...activity, showCursor: live }}
    environment={{ mode, hiddenToolNames: [], canRespondToPermissions: false }}
    permissions={permissions}
    projection={{ content, streamingIndexes: new Set() }}
    responseResumed={!live}
  /></I18nProvider>;
}

describe("Assistant process disclosure", () => {
  it.each(["dm_live", "room_thread", "room_thread_process"] as const)(
    "%s opens only the directory and preserves explicit child choices on updates", (mode) => {
      const { container, rerender } = render(view(mode, true));
      const group = container.querySelector('[data-tool-run-id] button[aria-expanded]')!;
      expect(group.getAttribute("aria-expanded")).toBe("false");
      expect(container.querySelector('[data-tool-run-detail-list]')).toBeNull();
      fireEvent.click(group);
      const list = container.querySelector('[data-tool-run-detail-list]')!;
      const children = list.querySelectorAll('[data-activity-row]');
      expect(children.length).toBe(3);
      expect(children[0].getAttribute("aria-expanded")).toBe("false");
      expect(children[1].getAttribute("data-tool-block-layout")).toBe("collapsed");
      expect(children[2].getAttribute("data-tool-block-layout")).toBe("collapsed");
      expect(list.querySelector('[data-tool-block-details]')).toBeNull();
      fireEvent.click(children[1]);
      expect(list.textContent).toContain("RESULT_A");
      expect(list.textContent).not.toContain("RESULT_B");
      expect(children[0].getAttribute("aria-expanded")).toBe("false");
      expect(children[2].getAttribute("data-tool-block-layout")).toBe("collapsed");
      rerender(view(mode, false));
      expect(container.querySelectorAll('[data-tool-block-details]').length).toBe(1);
      fireEvent.click(group);
      fireEvent.click(group);
      expect(container.querySelector('[data-tool-block-details]')).toBeNull();
    },
  );

  it("isolates disclosure targets when the same process appears in two surfaces", () => {
    const { container } = render(<>{view("dm_live", false)}{view("room_thread", false)}</>);
    const toggles = Array.from(container.querySelectorAll('[data-tool-run-id] button[aria-expanded]'));
    expect(toggles).toHaveLength(2);
    expect(toggles[0].getAttribute("aria-controls")).not.toBe(toggles[1].getAttribute("aria-controls"));
    toggles.forEach((toggle) => fireEvent.click(toggle));
    toggles.forEach((toggle) => expect(document.getElementById(toggle.getAttribute("aria-controls")!)).toBeTruthy());
  });

  it("Thought stays collapsed during streaming and preserves manual state at completion", () => {
    const thought = (streaming: boolean) => <I18nProvider><ThinkingBlock thinking="Inspect the source" isStreaming={streaming} /></I18nProvider>;
    const { container, rerender } = render(thought(false));
    const toggle = container.querySelector('button[aria-expanded]')!;
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    rerender(thought(true));
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(toggle);
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    rerender(thought(false));
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    fireEvent.click(toggle);
    rerender(thought(true));
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
  });
});
