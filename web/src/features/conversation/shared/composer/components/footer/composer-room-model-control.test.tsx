// INPUT: 受控 Room Agent/模型目录、设置命令与用户键盘/指针事件。
// OUTPUT: 证明宽窄级联的焦点、逐层返回、Tab 退出和 exact Agent 模型/重置命令。
// POS: Composer 模型菜单 DOM 回归；使用真实公共菜单/浮层，API 事务由控制器测试负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useCallback, useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import type { ComposerSessionSettingsController } from "../../controller/use-composer-session-settings";
import { ComposerRoomModelControl } from "./composer-room-model-control";

const targets = [
  { agentId: "nova", name: "Nova", sessionKey: "nova-session" },
  { agentId: "pixel", name: "Pixel", sessionKey: "pixel-session" },
];

function setupFixture(width = 1024) {
  vi.stubGlobal("innerWidth", width);
  const commands = {
    ensureTargetsLoaded: vi.fn(async () => undefined),
    resetModel: vi.fn(),
    updateModel: vi.fn(),
  };
  function Harness({ disabled = false, busy = false }: { disabled?: boolean; busy?: boolean }) {
    const [selectedId, setSelectedId] = useState("nova");
    const resetTarget = useCallback(() => setSelectedId("nova"), []);
    const controller: ComposerSessionSettingsController = {
      busy: false,
      connectors: [], connectorsFailure: null, connectorsLoading: false, enabledConnectorIds: [],
      ensureTargetsLoaded: commands.ensureTargetsLoaded,
      hasModelOverride: true, hasPermissionOverride: false,
      inheritedModel: "base", inheritedPermissionMode: "default", inheritedProvider: "provider",
      isDangerousPermission: false,
      modelBusy: busy, modelLabel: "Advanced", permissionLabel: "Default",
      providerOptions: {
        default_provider: null, default_model: null, default_selection: null,
        default_image_provider: null, default_image_model: null, default_image_selection: null,
        background_items: [], image_items: [], vision_items: [],
        items: [{ provider: "provider", display_name: "Provider", models: [
          { model_id: "base", display_name: "Base", is_default: true },
          { model_id: "advanced", display_name: "Advanced", is_default: false },
        ] }],
      },
      providerOptionsLoading: false, providerFailure: null,
      resetTarget, saving: false, scope: undefined, selectTarget: setSelectedId,
      settings: { provider: "provider", model: "advanced", permission_mode: "", connector_ids: null },
      settingsLoading: false, settingsReadFailure: null, mutationFailure: null,
      target: targets.find((target) => target.agentId === selectedId),
      targetViews: targets.map((target) => ({ target, busy: false, modelLabel: "Advanced" })),
      retryConnectors: vi.fn(), retryProviderOptions: vi.fn(), retrySessionSettings: vi.fn(async () => undefined),
      resetModel: async () => { commands.resetModel(selectedId); },
      updateModel: async (provider, model) => { commands.updateModel(selectedId, provider, model); },
      resetPermission: vi.fn(async () => undefined), updatePermission: vi.fn(async () => undefined),
      toggleConnector: vi.fn(async () => undefined),
    };
    return <I18nProvider>
      <button type="button">Before</button>
      <ComposerRoomModelControl controller={controller} disabled={disabled} />
      <button type="button">After</button>
    </I18nProvider>;
  }
  return { commands, Harness };
}

afterEach(() => vi.unstubAllGlobals());

// jsdom 不计算布局；为焦点目录提供可见控件的矩形，不模拟浏览器视觉验收。
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
});
afterEach(() => vi.restoreAllMocks());

describe("ComposerRoomModelControl", () => {
  it.each([390, 1024])("navigates the %spx cascade and updates only the chosen Agent", async (width) => {
    const user = userEvent.setup();
    const { commands, Harness } = setupFixture(width);
    render(<Harness />);
    const trigger = screen.getAllByRole("button").find((button) => button.getAttribute("aria-haspopup") === "dialog")!;
    await user.click(trigger);
    const first = screen.getByRole("menuitem", { name: /Nova/ });
    expect(document.activeElement).toBe(first);
    await user.keyboard("{ArrowDown}");
    const pixel = screen.getByRole("menuitem", { name: /Pixel/ });
    expect(document.activeElement).toBe(pixel);
    fireEvent.keyDown(pixel, { key: "ArrowRight", isComposing: true });
    expect(screen.queryByRole("menuitem", { name: "Base Provider" })).toBeNull();
    await user.keyboard("{ArrowRight}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Base Provider" }));
    await user.keyboard("{ArrowLeft}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: /Pixel/ }));
    await user.keyboard("{Enter}{ArrowDown}{Enter}");
    expect(commands.updateModel).toHaveBeenCalledExactlyOnceWith("pixel", "provider", "advanced");
    expect(commands.resetModel).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    expect(commands.ensureTargetsLoaded).toHaveBeenCalledTimes(1);
  });

  it("closes one level per Escape, preserves IME and dismisses through normal Tab order", async () => {
    const user = userEvent.setup();
    const { Harness } = setupFixture();
    render(<Harness />);
    const trigger = screen.getAllByRole("button").find((button) => button.getAttribute("aria-haspopup") === "dialog")!;
    await user.click(trigger);
    await user.keyboard("{ArrowRight}");
    fireEvent.keyDown(document.activeElement!, { key: "Escape", keyCode: 229 });
    expect(screen.getByRole("menuitem", { name: "Base Provider" })).toBeTruthy();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menuitem", { name: "Base Provider" })).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: /Nova/ }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
    await user.click(trigger);
    await user.keyboard("{ArrowRight}");
    await user.tab();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
    await user.click(trigger);
    await user.tab({ shift: true });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Before" }));
  });

  it("keeps hover focus in the Agent menu, and preserves inherited/reset commands", async () => {
    const user = userEvent.setup();
    const { commands, Harness } = setupFixture();
    render(<Harness />);
    const trigger = screen.getAllByRole("button").find((button) => button.getAttribute("aria-haspopup") === "dialog")!;
    await user.click(trigger);
    const first = screen.getByRole("menuitem", { name: /Nova/ });
    await user.hover(screen.getByRole("menuitem", { name: /Pixel/ }));
    expect(document.activeElement).toBe(first);
    await user.keyboard("{ArrowDown}{ArrowRight}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Base Provider" }));
    const modelMenu = screen.getByRole("menuitem", { name: "Base Provider" }).closest('[role="menu"]')!;
    expect(within(modelMenu as HTMLElement).getAllByRole("menuitem")).toHaveLength(3);
    await user.click(screen.getByRole("menuitem", { name: "Base Provider" }));
    expect(commands.resetModel).toHaveBeenLastCalledWith("pixel");
    await user.click(trigger);
    await user.keyboard("{ArrowRight}{End}{Enter}");
    expect(commands.resetModel).toHaveBeenLastCalledWith("nova");
    expect(commands.updateModel).not.toHaveBeenCalled();
  });

  it("keeps busy model actions disabled and clears an externally disabled menu", async () => {
    const user = userEvent.setup();
    const { commands, Harness } = setupFixture();
    const view = render(<Harness busy />);
    const trigger = screen.getAllByRole("button").find((button) => button.getAttribute("aria-haspopup") === "dialog")!;
    await user.click(trigger);
    await user.keyboard("{ArrowRight}");
    const base = screen.getByRole("menuitem", { name: "Base Provider" }) as HTMLButtonElement;
    expect(base.disabled).toBe(true);
    expect(document.activeElement).toBe(base.closest('[role="menu"]'));
    await user.keyboard("{Enter}");
    expect(commands.updateModel).not.toHaveBeenCalled();
    expect(commands.resetModel).not.toHaveBeenCalled();
    view.rerender(<Harness disabled />);
    expect(screen.queryByRole("dialog")).toBeNull();
    view.rerender(<Harness />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
