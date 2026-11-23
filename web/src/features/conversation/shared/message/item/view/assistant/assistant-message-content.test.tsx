/**
 * INPUT: live DM 的过程区、final surface（Widget / 叙述）与运行中活动状态。
 * OUTPUT: 证明尾部活动行只有一个 owner，且始终位于整条回复最底部。
 * POS: AssistantMessageContent 编排回归；不 mock 过程组或内容渲染器。
 */
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ContentBlock } from "@/types/conversation/message/content";

import { AssistantMessageContent } from "./assistant-message-content";
import type {
  AssistantActivityState,
  AssistantFinalState,
  AssistantProcessState,
} from "./assistant-message-model";

const activity: AssistantActivityState = {
  emptyStreamStatus: null,
  label: null,
  showCursor: true,
  standalone: false,
  state: "thinking",
  toolUseSummary: null,
};

const directContent: ContentBlock[] = [
  { type: "thinking", thinking: "Check the SVG" },
  { type: "tool_use", id: "bash-a", name: "Bash", input: { command: "grep -c feather a.svg" } },
  { type: "tool_result", tool_use_id: "bash-a", content: "12" },
];

const process = {
  anchorRef: { current: null },
  expanded: false,
  projection: { content: [], streamingIndexes: new Set<number>() },
  summary: null,
  toggle: () => {},
  visible: false,
} as unknown as AssistantProcessState;

function renderContent(final: Partial<AssistantFinalState>) {
  return render(
    <I18nProvider>
      <AssistantMessageContent
        activity={activity}
        direct={{ projection: { content: directContent, streamingIndexes: new Set() }, visible: true }}
        environment={{ mode: "dm_live", hiddenToolNames: [], canRespondToPermissions: false }}
        final={{
          content: null,
          isStreaming: false,
          mentions: [],
          streamingIndexes: new Set(),
          visible: false,
          ...final,
        }}
        permissions={{ all: [], matchedByToolUseId: new Map(), owner: "composer", unmatched: [] }}
        process={process}
        showMaxTokensWarning={false}
      />
    </I18nProvider>,
  );
}

function activityRows(container: HTMLElement): Element[] {
  return Array.from(container.querySelectorAll("[data-message-activity-icon]"));
}

describe("AssistantMessageContent tail activity", () => {
  it("keeps the activity row under the process group while it is the tail", () => {
    const { container } = renderContent({});
    const rows = activityRows(container);
    expect(rows).toHaveLength(1);
    expect(rows[0].closest("[data-tool-run-id]")).not.toBeNull();
  });

  it("moves the single activity row below final content that is not streaming text", () => {
    const { container } = renderContent({
      // 现场形态：final 里是仍在生成的 Widget 与一句已写完的叙述，二者都不处于流式正文。
      content: [{ type: "text", text: "校验干净。渲染羽翼版：" }],
      visible: true,
    });
    const rows = activityRows(container);
    expect(rows).toHaveLength(1);
    expect(rows[0].closest("[data-tool-run-id]")).toBeNull();
    const narrative = Array.from(container.querySelectorAll(".nexus-chat-final-content")).at(-1)!;
    expect(narrative.compareDocumentPosition(rows[0]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(narrative.contains(rows[0])).toBe(false);
  });

  it("adds no tail row while the final reply text itself is streaming", () => {
    const { container } = renderContent({
      content: [{ type: "text", text: "正在写最终回复" }],
      isStreaming: true,
      streamingIndexes: new Set([0]),
      visible: true,
    });
    expect(activityRows(container)).toHaveLength(0);
  });
});
