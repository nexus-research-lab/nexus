// INPUT: 可连接的 Connector 详情、状态投影与动作回调。
// OUTPUT: 证明对象身份和主动作仍使用共享 Typography 与 Button 并正确派发。
// POS: Connector 对象 Header DOM 合同；统一详情导航由 capability/shared 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ConnectorDetail } from "@/types/capability/connector";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import { getConnectorState } from "../model/connector-state-model";

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

function callbacks() {
  return { onConnect: vi.fn(), onDisconnect: vi.fn(), onConfigureCredential: vi.fn(),
    onConfigureOauthClient: vi.fn(), onReplaceOauthClient: vi.fn() };
}

beforeEach(() => localStorage.setItem(LOCALE_STORAGE_KEY, "zh"));

describe("ConnectorDetailHeader", () => {
  it("renders semantic identity and dispatches its projected primary action", async () => {
    const user = userEvent.setup();
    const onConnect = vi.fn();
    const { container } = render(
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
      { wrapper: I18nProvider },
    );

    expect(screen.getByRole("heading", { name: DETAIL.title }).className)
      .toContain("ui-type-object-title");
    expect(screen.getByText(DETAIL.description).className)
      .toContain("ui-type-supporting");
    expect(container.querySelector("[data-slot='capability-detail-identity']")).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "添加到 Nexus" }));
    expect(onConnect).toHaveBeenCalledWith(DETAIL.connector_id);
  });

  it.each(["zh", "en"] as const)("preserves exact %s command targets and busy locks for each action", async (locale) => {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    const user = userEvent.setup();
    const cases: { patch: Partial<ConnectorDetail>; labels: [string, string]; action: keyof ReturnType<typeof callbacks> }[] = [
      { patch: {}, labels: ["添加到 Nexus", "Add to Nexus"], action: "onConnect" },
      { patch: { auth_type: "api_key" }, labels: ["配置凭证", "Configure credentials"], action: "onConfigureCredential" },
      { patch: { connection_state: "connected" }, labels: ["断开连接", "Disconnect"], action: "onDisconnect" },
      { patch: { is_configured: false, oauth_client_config_required: true }, labels: ["配置应用", "Configure app"], action: "onConfigureOauthClient" },
      { patch: { oauth_client_config_required: true, oauth_client_configured: true }, labels: ["配置应用", "Configure app"], action: "onConfigureOauthClient" },
      { patch: { connector_id: "feishu-docx", connection_state: "connected", oauth_client_id: " app-id " },
        labels: ["更换飞书应用", "Replace Feishu app"], action: "onReplaceOauthClient" },
    ];
    for (const item of cases) {
      const detail = { ...DETAIL, ...item.patch };
      const handlers = callbacks();
      const state = getConnectorState(detail);
      const view = render(<ConnectorDetailHeader {...handlers} busy={false} detail={detail} state={state} />,
        { wrapper: I18nProvider });
      const button = screen.getByRole("button", { name: item.labels[locale === "zh" ? 0 : 1] });
      await user.click(button);
      const target = item.action === "onConnect" || item.action === "onDisconnect" ? detail.connector_id : detail;
      expect(handlers[item.action]).toHaveBeenCalledExactlyOnceWith(target);
      for (const [name, action] of Object.entries(handlers)) if (name !== item.action) expect(action).not.toHaveBeenCalled();
      view.rerender(<ConnectorDetailHeader {...handlers} busy detail={detail} state={state} />);
      for (const action of screen.getAllByRole("button")) {
        expect((action as HTMLButtonElement).disabled).toBe(true);
        await user.click(action);
      }
      expect(handlers[item.action]).toHaveBeenCalledOnce();
      view.unmount();
    }
  });

  it("keeps unavailable actions disabled and excludes ineligible app replacement", () => {
    const handlers = callbacks();
    const detail = { ...DETAIL, is_configured: false };
    const view = render(<ConnectorDetailHeader {...handlers} busy={false} detail={detail} state={getConnectorState(detail)} />,
      { wrapper: I18nProvider });
    expect((screen.getByRole("button", { name: "服务尚未配置" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByRole("button", { name: "更换飞书应用" })).toBeNull();
    const planned = { ...DETAIL, status: "coming_soon" as const };
    view.rerender(<ConnectorDetailHeader {...handlers} busy={false} detail={planned} state={getConnectorState(planned)} />);
    expect((screen.getByRole("button", { name: "即将推出" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
