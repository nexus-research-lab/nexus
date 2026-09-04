// INPUT: Agent 模型选择、不可用选择、Provider 目录失败与恢复动作。
// OUTPUT: 证明模型反馈复用共享 notice，并继续隐藏原始错误、保留当前选择和显式恢复行为。
// POS: Agent 身份模型选择器 DOM 合同；Provider 读取与保存由外层控制器负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { IdentityModelSelector } from "./identity-model-selector";

function renderSelector({
  error = null as string | null,
  model = "",
  onModelChange = vi.fn(),
  onProviderChange = vi.fn(),
  provider = "",
} = {}) {
  return {
    ...render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <IdentityModelSelector
          defaultModel="default-model"
          defaultProvider="default-provider"
          error={error}
          loading={false}
          model={model}
          onModelChange={onModelChange}
          onProviderChange={onProviderChange}
          options={[]}
          provider={provider}
          variant="dialog"
        />
      </I18N_CONTEXT.Provider>,
    ),
    onModelChange,
    onProviderChange,
  };
}

describe("IdentityModelSelector", () => {
  it("uses a full-width shared warning notice for an unavailable selection", async () => {
    const user = userEvent.setup();
    const { onModelChange, onProviderChange } = renderSelector({
      model: "missing-model",
      provider: "missing-provider",
    });

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(notice.getAttribute("data-inline-notice-width")).toBe("full");
    expect(notice.textContent).toContain(
      "agent_options.identity.model_temporarily_unavailable",
    );

    await user.click(screen.getByRole("button", {
      name: "agent_options.identity.use_default_model",
    }));
    expect(onProviderChange).toHaveBeenCalledWith("");
    expect(onModelChange).toHaveBeenCalledWith("");
  });

  it("uses a shared danger notice with safe copy for Provider read failure", () => {
    renderSelector({ error: "raw provider failure must stay hidden" });

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(notice.textContent).toContain(
      "agent_options.identity.provider_load_failed_impact",
    );
    expect(notice.textContent).toContain(
      "agent_options.identity.provider_load_failed_next_step",
    );
    expect(screen.queryByText("raw provider failure must stay hidden")).toBeNull();
  });
});
