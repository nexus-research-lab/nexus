// INPUT: Visibility, generation state and an explicit scrolling command.
// OUTPUT: One named keyboard action, removed when hidden.
// POS: Shared scroll control regression; scrolling geometry belongs to the caller.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ScrollToLatestButton } from "./scroll-to-latest-button";
it("keeps the same named keyboard action while generating and removes it when hidden", async () => {
  const onClick = vi.fn();
  const view = (visible: boolean, isGenerating: boolean) => <I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: key => key}}>
    <ScrollToLatestButton visible={visible} isGenerating={isGenerating} onClick={onClick} />
  </I18N_CONTEXT.Provider>;
  const { rerender } = render(view(true, true));
  const user = userEvent.setup();
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", {name: "room.scroll_to_latest"}));
  await user.keyboard("{Enter}");
  expect(onClick).toHaveBeenCalledOnce();
  rerender(view(true, false));
  expect(screen.getByRole("button", {name: "room.scroll_to_latest"})).toBeTruthy();
  rerender(view(false, false));
  expect(screen.queryByRole("button")).toBeNull();
});
