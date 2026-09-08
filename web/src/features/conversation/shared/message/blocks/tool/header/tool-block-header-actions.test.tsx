// INPUT: ToolBlock 权限、复制与展开动作资格。
// OUTPUT: 证明权限和复制复用共享微型控件、阻止父行误触并保留禁用原因。
// POS: ToolBlock Header 动作 DOM 行为测试；工具状态投影由 tool-block-model 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { ToolBlockHeaderActions } from "./tool-block-header-actions";

describe("ToolBlockHeaderActions", () => {
  it("uses shared permission and copy controls without triggering the parent row", () => {
    const onAllow = vi.fn();
    const onCopyResult = vi.fn();
    const onDeny = vi.fn();
    const onParentClick = vi.fn();
    render(
      <I18nProvider>
        <div
          onClick={onParentClick}
          onKeyDown={() => undefined}
          role="button"
          tabIndex={0}
        >
          <ToolBlockHeaderActions
            canCopyResult
            canToggle
            copied
            expanded
            interactionDisabled={false}
            onAllow={onAllow}
            onCopyResult={onCopyResult}
            onDeny={onDeny}
            showPermissionActions
          />
        </div>
      </I18nProvider>,
    );

    const deny = screen.getByRole("button", { name: /^(deny|拒绝)$/i });
    const allow = screen.getByRole("button", { name: /^(allow|允许)$/i });
    const copy = screen.getByRole("button", { name: /^(result copied|已复制结果)$/i });
    expect(deny.className).toContain("min-h-6");
    expect(deny.className).toContain("radius-control-xs");
    expect(allow.className).toContain("text-(--brand-action)");
    expect(copy.className).toContain("h-6");
    expect(copy.className).toContain("text-(--success)");

    fireEvent.click(deny);
    fireEvent.click(allow);
    fireEvent.click(copy);
    expect(onDeny).toHaveBeenCalledOnce();
    expect(onAllow).toHaveBeenCalledOnce();
    expect(onCopyResult).toHaveBeenCalledOnce();
    expect(onParentClick).not.toHaveBeenCalled();
  });

  it("keeps permission actions disabled with the supplied reason", () => {
    const onAllow = vi.fn();
    const onDeny = vi.fn();
    render(
      <I18nProvider>
        <ToolBlockHeaderActions
          canCopyResult={false}
          canToggle={false}
          copied={false}
          expanded={false}
          interactionDisabled
          interactionDisabledReason="Another request is active."
          onAllow={onAllow}
          onCopyResult={vi.fn()}
          onDeny={onDeny}
          showPermissionActions
        />
      </I18nProvider>,
    );

    const deny = screen.getByRole("button", { name: /deny|拒绝/i });
    const allow = screen.getByRole("button", { name: /allow|允许/i });
    expect((deny as HTMLButtonElement).disabled).toBe(true);
    expect((allow as HTMLButtonElement).disabled).toBe(true);
    expect(deny.title).toBe("Another request is active.");
    expect(allow.title).toBe("Another request is active.");

    fireEvent.click(deny);
    fireEvent.click(allow);
    expect(onDeny).not.toHaveBeenCalled();
    expect(onAllow).not.toHaveBeenCalled();
  });
});
