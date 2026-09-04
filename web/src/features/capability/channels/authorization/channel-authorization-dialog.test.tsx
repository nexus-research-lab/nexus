// INPUT: Channel 授权展示、受控失败与到期事实。
// OUTPUT: 证明授权弹窗通过共享行内提示呈现错误/过期状态，且保留完整恢复文案。
// POS: Channel 授权 DOM 合同；事件校验、写锁与命令受理由 model/presenter 测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ChannelAuthorizationData } from "@/types/generated/protocol";

import { ChannelAuthorizationDialog } from "./channel-authorization-dialog";

const PRESENTATION = {
  account_binding: "owner-channel",
  channel_type: "telegram",
  expires_at: "2099-09-04T10:00:00Z",
  flow_id: "flow-1",
  kind: "verification_code",
  presentation_token: "presentation-token",
  prompt: "输入 Telegram 提供的验证码。",
} satisfies ChannelAuthorizationData;

function renderDialog(
  presentation: ChannelAuthorizationData,
  error: {
    impact: string;
    nextStep: string;
    title: string;
    writeLocked: boolean;
  } | null,
) {
  return render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <ChannelAuthorizationDialog
        busy={false}
        error={error}
        onCancelAuthorization={vi.fn()}
        onClose={vi.fn()}
        onSubmitCode={vi.fn()}
        presentation={presentation}
        writeLocked={error?.writeLocked ?? false}
      />
    </I18N_CONTEXT.Provider>,
  );
}

describe("ChannelAuthorizationDialog", () => {
  it("renders a controlled authorization failure through the shared danger notice", () => {
    renderDialog(PRESENTATION, {
      impact: "本次验证码尚未提交。",
      nextStep: "请检查连接后再试。",
      title: "验证码提交失败",
      writeLocked: false,
    });

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(notice.getAttribute("data-inline-notice-width")).toBe("full");
    expect(screen.getByText("验证码提交失败")).toBeTruthy();
    expect(screen.getByText("本次验证码尚未提交。")).toBeTruthy();
    expect(screen.getByText("请检查连接后再试。")).toBeTruthy();
  });

  it("renders expiry through the same shared warning notice", () => {
    renderDialog({ ...PRESENTATION, expires_at: "2000-01-01T00:00:00Z" }, null);

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(screen.getByText("capability.channel_authorization_expired_title")).toBeTruthy();
    expect(screen.getByText("capability.channel_authorization_expired_impact")).toBeTruthy();
    expect(screen.getByText("capability.channel_authorization_expired_next_step")).toBeTruthy();
  });
});
