// INPUT: Composer 动作权限、精确 Connector、目录失败与 Session 设置失败/读取状态。
// OUTPUT: 验证菜单唯一切换、禁用命令、失败优先级与显式恢复路由。
// POS: Footer DOM 行为回归；使用真实公共菜单和反馈面，不模拟持久化结果。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ComponentProps, type ReactNode, useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ComposerReadResource } from "../../controller/composer-settings-reliability";
import { ComposerFooterActions } from "./composer-footer-actions";
import { ComposerSessionSettingsReliability } from "./composer-session-settings-reliability";
import { makeController } from "./composer-session-settings.test-support";

function Localized({ children }: { children: ReactNode }) {
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>;
}

type ActionProps = ComponentProps<typeof ComposerFooterActions>;
function Actions({ overrides = {} }: { overrides?: Partial<ActionProps> }) {
  const actionButtonRef = useRef<HTMLButtonElement>(null);
  const [isActionMenuOpen, setOpen] = useState(false);
  const [isGoalMode, setGoalMode] = useState(false);
  return <Localized><ComposerFooterActions
    actionButtonRef={actionButtonRef} canCreateGoal canUseLoop canUseWorkGraphDistillations
    isActionMenuOpen={isActionMenuOpen} isGoalCreating={false} isGoalMode={isGoalMode} isPreparingAttachments={false}
    localDirectoriesController={{ available: true, directories: [], failure: null, loading: false, saving: false,
      chooseDirectory: vi.fn(async () => undefined), removeDirectory: vi.fn(async () => undefined), reload: vi.fn() }}
    onActionMenuClose={() => setOpen(false)} onActionMenuToggle={() => setOpen((value) => !value)}
    onAttachmentSelect={vi.fn()} onGoalToggle={setGoalMode} onLoopSelect={vi.fn()}
    onWorkGraphDistillationsSelect={vi.fn()} onLocalDirectorySelect={vi.fn()}
    sessionSettingsController={makeController()} sessionSettingsDisabled={false} {...overrides}
  /></Localized>;
}

describe("Composer Footer actions", () => {
  it("exposes one checked Goal item, closes on activation and respects Goal locks", async () => {
    const user = userEvent.setup();
    const onGoalToggle = vi.fn();
    const { rerender } = render(<Actions overrides={{ onGoalToggle }} />);
    const anchor = screen.getByRole("button", { name: "composer.open_actions" });
    await user.click(anchor);
    const goal = screen.getByRole("menuitemcheckbox", { name: "composer.start_goal", checked: false });
    expect(goal.querySelector("button, input")).toBeNull();
    expect(screen.queryByRole("switch")).toBeNull();
    await user.click(goal);
    expect(onGoalToggle.mock.calls).toEqual([[true]]);
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(anchor);

    rerender(<Actions overrides={{ onGoalToggle, isGoalMode: true }} />);
    await user.click(anchor);
    screen.getByRole("menuitemcheckbox", { name: "composer.start_goal", checked: true }).focus();
    await user.keyboard(" ");
    expect(onGoalToggle.mock.calls).toEqual([[true], [false]]);
    rerender(<Actions overrides={{ onGoalToggle, isGoalCreating: true }} />);
    await user.click(anchor);
    await user.click(screen.getByRole("menuitemcheckbox", { name: "composer.start_goal" }));
    expect(onGoalToggle).toHaveBeenCalledTimes(2);
  });

  it("routes checked Connector items by exact ID without changing Goal", async () => {
    const user = userEvent.setup();
    const controller = makeController({
      enabledConnectorIds: ["calendar-personal"],
      connectors: [{ connector_id: "calendar-personal", kind: "connector", name: "calendar", title: "Calendar",
        description: "", icon: "", category: "work", auth_type: "none", status: "available",
        connection_state: "connected", is_configured: true }],
    });
    const onGoalToggle = vi.fn();
    render(<Actions overrides={{ onGoalToggle, sessionSettingsController: controller }} />);
    await user.click(screen.getByRole("button", { name: "composer.open_actions" }));
    await user.click(screen.getByRole("menuitemcheckbox", { name: /Calendar/, checked: true }));
    expect(controller.toggleConnector).toHaveBeenCalledExactlyOnceWith("calendar-personal");
    expect(onGoalToggle).not.toHaveBeenCalled();
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("blocks a directory mutation after an unknown write while leaving other actions available", async () => {
    const user = userEvent.setup();
    const onLocalDirectorySelect = vi.fn();
    const onAttachmentSelect = vi.fn();
    render(<Actions overrides={{ onLocalDirectorySelect, onAttachmentSelect, localDirectoriesController: {
      available: true, directories: [], loading: false, saving: false,
      failure: { blocksMutation: true, impact: "核对目录", message: "结果未知", nextStep: "重新读取" },
      chooseDirectory: vi.fn(async () => undefined), removeDirectory: vi.fn(async () => undefined), reload: vi.fn(),
    } }} />);
    await user.click(screen.getByRole("button", { name: "composer.open_actions" }));
    await user.click(screen.getByRole("menuitem", { name: "composer.add_local_directory" }));
    expect(onLocalDirectorySelect).not.toHaveBeenCalled();
    await user.click(screen.getByRole("menuitem", { name: "composer.add_attachment" }));
    expect(onAttachmentSelect).toHaveBeenCalledOnce();
  });
});

describe("Composer Session settings recovery", () => {
  it("prioritizes unknown writes in a dialog and clears the failure on close", async () => {
    const user = userEvent.setup();
    const controller = makeController({ busy: true,
      mutationFailure: { title: "结果未知", impact: "先核对设置", blocksRepeat: true, effect: "unknown",
        intent: { fingerprint: "intent-a", sessionKey: "session-a", setting: "model" } },
      settingsReadFailure: { resource: "session_settings", title: "读取失败", impact: "读取设置" },
    });
    render(<Localized><ComposerSessionSettingsReliability controller={controller} /></Localized>);
    expect(screen.getByText("结果未知")).toBeTruthy();
    expect(screen.queryByText("读取失败")).toBeNull();
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "state.reload_check" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "common.close" }));
    expect(controller.dismissMutationFailure).toHaveBeenCalledOnce();
    expect(controller.retrySessionSettings).not.toHaveBeenCalled();
    expect(controller.updateModel).not.toHaveBeenCalled();
  });

  it.each(["session_settings", "providers", "connectors"] as const)("retries only the failed %s read and disables it while loading", async (resource) => {
    const user = userEvent.setup();
    const failure = { resource, title: "目录读取失败", impact: "已有数据保留" };
    const field = { session_settings: "settingsReadFailure", providers: "providerFailure", connectors: "connectorsFailure" } as const;
    const loading = { session_settings: "settingsLoading", providers: "providerOptionsLoading", connectors: "connectorsLoading" } as const;
    const controller = makeController({ [field[resource]]: failure });
    const commands = { session_settings: controller.retrySessionSettings, providers: controller.retryProviderOptions, connectors: controller.retryConnectors };
    const view = (busy: boolean) => <Localized><ComposerSessionSettingsReliability controller={{ ...controller, [loading[resource]]: busy }} /></Localized>;
    const { rerender } = render(view(false));
    await user.click(screen.getByRole("button", { name: "state.retry" }));
    for (const [key, command] of Object.entries(commands)) expect(command).toHaveBeenCalledTimes(key === resource ? 1 : 0);
    rerender(view(true));
    expect((screen.getByRole("button", { name: "state.retry" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("does not offer a no-op retry for a resource outside this controller", () => {
    render(<Localized><ComposerSessionSettingsReliability controller={makeController({
      settingsReadFailure: { resource: "skills" as ComposerReadResource, title: "不可读取", impact: "稍后重开" },
    })} /></Localized>);
    expect(screen.getByText("不可读取")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "state.retry" })).toBeNull();
  });
});
