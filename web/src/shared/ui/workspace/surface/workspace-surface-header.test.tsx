// INPUT: Workspace Header 的身份、标题、副标题、标签与动作。
// OUTPUT: 证明核心 Header 复用语义排版/形状并保留标签切换行为。
// POS: Workspace Header DOM 行为测试；业务导航和动作事务由消费者负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { WorkspaceSurfaceHeader } from "./workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "./workspace-surface-toolbar-action";

describe("WorkspaceSurfaceHeader", () => {
  it("keeps identity, typography and tab behavior under shared owners", () => {
    const onChangeTab = vi.fn();
    const { container } = render(
      <I18nProvider>
        <WorkspaceSurfaceHeader
          activeTab="files"
          leading={<span aria-hidden="true">N</span>}
          leadingVariant="identity"
          onChangeTab={onChangeTab}
          subtitle="资料与工具"
          tabs={[
            { key: "files", label: "文件" },
            { key: "activity", label: "动态" },
          ]}
          title="工作区"
          trailing={(
            <WorkspaceSurfaceToolbarAction onClick={() => undefined}>
              新建
            </WorkspaceSurfaceToolbarAction>
          )}
        />
      </I18nProvider>,
    );

    expect(screen.getByText("工作区").className).toContain("ui-type-page-title");
    expect(screen.getByText("资料与工具").className).toContain("ui-type-metadata");
    expect(screen.getByRole("button", { name: "新建" }).className).toContain("ui-type-caption");
    expect(container.querySelector(".workspace-surface-header-identity-avatar")?.className)
      .toContain("radius-control-md");

    fireEvent.click(screen.getByRole("button", { name: "动态" }));
    expect(onChangeTab).toHaveBeenCalledWith("activity");
  });

  it("renders compact navigation through the shared button owner", () => {
    const { container } = render(
      <I18nProvider>
        <WorkspaceSurfaceHeader
          activeTab="files"
          compactTabsLabel="工作区导航"
          onChangeTab={() => undefined}
          tabs={[
            { key: "files", label: "文件" },
            { key: "activity", label: "动态" },
          ]}
        />
      </I18nProvider>,
    );

    const compactTrigger = screen.getByRole("button", { name: "工作区导航" });
    expect(compactTrigger.className).toContain("ui-type-caption");
    expect(compactTrigger.closest(".workspace-surface-header-compact-tabs"))
      .toBe(container.querySelector(".workspace-surface-header-compact-tabs"));
  });
});
