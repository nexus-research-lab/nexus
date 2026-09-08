// INPUT: 单会话标签的关闭资格、活动/固定状态及独立关闭、选择、固定命令。
// OUTPUT: 证明共享关闭按钮保留原生标题、动作隔离与受控可见性。
// POS: Workspace 标签关闭组合回归；会话集合和持久化由控制器测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { WorkspaceConversationTab } from "./workspace-conversation-tab";

describe("WorkspaceConversationTab close action", () => {
  it("closes through pointer and keyboard without selecting or pinning the conversation", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSelect = vi.fn();
    const onTogglePin = vi.fn();
    const props = {
      canClose: true,
      canPin: true,
      closeLabel: "关闭任务会话",
      conversationId: "conversation-a",
      externalSessionLabel: null,
      isActive: false,
      isPinned: false,
      onClose,
      onSelect,
      onTogglePin,
      pinLabel: "固定任务会话",
      title: "任务会话",
    };
    const { rerender } = render(<WorkspaceConversationTab {...props} />);
    const closeButton = screen.getByRole("button", { name: "关闭任务会话" });
    expect(closeButton.getAttribute("title")).toBe("关闭任务会话");
    expect(closeButton.className).toContain("opacity-0");
    expect(closeButton.className).toContain("group-hover:opacity-100");

    await user.click(closeButton);
    await user.keyboard("{Enter}");
    await user.keyboard(" ");
    expect(onClose).toHaveBeenCalledTimes(3);
    expect(onSelect).not.toHaveBeenCalled();
    expect(onTogglePin).not.toHaveBeenCalled();

    rerender(<WorkspaceConversationTab {...props} isActive />);
    expect(screen.getByRole("button", { name: "关闭任务会话" }).className).toContain("opacity-80");
    rerender(<WorkspaceConversationTab {...props} canClose={false} />);
    expect(screen.queryByRole("button", { name: "关闭任务会话" })).toBeNull();
    expect(screen.getByRole("button", { name: "任务会话" })).toBeTruthy();
  });
});
