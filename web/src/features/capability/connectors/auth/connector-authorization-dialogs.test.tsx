// INPUT: 直接凭证与 OAuth Connector 详情，以及用户填写和提交动作。
// OUTPUT: 证明授权弹窗复用共享排版/表单并只提交完整的业务凭证。
// POS: Connector 授权弹窗 DOM 合同；Device Flow 时序由相邻 poller 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorCredentialDialog } from "./connector-credential-dialog";
import { ConnectorOAuthClientDialog } from "./connector-oauth-client-dialog";

const AMAP_DETAIL = {
  auth_type: "api_key",
  category: "travel",
  connection_state: "disconnected",
  connector_id: "amap",
  description: "查询地点和路线。",
  docs_url: "https://lbs.amap.com/",
  features: [],
  icon: "/icon/connector/amap.svg",
  is_configured: false,
  kind: "connector",
  name: "amap",
  scopes: [],
  status: "available",
  title: "高德地图",
} satisfies ConnectorDetail;

const FEISHU_DETAIL = {
  ...AMAP_DETAIL,
  auth_type: "oauth2",
  category: "productivity",
  connector_id: "feishu-docx",
  description: "访问飞书云文档。",
  name: "feishu-docx",
  oauth_client_configured: false,
  title: "飞书云文档",
} satisfies ConnectorDetail;

describe("Connector authorization dialogs", () => {
  it("keeps direct credential copy semantic and submits the exact trimmed value", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <I18nProvider>
        <ConnectorCredentialDialog
          busy={false}
          detail={AMAP_DETAIL}
          onClose={vi.fn()}
          onSave={onSave}
        />
      </I18nProvider>,
    );

    expect(screen.getByText("粘贴高德开放平台的 Web 服务 Key。").className)
      .toContain("ui-type-supporting");
    await user.type(screen.getByLabelText("API Key*"), "  secret-key  ");
    await user.click(screen.getByRole("button", { name: "连接" }));

    expect(onSave).toHaveBeenCalledWith("amap", "secret-key");
  });

  it("renders OAuth callback data as shared code typography and submits complete fields", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const { container } = render(
      <I18nProvider>
        <ConnectorOAuthClientDialog
          busy={false}
          detail={FEISHU_DETAIL}
          onClose={vi.fn()}
          onDelete={vi.fn()}
          onSave={onSave}
        />
      </I18nProvider>,
    );

    expect(screen.getByText(/先在飞书开放平台应用添加回调地址/).className)
      .toContain("ui-type-supporting");
    expect(container.querySelector("code")?.className).toContain("ui-type-code");

    await user.type(screen.getByLabelText("Client ID*"), "client-id");
    await user.type(screen.getByLabelText("Client Secret*"), "client-secret");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(onSave).toHaveBeenCalledWith("feishu-docx", "client-id", "client-secret");
  });
});
