// INPUT: 实际详情页、双语 RichMail 说明、能力预览和隔离的工具目录快照。
// OUTPUT: 资源状态、原始能力内容、授权事实与弹窗身份不被视觉整理改写。
// POS: 详情组合回归；不请求 MCP、不执行连接或认证。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { getResourceFailure } from "@/lib/error-message";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import type { ConnectorDetail } from "@/types/capability/connector";
import { getConnectorState } from "../model/connector-state-model";

import { ConnectorDetailContent } from "./connector-detail-content";
import { ConnectorDetailView } from "./connector-detail-view";

vi.mock("./use-connector-mcp-tools", () => ({ useConnectorMCPTools: () => ({
  catalog: null, failure: null, loading: false, refresh: vi.fn(), supported: false,
}) }));

const SCOPE = `mail:${"account/".repeat(28)}read`;
const DETAIL: ConnectorDetail = {
  auth_type: "local_pairing", category: "productivity", connection_state: "disconnected",
  connector_id: "richmail", description: "Manage mail and schedules.", icon: "richmail",
  is_configured: true, kind: "connector", name: "richmail", scopes: [SCOPE], status: "available", title: "RichMail",
  docs_url: "https://example.test/richmail/docs", mcp_server_url: `https://example.test/${"mail/".repeat(36)}mcp`,
  features: ["Mail"], feature_details: [{ name: "Mail", description: "Read the selected mailbox.", items: ["Read mail", "Find messages"], scopes: [SCOPE] }],
};

function actions(): ComponentProps<typeof ConnectorDetailView> {
  return { busy: false, detail: DETAIL, failure: null, loading: false, onBack: vi.fn(),
    onConfigureCredential: vi.fn(), onConfigureOauthClient: vi.fn(), onConnect: vi.fn(),
    onDisconnect: vi.fn(), onReplaceOauthClient: vi.fn(), onRetry: vi.fn() };
}

beforeEach(() => localStorage.setItem(LOCALE_STORAGE_KEY, "en"));

describe("Connector detail surfaces", () => {
  it("keeps loading, failed and missing views distinct with explicit retry and return", async () => {
    const user = userEvent.setup();
    const props = actions();
    const view = render(<ConnectorDetailView {...props} detail={null} loading />,
      { wrapper: I18nProvider });
    expect(screen.getByRole("status").getAttribute("aria-busy")).toBe("true");
    expect(screen.getByRole("heading", { name: "Loading connector details…" })).toBeTruthy();
    view.rerender(<ConnectorDetailView {...props} detail={null} failure={getResourceFailure(new Error("offline"), "Failed")} />);
    expect(screen.getByRole("status").getAttribute("data-resource-state")).toBe("error");
    expect(props.onRetry).not.toHaveBeenCalled();
    await user.click(within(screen.getByRole("status")).getByRole("button"));
    expect(props.onRetry).toHaveBeenCalledOnce();
    view.rerender(<ConnectorDetailView {...props} detail={null} />);
    expect(screen.getByRole("status").getAttribute("data-resource-state")).toBe("empty");
    await user.click(screen.getByRole("button", { name: "Back to connectors" }));
    expect(props.onBack).toHaveBeenCalledOnce();
    expect(props.onConnect).not.toHaveBeenCalled();
  });

  it("retains detail on refresh failure and isolates its feature dialog across identity changes", async () => {
    const user = userEvent.setup();
    const props = actions();
    const view = render(<ConnectorDetailView {...props} failure={getResourceFailure(new Error("offline"), "Failed")} />,
      { wrapper: I18nProvider });
    expect(screen.getByRole("heading", { name: "RichMail" })).toBeTruthy();
    await user.click(screen.getByText("Mail", { selector: "span" }));
    const dialog = screen.getByRole("dialog", { name: "Mail" });
    expect(document.getElementById(dialog.getAttribute("aria-describedby")!)?.textContent).toBe("RichMail");
    expect(within(dialog).getByText("Read the selected mailbox.")).toBeTruthy();
    expect(within(dialog).getByRole("heading", { name: "Includes" })).toBeTruthy();
    const scopes = within(dialog).getByText("OAuth scopes").closest("summary")!;
    expect(scopes.parentElement?.hasAttribute("open")).toBe(false);
    await user.click(scopes);
    expect(scopes.parentElement?.hasAttribute("open")).toBe(true);
    expect(within(dialog).getByText(SCOPE).tagName).toBe("CODE");
    view.rerender(<ConnectorDetailView {...props} detail={{ ...DETAIL, connector_id: "other", title: "Other connector", features: [], feature_details: [] }} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("heading", { name: "Other connector" })).toBeTruthy();
    expect(props.onConnect).not.toHaveBeenCalled();
  });

  it.each(["zh", "en"] as const)("localizes %s instructions and preserves exact metadata, feature and document targets", async (locale) => {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    const user = userEvent.setup();
    const onSelectFeature = vi.fn();
    const mcpTools = { catalog: null, failure: null, loading: false, refresh: vi.fn(), supported: true };
    const view = render(<ConnectorDetailContent detail={DETAIL} features={DETAIL.feature_details!}
      mcpTools={mcpTools} onSelectFeature={onSelectFeature} state={getConnectorState(DETAIL)} />, { wrapper: I18nProvider });
    expect(screen.getByText(locale === "zh" ? "本机应用配对" : "Local app pairing")).toBeTruthy();
    const note = screen.getByRole("note");
    expect(within(note).getAllByRole("heading")).toHaveLength(1);
    expect(within(note).getByText("设置 → Rwork → 智能体与能力 → 对外 MCP 服务")).toBeTruthy();
    expect(screen.getByText(DETAIL.mcp_server_url!)).toBeTruthy();
    const docs = screen.getByRole("link", { name: locale === "zh" ? "查看文档" : "Documentation" });
    expect(docs.getAttribute("href")).toBe(DETAIL.docs_url);
    expect(docs.getAttribute("rel")).toBe("noopener noreferrer");
    await user.click(screen.getByText("Mail", { selector: "span" }));
    expect(onSelectFeature).toHaveBeenCalledExactlyOnceWith("Mail");
    const connected = { ...DETAIL, connection_state: "connected" as const };
    view.rerender(<ConnectorDetailContent detail={connected} features={[]}
      mcpTools={mcpTools} onSelectFeature={onSelectFeature} state={getConnectorState(connected)} />);
    expect(screen.queryByRole("note")).toBeNull();
    expect(screen.getByText(locale === "zh" ? "已安全保存" : "Stored securely")).toBeTruthy();
    expect(screen.queryByText(/7 天|7 days/)).toBeNull();
  });
});
