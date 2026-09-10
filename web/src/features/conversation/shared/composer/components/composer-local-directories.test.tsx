// INPUT: Directory read facts, exact path commands and mutation-blocking states.
// OUTPUT: Shared chips keep named removal independent, accurately disabled and recoverable.
// POS: Offline pure-view regression; native selection, authorization and persistence remain controller-owned.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ComposerLocalDirectoriesController } from "../controller/use-composer-local-directories";
import { ComposerLocalDirectories } from "./composer-local-directories";

function controller(overrides: Partial<ComposerLocalDirectoriesController> = {}): ComposerLocalDirectoriesController {
  return { available: true, directories: ["C:\\Workspace\\Notes\\"], loading: false, saving: false, failure: null,
    chooseDirectory: vi.fn(async () => {}), removeDirectory: vi.fn(async () => {}), reload: vi.fn(), ...overrides };
}
function view(state: ComposerLocalDirectoriesController, disabled = false) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key, params) => params?.name ? `${key}:${params.name}` : key }}>
    <ComposerLocalDirectories controller={state} disabled={disabled} />
  </I18N_CONTEXT.Provider>;
}

it("keeps exact path removal and selection as separate commands", async () => {
  const state = controller(); render(view(state));
  const user = userEvent.setup();
  expect(screen.getByRole("group", { name: "composer.local_directories_label" })).toBeTruthy();
  await user.hover(screen.getByText("Notes"));
  expect((await screen.findByRole("tooltip")).textContent).toBe(state.directories[0]);
  await user.click(screen.getByText("Notes"));
  expect(state.removeDirectory).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "composer.remove_local_directory:Notes" }));
  expect(state.removeDirectory).toHaveBeenCalledExactlyOnceWith(state.directories[0]);
  expect(state.chooseDirectory).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "composer.add_local_directory" }));
  expect(state.chooseDirectory).toHaveBeenCalledTimes(1);
});

it.each(["disabled", "saving", "unknown"])("disables directory mutations while %s without disabling a safe reload", async (reason) => {
  const failure = { blocksMutation: reason === "unknown", impact: "Check current directories", message: "diagnostic", nextStep: "read" };
  const state = controller({ saving: reason === "saving", failure });
  render(view(state, reason === "disabled"));
  const user = userEvent.setup();
  const remove = screen.getByRole<HTMLButtonElement>("button", { name: "composer.remove_local_directory:Notes" });
  const add = screen.getByRole<HTMLButtonElement>("button", { name: "composer.add_local_directory" });
  expect(remove.disabled).toBe(true); expect(add.disabled).toBe(true);
  await user.click(remove); await user.click(add);
  expect(state.removeDirectory).not.toHaveBeenCalled(); expect(state.chooseDirectory).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "composer.local_directories_reload" }));
  expect(state.reload).toHaveBeenCalledTimes(1);
  expect(screen.queryByText("diagnostic")).toBeNull();
});

it("hides unavailable/empty scopes and keeps a failed empty read recoverable without an empty group", () => {
  const state = controller({ available: false });
  const rendered = render(view(state));
  expect(rendered.container.textContent).toBe("");
  rendered.rerender(view(controller({ directories: [] })));
  expect(rendered.container.textContent).toBe("");
  rendered.rerender(view(controller({ directories: [], failure: { blocksMutation: false, impact: "Read failed", message: "hidden", nextStep: "reload" } })));
  expect(screen.getByText("Read failed")).toBeTruthy();
  expect(screen.queryByRole("group")).toBeNull();
  expect(screen.getByRole("button", { name: "composer.local_directories_reload" })).toBeTruthy();
});
