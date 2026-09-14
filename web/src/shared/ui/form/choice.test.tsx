// INPUT: Choice variants, native button/radio events and explicit or inherited disabled states.
// OUTPUT: Controlled selection, no implicit form submission and keyboard/command isolation.
// POS: Shared Choice behavior regression; computed paint and geometry belong to browser tests.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { UiChoiceButton, UiRadioChoice } from "./choice";

describe("Choice controls", () => {
  const variants = ["surface", "picker", "calendar", "icon"] as const;
  for (const scope of ["control", "fieldset"] as const) {
    it.each(variants)(`keeps %s button selection native with ${scope} disabled`, async (variant) => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      const onSubmit = vi.fn();
      const ref = createRef<HTMLButtonElement>();
      function Harness({ disabled = false }: { disabled?: boolean }) {
        const [selected, setSelected] = useState(false);
        return <form onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
          <fieldset disabled={scope === "fieldset" && disabled}>
            <UiChoiceButton active={selected} disabled={scope === "control" && disabled} onClick={() => { setSelected(!selected); onChange(!selected); }} ref={ref} variant={variant}>
              Select option
            </UiChoiceButton>
          </fieldset>
          <button type="button">Next action</button>
        </form>;
      }
      const { rerender } = render(<Harness />);
      const control = screen.getByRole("button", { name: "Select option" });
      expect(ref.current).toBe(control);
      await user.tab();
      expect(document.activeElement).toBe(control);
      await user.keyboard(" ");
      expect(control.getAttribute("aria-pressed")).toBe("true");
      expect(onChange).toHaveBeenCalledExactlyOnceWith(true);
      expect(onSubmit).not.toHaveBeenCalled();
      rerender(<Harness disabled />);
      await user.click(control);
      expect(onChange).toHaveBeenCalledOnce();
      expect(control.getAttribute("aria-pressed")).toBe("true");
      await user.tab();
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Next action" }));
    });
  }

  it.each(["control", "fieldset"])("keeps native radio exclusivity and %s disabled semantics", async (scope) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const ref = createRef<HTMLInputElement>();
    function Harness({ disabled = false }: { disabled?: boolean }) {
      const [value, setValue] = useState("first");
      return <fieldset disabled={scope === "fieldset" && disabled}>
        <legend>Permission scope</legend>
        {["first", "second"].map((option) => <UiRadioChoice
          checked={value === option} disabled={scope === "control" && disabled} key={option}
          name="test-scope" ref={option === "second" ? ref : undefined}
          onChange={() => { setValue(option); onChange(option); }}
        >{option}</UiRadioChoice>)}
      </fieldset>;
    }
    const { rerender } = render(<Harness />);
    const first = screen.getByRole("radio", { name: "first" }) as HTMLInputElement;
    const second = screen.getByRole("radio", { name: "second" }) as HTMLInputElement;
    expect(ref.current).toBe(second);
    await user.click(second);
    expect(second.checked).toBe(true);
    expect(first.checked).toBe(false);
    rerender(<Harness disabled />);
    await user.click(first);
    expect(onChange).toHaveBeenCalledExactlyOnceWith("second");
    expect(second.checked).toBe(true);
  });
});
