// INPUT: Conversation 可靠性快照、只读资源状态和显式刷新动作。
// OUTPUT: 证明两类业务提示复用 UiInlineNotice，且失败映射、数据标记与动作条件不变。
// POS: Conversation reliability 视图合同测试；可靠性状态机本身归 hooks/agent/reliability。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ConversationReliabilityNotice } from "./conversation-reliability-notice";
import { ProviderUnavailableBanner } from "./provider-unavailable-banner";
import { ReadResourceReliabilityNotice } from "./read-resource-reliability-notice";

vi.mock("@/features/onboarding/provider-setup/provider-setup-dialog", () => ({
  ProviderSetupDialog: ({ isOpen }: { isOpen: boolean }) => (
    isOpen ? <div aria-label="Provider setup" role="dialog" /> : null
  ),
}));

const I18N_VALUE = {
  locale: "zh" as const,
  setLocale: vi.fn(),
  t: (key: string) => key,
};

describe("Conversation reliability notices", () => {
  it("maps an unknown delivery result to the shared warning notice and reconcile action", () => {
    const onReconcile = vi.fn();
    render(
      <I18N_CONTEXT.Provider value={I18N_VALUE}>
        <ConversationReliabilityNotice
          compact
          isReconciling={false}
          onReconcile={onReconcile}
          reliability={{
            failure: {
              code: "delivery_unknown",
              session_key: "session-1",
            },
            provider_retry: null,
            transport_phase: "healthy",
          }}
        />
      </I18N_CONTEXT.Provider>,
    );

    const status = screen.getByRole("status");
    expect(status.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(status.getAttribute("data-conversation-failure-code")).toBe("delivery_unknown");
    expect(screen.getByText("conversation.reliability.delivery_unknown")).toBeTruthy();
    expect(screen.getByText("conversation.reliability.delivery_unknown_impact")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "common.refresh" }));
    expect(onReconcile).toHaveBeenCalledOnce();
  });

  it("keeps read-resource identity and stale state on the shared edge notice", () => {
    const onRefresh = vi.fn();
    render(
      <I18N_CONTEXT.Provider value={I18N_VALUE}>
        <ReadResourceReliabilityNotice
          impact="保留上一次成功读取的内容"
          isRefreshing={false}
          onRefresh={onRefresh}
          problem="工作图读取失败"
          resource="execution-workgraph"
          stale
        />
      </I18N_CONTEXT.Provider>,
    );

    const status = screen.getByRole("status", { name: "工作图读取失败" });
    expect(status.getAttribute("data-inline-notice-variant")).toBe("edge");
    expect(status.getAttribute("data-read-resource")).toBe("execution-workgraph");
    expect(status.getAttribute("data-read-resource-state")).toBe("stale");
    expect(screen.getByText("保留上一次成功读取的内容")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "state.reload_check" }));
    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it("uses the same inline notice for Provider recovery and opens setup on demand", () => {
    render(
      <I18N_CONTEXT.Provider value={I18N_VALUE}>
        <ProviderUnavailableBanner compact />
      </I18N_CONTEXT.Provider>,
    );

    const status = screen.getByRole("status");
    expect(status.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(screen.getByText("onboarding.provider_setup_banner")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", {
      name: "onboarding.provider_setup_action",
    }));
    expect(screen.getByRole("dialog", { name: "Provider setup" })).toBeTruthy();
  });
});
