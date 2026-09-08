// INPUT: Assistant 页脚复制、分支、记忆引用动作与最小统计状态。
// OUTPUT: 证明页脚复用共享微型 IconButton、Popover Typography 并保持动作行为。
// POS: Assistant 页脚 DOM 行为测试；统计值与 Goal 回执格式由纯模型测试负责。

import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { AssistantMessageStats } from "./assistant-message-stats";

describe("AssistantMessageStats", () => {
  it("uses shared micro actions for copy and branch behavior", async () => {
    const onCopy = vi.fn().mockResolvedValue(undefined);
    const onFork = vi.fn().mockResolvedValue(undefined);
    render(
      <I18nProvider>
        <AssistantMessageStats
          copied
          goalCompletionReceipt={null}
          memories={[]}
          onCopy={onCopy}
          onFork={onFork}
          stats={null}
        />
      </I18nProvider>,
    );

    const copy = screen.getByRole("button", { name: /copy reply|复制回答/i });
    const fork = screen.getByRole("button", { name: /branch in new chat|分支到新聊天/i });
    expect(copy.className).toContain("h-5");
    expect(copy.className).toContain("radius-control-xs");
    expect(copy.className).toContain("text-(--success)");
    expect(fork.className).toContain("h-5");
    expect(fork.className).toContain("radius-control-xs");

    fireEvent.click(copy);
    await act(async () => {
      fireEvent.click(fork);
      await Promise.resolve();
    });
    expect(onCopy).toHaveBeenCalledOnce();
    expect(onFork).toHaveBeenCalledOnce();
  });

  it("opens referenced memories from a shared round action and semantic popover text", () => {
    render(
      <I18nProvider>
        <AssistantMessageStats
          copied={false}
          goalCompletionReceipt={null}
          memories={[{
            description: "The user prefers compact controls.",
            name: "ui-preference",
          }]}
          stats={null}
        />
      </I18nProvider>,
    );

    const trigger = screen.getByRole("button", {
      name: /referenced memories|引用的记忆/i,
    });
    expect(trigger.className).toContain("h-6");
    expect(trigger.className).toContain("rounded-full");

    fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog", {
      name: /referenced memories|引用的记忆/i,
    });
    expect(dialog.className).toContain("surface-popover");
    expect(screen.getByRole("heading", {
      name: /referenced memories|引用的记忆/i,
    }).className).toContain("ui-type-metadata");
    expect(screen.getByText("The user prefers compact controls.").parentElement?.className)
      .toContain("ui-type-supporting");

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog", {
      name: /referenced memories|引用的记忆/i,
    })).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });
});
