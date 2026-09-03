// INPUT: 能力页标题、说明、动作、移动页头目标和共享分区内容。
// OUTPUT: 证明 Header 动作投影、Typography 与身份框语义由公共布局持有。
// POS: 能力页共享布局 DOM 合同；各目录资源与筛选行为由所属领域测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { MobileAppPageHeaderActionsProvider } from "@/app/layout/mobile-app-page-header-actions";

import {
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
      <MobileAppPageHeaderActionsProvider target={target}>
        <CapabilityPageLayout
          actions={<button type="button">Add MCP</button>}
          description="Manage MCP servers."
          title="Connectors"
        >
          <div>Catalog</div>
        </CapabilityPageLayout>
      </MobileAppPageHeaderActionsProvider>,
    );

    expect(target.querySelector("button")?.textContent).toBe("Add MCP");
    expect(document.querySelector(".workspace-content-header button")).toBeNull();
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
});
