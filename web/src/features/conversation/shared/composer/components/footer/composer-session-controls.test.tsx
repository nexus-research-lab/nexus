// INPUT: Session 身份、继承设置、模型目录与菜单用户事件。
// OUTPUT: 验证直接设置菜单不会跨 Session 延续，DM/Room 权限继承和模型命令保持精确。
// POS: Composer Footer 控件 DOM 回归；持久事务仍由控制器合同负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import type { ComposerSessionSettingsController } from "../../controller/use-composer-session-settings";
import { makeController } from "./composer-session-settings.test-support";
import { ComposerSessionControls } from "./composer-session-controls";
import { applySessionModelSelection } from "./composer-session-control-options";


function Controls({ controller, disabled = false }: { controller: ComposerSessionSettingsController; disabled?: boolean }) {
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
    <ComposerSessionControls controller={controller} disabled={disabled} slot="leading" />
    <ComposerSessionControls controller={controller} disabled={disabled} slot="trailing" />
  </I18N_CONTEXT.Provider>;
}

describe("ComposerSessionControls", () => {
  it.each(["model", "permission"])("closes the %s menu when the exact Session changes and does not reopen on return", async (kind) => {
    const user = userEvent.setup();
    const controller = makeController();
    const { rerender } = render(<Controls controller={controller} />);
    await user.click(screen.getByRole("button", { name: `composer.session_${kind}` }));
    expect(screen.getByRole("menu")).toBeTruthy();
    const next = makeController({ target: { ...controller.target!, sessionKey: "session-b" } });
    rerender(<Controls controller={next} />);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(screen.getByRole("button", { name: `composer.session_${kind}` }).getAttribute("aria-expanded")).toBe("false");
    rerender(<Controls controller={controller} />);
    expect(screen.queryByRole("menu")).toBeNull();
    for (const current of [controller, next]) {
      expect(current.updateModel).not.toHaveBeenCalled();
      expect(current.updatePermission).not.toHaveBeenCalled();
    }
  });

  it("retains exact Provider identity, inherited reset and full model labels", async () => {
    const user = userEvent.setup();
    const controller = makeController();
    render(<Controls controller={controller} />);
    const open = () => user.click(screen.getByRole("button", { name: "composer.session_model" }));
    await open();
    await user.click(screen.getByRole("menuitem", { name: "Base Provider" }));
    expect(controller.resetModel).toHaveBeenCalledOnce();
    await open();
    expect(screen.getByText("Other provider with a long name").className).toContain("ui-type-metadata");
    expect(screen.getByText("Other provider with a long name").className).toContain("ui-type-tone-muted");
    expect(screen.getByText("Base with a long name")).toBeTruthy();
    await user.click(screen.getByRole("menuitem", { name: "Base with a long name Other provider with a long name" }));
    expect(controller.updateModel).toHaveBeenCalledWith("other", "base");
    await open();
    await user.click(screen.getByRole("menuitem", { name: "composer.session_reset_defaults" }));
    expect(controller.resetModel).toHaveBeenCalledTimes(2);
  });

  it.each([false, true])("preserves inherited permission semantics with roomWide=%s", async (roomWide) => {
    const user = userEvent.setup();
    const controller = makeController();
    if (roomWide) controller.scope!.targets.push({ agentId: "pixel", name: "Pixel", sessionKey: "pixel-session" });
    render(<Controls controller={controller} />);
    const open = () => user.click(screen.getByRole("button", { name: "composer.session_permission" }));
    await open();
    await user.click(screen.getByRole("menuitem", { name: /agent_options\.advanced\.permission\.default\.label/ }));
    expect(controller.updatePermission).toHaveBeenCalledWith(roomWide ? "default" : "");
    await open();
    await user.click(screen.getByRole("menuitem", { name: /agent_options\.advanced\.permission\.bypass\.label/ }));
    expect(controller.updatePermission).toHaveBeenCalledWith("bypassPermissions");
    await open();
    await user.click(screen.getByRole("menuitem", { name: "composer.session_reset_defaults" }));
    expect(controller.resetPermission).toHaveBeenCalledOnce();
  });

  it.each(["model", "permission"])("closes the %s menu while busy and keeps reset unavailable without an override", async (kind) => {
    const user = userEvent.setup();
    const controller = makeController({ hasModelOverride: false, hasPermissionOverride: false });
    const { rerender } = render(<Controls controller={controller} />);
    const trigger = screen.getByRole("button", { name: `composer.session_${kind}` }) as HTMLButtonElement;
    await user.click(trigger);
    expect((screen.getByRole("menuitem", { name: "composer.session_reset_defaults" }) as HTMLButtonElement).disabled).toBe(true);
    rerender(<Controls controller={{ ...controller, busy: kind === "permission", modelBusy: kind === "model" }} />);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(trigger.disabled).toBe(true);
    rerender(<Controls controller={controller} />);
    expect(screen.queryByRole("menu")).toBeNull();
    await user.click(trigger);
    rerender(<Controls controller={controller} disabled />);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(trigger.disabled).toBe(true);
  });

  it("does not turn malformed model values into settings mutations", () => {
    const controller = makeController();
    for (const value of ["bad json", "null", '["provider"]', '["provider",3]', '["","base"]']) {
      applySessionModelSelection(controller, value);
    }
    expect(controller.updateModel).not.toHaveBeenCalled();
    expect(controller.resetModel).not.toHaveBeenCalled();
  });
});
