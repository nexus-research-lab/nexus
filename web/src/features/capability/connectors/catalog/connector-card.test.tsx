// INPUT: 双语 Connector 目录的连接、配置、断开和在途状态。
// OUTPUT: 本地化名称/徽标、精确命令及行内点击/键盘隔离。
// POS: 实际卡片与状态模型组合回归；不执行授权或远程写入。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ConnectorInfo } from "@/types/capability/connector";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";

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
  it.each(["zh", "en"] as const)("localizes %s card states and isolates exact actions from opening the row", async (locale) => {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    const user = userEvent.setup();
    const cases = [
      { connector: CONNECTOR, label: locale === "zh" ? "连接 GitHub" : "Connect GitHub", action: "connect" },
      { connector: { ...CONNECTOR, connection_state: "connected" as const },
        label: locale === "zh" ? "断开 GitHub" : "Disconnect GitHub", action: "disconnect" },
      { connector: { ...CONNECTOR, auth_type: "api_key" as const },
        label: locale === "zh" ? "配置 GitHub" : "Configure GitHub", action: "select" },
      { connector: { ...CONNECTOR, is_configured: false, oauth_client_config_required: true },
        label: locale === "zh" ? "配置 GitHub" : "Configure GitHub", action: "select", badge: locale === "zh" ? "待配置" : "Setup required" },
    ] as const;
    for (const item of cases) {
      const callbacks = { connect: vi.fn(), disconnect: vi.fn(), select: vi.fn() };
      const view = render(<ConnectorCard connector={item.connector} onConnect={callbacks.connect}
        onDisconnect={callbacks.disconnect} onSelect={callbacks.select} />, { wrapper: I18nProvider });
      if ("badge" in item) expect(screen.getByText(item.badge)).toBeTruthy();
      const action = screen.getByRole("button", { name: item.label });
      await user.click(action);
      await user.keyboard("{Enter}");
      expect(callbacks[item.action]).toHaveBeenCalledTimes(2);
      for (const [name, callback] of Object.entries(callbacks)) {
        if (name !== item.action) expect(callback).not.toHaveBeenCalled();
      }
      await user.click(screen.getByText("GitHub"));
      expect(callbacks.select).toHaveBeenCalledTimes(item.action === "select" ? 3 : 1);
      view.unmount();
    }
  });

  it("uses the shared medium Spinner for an in-flight connector action", () => {
    const { container } = render(
      <ConnectorCard
        busy
        connector={CONNECTOR}
        onSelect={vi.fn()}
      />,
      { wrapper: I18nProvider },
    );

    const spinner = container.querySelector("svg.animate-spin");
    expect(spinner?.getAttribute("class")).toContain("h-4 w-4");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });

  it("shows coming-soon status without exposing a connection action", () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, "en");
    render(<ConnectorCard connector={{ ...CONNECTOR, status: "coming_soon" }} onSelect={vi.fn()} />,
      { wrapper: I18nProvider });
    expect(screen.getByText("Coming soon")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Connect GitHub" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Configure GitHub" })).toBeNull();
  });
});
