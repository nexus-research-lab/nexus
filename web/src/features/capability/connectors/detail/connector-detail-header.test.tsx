// INPUT: 可连接的 Connector 详情、状态投影与动作回调。
// OUTPUT: 证明对象身份和主动作仍使用共享 Typography 与 Button 并正确派发。
// POS: Connector 对象 Header DOM 合同；统一详情导航由 capability/shared 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorDetailHeader } from "./connector-detail-header";

const DETAIL = {
  auth_type: "oauth2",
  category: "productivity",
  connection_state: "disconnected",
  connector_id: "richmail",
  description: "Manage mail and schedules.",
  features: [],
  icon: "/icon/connector/richmail.svg",
  is_configured: true,
  kind: "connector",
  name: "richmail",
  scopes: [],
  status: "available",
  title: "RichMail",
} satisfies ConnectorDetail;

describe("ConnectorDetailHeader", () => {
  it("renders semantic identity and dispatches its projected primary action", async () => {
    const user = userEvent.setup();
    const onConnect = vi.fn();
    render(
      <ConnectorDetailHeader
        busy={false}
        detail={DETAIL}
        onConfigureCredential={vi.fn()}
        onConfigureOauthClient={vi.fn()}
        onConnect={onConnect}
        onDisconnect={vi.fn()}
        onReplaceOauthClient={vi.fn()}
        state={{
          configurationError: null,
          oauthClientAction: null,
          primaryAction: "connect",
          status: "disconnected",
        }}
      />,
    );

    expect(screen.getByRole("heading", { name: DETAIL.title }).className)
      .toContain("ui-type-object-title");
    expect(screen.getByText(DETAIL.description).className)
      .toContain("ui-type-supporting");

    await user.click(screen.getByRole("button", { name: "添加到 Nexus" }));
    expect(onConnect).toHaveBeenCalledWith(DETAIL.connector_id);
  });
});
