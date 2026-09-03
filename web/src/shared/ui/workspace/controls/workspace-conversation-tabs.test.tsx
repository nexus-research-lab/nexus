// INPUT: 可创建会话的空标签带，以及受控的未完成创建事务。
// OUTPUT: 证明新建动作从加号切换到共享 Spinner、锁定按钮并正确恢复。
// POS: WorkspaceConversationTabs DOM 行为测试；会话持久化和路由归控制器调用方。

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";

describe("WorkspaceConversationTabs", () => {
  it("uses a real busy indicator while creating a conversation", async () => {
    let resolveCreate!: (conversationId: string | null) => void;
    const onCreateConversation = vi.fn(() => new Promise<string | null>((resolve) => {
      resolveCreate = resolve;
    }));

    render(
      <I18nProvider>
        <WorkspaceConversationTabs
          conversationId={null}
          conversations={[]}
          onCreateConversation={onCreateConversation}
          onSelectConversation={vi.fn()}
        />
      </I18nProvider>,
    );

    const navigation = screen.getByRole("navigation");
    const createButton = within(navigation).getByRole("button");
    expect(createButton.querySelector(".lucide-plus")).toBeTruthy();

    fireEvent.click(createButton);
    await waitFor(() => expect(createButton.getAttribute("aria-busy")).toBe("true"));
    expect((createButton as HTMLButtonElement).disabled).toBe(true);
    expect(createButton.querySelector(".lucide-plus")).toBeNull();
    expect(createButton.querySelector(".lucide-loader-circle")?.getAttribute("class"))
      .toContain("motion-reduce:animate-none");

    await act(async () => resolveCreate(null));
    await waitFor(() => expect(createButton.getAttribute("aria-busy")).toBe("false"));
    expect((createButton as HTMLButtonElement).disabled).toBe(false);
    expect(createButton.querySelector(".lucide-plus")).toBeTruthy();
  });
});
