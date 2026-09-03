// INPUT: Connector 详情返回动作与当前 Connector 标题。
// OUTPUT: 证明详情面包屑复用共享 Button 与语义 Typography，而非页面私有样式。
// POS: Connector 详情 Header DOM 合同；连接命令与状态投影由 model/controller 测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorDetailBreadcrumb } from "./connector-detail-header";

describe("ConnectorDetailBreadcrumb", () => {
  it("renders the back action and current item through shared semantic owners", () => {
    render(
      <ConnectorDetailBreadcrumb
        detail={{ title: "RichMail" } as ConnectorDetail}
        onBack={vi.fn()}
      />,
    );

    const backButton = screen.getByRole("button", { name: "连接器" });
    expect(backButton.className).toContain("radius-control-sm");
    expect(backButton.className).toContain("ui-type-metadata");
    expect(screen.getByText("RichMail").className).toContain("ui-type-metadata");
  });
});
