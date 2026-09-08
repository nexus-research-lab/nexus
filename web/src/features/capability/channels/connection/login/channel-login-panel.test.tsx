// INPUT: 安全的频道登录快照、验证码动作与本地化上下文。
// OUTPUT: 证明扫码身份、验证码、二维码和进度使用共享视觉合同并保持提交行为。
// POS: Channel 登录 DOM 合同；轮询、恢复和命令互斥由控制器/model 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ChannelLoginView } from "@/lib/api/capability/channel-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ChannelLoginPanel } from "./channel-login-panel";

const LOGIN = {
  channel_type: "telegram",
  login_id: "hidden-login-id",
  qr_payload: "data:image/png;base64,AAAA",
  started_at: "2026-09-04T10:00:00Z",
  status: "verify_code_required",
  updated_at: "2026-09-04T10:01:00Z",
  user_id: "channel-user",
} satisfies ChannelLoginView;

describe("ChannelLoginPanel", () => {
  it("renders semantic login chrome and submits a verification code", async () => {
    const user = userEvent.setup();
    const onSubmitVerifyCode = vi.fn().mockResolvedValue(true);
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <ChannelLoginPanel
          channelTitle="Telegram"
          channelType="telegram"
          loading={false}
          loginView={LOGIN}
          mutationBlocked={false}
          onSubmitVerifyCode={onSubmitVerifyCode}
          recoveryNotice={null}
        />
      </I18N_CONTEXT.Provider>,
    );

    expect(screen.getByRole("heading", { name: "扫码连接" }).className)
      .toContain("ui-type-control");
    expect(screen.getByText(LOGIN.user_id).className).toContain("ui-type-code");
    expect(screen.getByRole("img", { name: "频道扫码登录二维码" }).className)
      .toContain("surface-radius-sm");
    expect(container.querySelectorAll("section.surface-radius-sm").length).toBeGreaterThan(1);

    await user.type(screen.getByPlaceholderText("验证码"), "648201");
    await user.click(screen.getByRole("button", { name: "提交" }));
    expect(onSubmitVerifyCode).toHaveBeenCalledWith("648201");
    expect((screen.getByPlaceholderText("验证码") as HTMLInputElement).value).toBe("");
  });
});
