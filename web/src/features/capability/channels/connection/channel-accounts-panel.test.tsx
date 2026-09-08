// INPUT: 已连接频道账号及删除命令。
// OUTPUT: 证明账号表面、身份、错误和删除等待态复用共享视觉合同并保持删除行为。
// POS: Channel 账号列表 DOM 合同；删除互斥与未知结果对账由控制器测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ChannelAccountView } from "@/lib/api/capability/channel-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ChannelAccountsPanel } from "./channel-accounts-panel";

const ACCOUNT = {
  account_id: "account-1",
  created_at: "2026-09-03T10:00:00Z",
  last_error: "provider detail must stay hidden",
  status: "error",
  updated_at: "2026-09-04T10:00:00Z",
  user_id: "user@example.com",
} satisfies ChannelAccountView;

function renderPanel(
  deletingAccountId = "",
  onDelete = vi.fn(),
) {
  return render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <ChannelAccountsPanel
        accounts={[ACCOUNT]}
        busy={false}
        deletingAccountId={deletingAccountId}
        onDelete={onDelete}
      />
    </I18N_CONTEXT.Provider>,
  );
}

describe("ChannelAccountsPanel", () => {
  it("renders managed account identity and safe error copy through shared owners", () => {
    const { container } = renderPanel(ACCOUNT.account_id);

    expect(screen.getByRole("heading", { name: "已连接账号" }).className)
      .toContain("ui-type-control");
    expect(screen.getByText(ACCOUNT.user_id).className).toContain("ui-type-code");
    expect(container.querySelectorAll("section.surface-radius-sm")).toHaveLength(2);
    const accountFailure = screen.getByRole("status");
    expect(accountFailure.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(accountFailure.getAttribute("data-inline-notice-width")).toBe("full");
    expect(screen.getByText("capability.channel_account_error_title").className)
      .toContain("ui-type-metadata");
    expect(screen.queryByText(ACCOUNT.last_error)).toBeNull();
    expect(screen.getByRole("button", { name: "删除该账号" }).querySelector("svg")?.classList)
      .toContain("motion-reduce:animate-none");
  });

  it("dispatches deletion for the exact account", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    renderPanel("", onDelete);

    await user.click(screen.getByRole("button", { name: "删除该账号" }));
    expect(onDelete).toHaveBeenCalledWith(ACCOUNT);
  });
});
