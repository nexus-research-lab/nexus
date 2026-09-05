// INPUT: 目录卡片主动作、独立次动作与禁用状态。
// OUTPUT: 证明 Article 语义、原生键盘激活及主次命令隔离。
// POS: Catalog Card DOM 合同；覆盖命中与可见焦点另由浏览器验证。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiButton } from "@/shared/ui/button/button";
import { WorkspaceCatalogCard, WorkspaceCatalogGhostAction } from "./workspace-catalog-card";

describe("WorkspaceCatalogCard", () => {
  it("delegates catalog creation to the shared button with safe native form semantics", async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    const onSubmit = vi.fn();
    const view = (disabled: boolean) => <form onSubmit={onSubmit}>
      <WorkspaceCatalogGhostAction disabled={disabled} onClick={onCreate}>Create item</WorkspaceCatalogGhostAction>
    </form>;
    const { rerender } = render(view(false));
    const create = screen.getByRole("button", { name: "Create item" }) as HTMLButtonElement;
    expect(create.type).toBe("button");
    await user.tab();
    await user.keyboard("{Enter}");
    expect(onCreate).toHaveBeenCalledOnce();
    expect(onSubmit).not.toHaveBeenCalled();
    rerender(view(true));
    expect(create.disabled).toBe(true);
    await user.click(create);
    await user.keyboard(" ");
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("keeps keyboard navigation and secondary commands separate", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onDelete = vi.fn();
    render(
      <WorkspaceCatalogCard primaryAction={{ label: "打开目录项", onClick: onOpen }}>
        <h3>目录项</h3>
        <UiButton onClick={onDelete}>删除目录项</UiButton>
      </WorkspaceCatalogCard>,
    );
    const open = screen.getByRole("button", { name: "打开目录项" });
    const remove = screen.getByRole("button", { name: "删除目录项" });
    expect(screen.getByRole("article").getAttribute("tabindex")).toBeNull();
    expect(remove.closest("button")).toBe(remove);
    expect(open.contains(remove)).toBe(false);
    await user.tab();
    expect(document.activeElement).toBe(open);
    await user.keyboard("{Enter}");
    await user.tab();
    expect(document.activeElement).toBe(remove);
    await user.keyboard(" ");
    expect(onOpen).toHaveBeenCalledOnce();
    expect(onDelete).toHaveBeenCalledOnce();
  });

  it("disables only the requested primary command", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    const onRetry = vi.fn();
    render(
      <WorkspaceCatalogCard primaryAction={{ disabled: true, label: "打开", onClick: onOpen }}>
        <UiButton onClick={onRetry}>刷新</UiButton>
      </WorkspaceCatalogCard>,
    );
    await user.click(screen.getByRole("button", { name: "打开" }));
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "刷新" }));
    await user.keyboard("{Enter}");
    expect(onOpen).not.toHaveBeenCalled();
    expect(onRetry).toHaveBeenCalledOnce();
  });
});
