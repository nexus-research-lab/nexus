// INPUT: Slash command/model/skill choices activated by pointer or keyboard.
// OUTPUT: One selection per native click; keyboard activation works without mousedown.
// POS: Slash picker interaction regression, independent of overlay geometry.
import { createRef, type ComponentProps } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { SlashCommandPopover } from "./slash-command-popover";

vi.mock("@/shared/ui/overlay/anchored-overlay-layer", () => ({
  useAnchoredOverlayLayer: () => ({ overlayId: "slash", overlayRef: { current: null }, overlayStyle: {}, portalContainer: document.body }),
}));
beforeEach(() => { HTMLElement.prototype.scrollIntoView = vi.fn(); });
type Props = ComponentProps<typeof SlashCommandPopover>;
function setup(mode: Props["mode"]) {
  const select = vi.fn();
  const props: Props = {
    activeIndex: 0, anchorRef: createRef(), mode, status: "ready",
    commands: [{ name: "goal", execution: "host", enabled: true } as Props["commands"][number]],
    modelItems: [{ id: "model", label: "Model" }], modelError: null, modelLoading: false, modelQuery: "", modelSearchRef: createRef(),
    skillItems: [{ name: "research", title: "Research" } as Props["skillItems"][number]], skillError: null, skillLoading: false, skillQuery: "", skillSearchRef: createRef(),
    onClose: vi.fn(), onModelQueryChange: vi.fn(), onModelQueryKeyDown: vi.fn(), onModelRetry: vi.fn(), onSkillQueryChange: vi.fn(), onSkillQueryKeyDown: vi.fn(), onSkillRetry: vi.fn(),
    onSelectCommand: select, onSelectModel: select, onSelectSkill: select,
  };
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><SlashCommandPopover {...props} /></I18N_CONTEXT.Provider>);
  return select;
}

describe("Slash picker activation", () => {
  it.each(["commands", "models", "skills"] as const)("supports %s with keyboard and one pointer activation", async (mode) => {
    const user = userEvent.setup();
    const select = setup(mode);
    const option = screen.getByRole("option");
    option.focus();
    await user.keyboard("{Enter}");
    expect(select).toHaveBeenCalledTimes(1);
    await user.keyboard(" ");
    expect(select).toHaveBeenCalledTimes(2);
    await user.click(option);
    expect(select).toHaveBeenCalledTimes(3);
  });
});
