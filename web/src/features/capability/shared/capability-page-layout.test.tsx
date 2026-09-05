// INPUT: 能力页标题、说明、动作、移动页头目标、内容模式、目录分区与详情内容。
// OUTPUT: 证明 Header 动作、模式标签、Typography、身份框与详情分栏语义由公共布局持有。
// POS: 能力页共享布局 DOM 合同；各目录资源与筛选行为由所属领域测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { PageHeaderActionsContext } from "@/shared/lib/react/page-header-actions-context";

import {
  CapabilityDetailPage,
  CapabilityDetailIdentity,
  CapabilityDetailSectionHeader,
  CapabilityDetailSplitLayout,
  CapabilityItemIcon,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "./capability-page-layout";

describe("CapabilityPageLayout", () => {
  it("renders desktop identity and actions through the shared Header", () => {
    render(
      <CapabilityPageLayout
        actions={<button type="button">Create</button>}
        description="Manage connected tools."
        title="Connectors"
      >
        <div>Catalog</div>
      </CapabilityPageLayout>,
    );

    expect(screen.getByRole("heading", { name: "Connectors" }).className).toContain("ui-type-page-title");
    const descriptions = screen.getAllByText("Manage connected tools.");
    expect(descriptions.some((node) => node.className.includes("ui-type-metadata"))).toBe(true);
    expect(descriptions.some((node) => node.className.includes("ui-type-supporting"))).toBe(true);
    expect(screen.getByRole("button", { name: "Create" })).toBeTruthy();
    expect(screen.getByText("Catalog")).toBeTruthy();
  });

  it("projects the same page action into the mobile app Header target", () => {
    const target = document.createElement("div");
    document.body.appendChild(target);

    render(
      <PageHeaderActionsContext.Provider value={target}>
        <CapabilityPageLayout
          actions={<button type="button">Add MCP</button>}
          description="Manage MCP servers."
          title="Connectors"
        >
          <div>Catalog</div>
        </CapabilityPageLayout>
      </PageHeaderActionsContext.Provider>,
    );

    expect(target.querySelector("button")?.textContent).toBe("Add MCP");
    expect(document.querySelector(".workspace-content-header button")).toBeNull();
  });

  it("moves the action with the host target and restores its inline owner when the target disappears", () => {
    const firstTarget = document.createElement("div");
    const nextTarget = document.createElement("div");
    let actions = 0;
    const view = (target: HTMLElement | null) => (
      <PageHeaderActionsContext.Provider value={target}>
        <CapabilityPageLayout
          actions={<button onClick={() => { actions += 1; }} type="button">Add connector</button>}
          title="Connectors"
        >
          <div>Catalog</div>
        </CapabilityPageLayout>
      </PageHeaderActionsContext.Provider>
    );
    const { container, rerender, unmount } = render(view(firstTarget));
    expect(firstTarget.querySelector("button")?.textContent).toBe("Add connector");
    expect(container.querySelector("button")).toBeNull();
    fireEvent.click(firstTarget.querySelector("button")!);
    expect(actions).toBe(1);

    rerender(view(nextTarget));
    expect(firstTarget.childElementCount).toBe(0);
    expect(nextTarget.querySelector("button")?.textContent).toBe("Add connector");
    fireEvent.click(nextTarget.querySelector("button")!);
    expect(actions).toBe(2);

    rerender(view(null));
    expect(nextTarget.childElementCount).toBe(0);
    fireEvent.click(screen.getByRole("button", { name: "Add connector" }));
    expect(actions).toBe(3);
    unmount();
    expect(firstTarget.childElementCount).toBe(0);
    expect(nextTarget.childElementCount).toBe(0);
  });

  it("owns one detail page axis and current-object navigation contract", () => {
    let backCount = 0;
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "en", setLocale: () => undefined, t: (key) => key }}
      >
        <CapabilityDetailPage
          backLabel="Skills"
          currentTitle="Research Skill"
          onBack={() => { backCount += 1; }}
        >
          <div>Detail content</div>
        </CapabilityDetailPage>
      </I18N_CONTEXT.Provider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Skills" }));

    expect(backCount).toBe(1);
    expect(container.querySelector("[data-slot='capability-detail-page']")).toBeTruthy();
    expect(container.querySelector("[data-slot='capability-detail-header']")).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "common.location_aria" })).toBeTruthy();
    expect(container.querySelector(".workspace-content-header")?.className)
      .toContain("max-[559px]:hidden");
    expect(container.querySelector(".workspace-content-header")?.className)
      .not.toContain("lg:block");
    expect(container.querySelector("[data-slot='capability-detail-body']")?.className)
      .toContain("pt-5");
    expect(screen.getByText("Research Skill").className).toContain("ui-type-metadata");
    expect(screen.getByText("Research Skill").getAttribute("aria-current")).toBe("page");
    expect(screen.getByText("Detail content")).toBeTruthy();
  });

  it("owns detail identity alignment, semantics, metadata, and actions", () => {
    let actionCount = 0;
    const { container } = render(
      <CapabilityDetailIdentity
        actions={(
          <button onClick={() => { actionCount += 1; }} type="button">
            Connect
          </button>
        )}
        description="Manage mail and schedules."
        leading={<span>RM</span>}
        title="RichMail"
        titleMeta={<span>Connected</span>}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Connect" }));

    expect(actionCount).toBe(1);
    expect(container.querySelector("[data-slot='capability-detail-identity']")).toBeTruthy();
    expect(container.querySelector("[data-slot='capability-detail-identity-leading']")?.textContent)
      .toBe("RM");
    expect(container.querySelector("[data-slot='capability-detail-identity-actions']")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "RichMail" }).className)
      .toContain("ui-type-object-title");
    expect(screen.getByText("Manage mail and schedules.").className)
      .toContain("ui-type-supporting");
    expect(screen.getByText("Connected")).toBeTruthy();
  });

  it("uses semantic section typography and icon geometry", () => {
    const { container } = render(
      <>
        <CapabilitySectionHeader
          count={4}
          description="Available now"
          title="Messaging"
        />
        <CapabilityItemIcon size="sm">M</CapabilityItemIcon>
      </>,
    );

    expect(screen.getByRole("heading", { name: "Messaging" }).className).toContain("ui-type-section-title");
    expect(screen.getByText("Available now").className).toContain("ui-type-metadata");
    expect(screen.getByText("4").className).toContain("ui-type-caption");
    expect(container.querySelector(".radius-control-sm")).toBeTruthy();
  });

  it("owns the responsive detail reading column and configuration rail", () => {
    const { container } = render(
      <CapabilityDetailSplitLayout
        aside={(
          <CapabilityDetailSectionHeader
            description="Per-agent configuration"
            meta="2/4 enabled"
            title="Availability"
          />
        )}
        header={<div>Identity</div>}
      >
        <div>Instructions</div>
      </CapabilityDetailSplitLayout>,
    );

    const layout = container.querySelector("[data-slot='capability-detail-layout']");
    const aside = container.querySelector("[data-slot='capability-detail-aside']");
    const main = container.querySelector("[data-slot='capability-detail-main']");

    expect(layout?.className).toContain("max-w-[1180px]");
    expect(aside?.className).toContain("xl:col-start-2");
    expect(main?.className).toContain("xl:col-start-1");
    expect(aside?.compareDocumentPosition(main as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(screen.getByRole("heading", { name: "Availability" }).className)
      .toContain("ui-type-section-title");
    expect(screen.getByText("Per-agent configuration").className)
      .toContain("ui-type-supporting");
    expect(screen.getByText("2/4 enabled").className)
      .toContain("ui-type-caption");
  });
});
