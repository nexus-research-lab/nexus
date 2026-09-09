// INPUT: Reading size drafts below and above the supported range.
// OUTPUT: Range feedback precedes clamping, and changes persist only within bounds.
// POS: Appearance number field interaction regression.
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { useChatTypography, DEFAULT_CHAT_TYPOGRAPHY } from "@/shared/theme/chat-typography";
import { SettingsAppearanceSection } from "./settings-appearance-section";
vi.mock("@/shared/theme/theme-context", () => ({ useTheme: () => ({ theme: "light", setTheme: vi.fn() }), defaultTheme: () => "light" }));
vi.mock("../components/settings-font-picker", () => ({ SettingsFontPicker: () => null }));
vi.mock("@/shared/ui/markdown/markdown-content", () => ({ UiMarkdownContent: () => null }));
it.each([[10, 14], [30, 22]])("warns about %s before clamping to %s on blur", (entered, expected) => {
 useChatTypography.setState({ typography: DEFAULT_CHAT_TYPOGRAPHY });
 render(<SettingsAppearanceSection />, { wrapper: I18nProvider });
 const input = screen.getAllByRole("spinbutton")[0] as HTMLInputElement;
 const hint = document.getElementById(input.getAttribute("aria-describedby")!)!;
 expect(hint.textContent).toContain("14"); expect(hint.textContent).toContain("22");
 fireEvent.change(input, { target: { value: String(entered) } });
 expect(input.value).toBe(String(entered));
 expect(input.getAttribute("aria-invalid")).toBe("true");
 expect(useChatTypography.getState().typography.fontSize).toBe(16);
 const warning = hint.textContent;
 fireEvent.blur(input);
 expect(input.value).toBe(String(expected));
 expect(useChatTypography.getState().typography.fontSize).toBe(expected);
 expect(input.getAttribute("aria-invalid")).toBe("false");
 expect(hint.textContent).not.toBe(warning);
 expect(hint.textContent).toContain(String(expected));
});
