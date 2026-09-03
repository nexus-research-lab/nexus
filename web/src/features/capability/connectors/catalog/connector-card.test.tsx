// INPUT: 可用 Connector 的普通与在途目录状态。
// OUTPUT: 证明目录条目保留身份结构并使用共享 Spinner 合同。
// POS: Connector 卡片 DOM 合同；状态决策由 connector-card-model 测试负责。

import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ConnectorInfo } from "@/types/capability/connector";

import { ConnectorCard } from "./connector-card";

const CONNECTOR: ConnectorInfo = {
  auth_type: "oauth2",
  category: "productivity",
  connection_state: "disconnected",
  connector_id: "github",
  description: "Read repositories and issues",
  icon: "github",
  is_configured: true,
  kind: "connector",
  name: "github",
  status: "available",
  title: "GitHub",
};

describe("ConnectorCard", () => {
  it("uses the shared medium Spinner for an in-flight connector action", () => {
    const { container } = render(
      <ConnectorCard
        busy
        connector={CONNECTOR}
        onSelect={vi.fn()}
      />,
    );

    const spinner = container.querySelector("svg.animate-spin");
    expect(spinner?.getAttribute("class")).toContain("h-4 w-4");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });
});
