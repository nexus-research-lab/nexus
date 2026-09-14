// INPUT: Initial/localized Hero text and interactive children of an entering container.
// OUTPUT: Complete first-render content, intact graphemes and preserved child interaction/state.
// POS: Offline content and lifecycle tests; CSS motion policy is checked by the recipe contract.

import { renderToStaticMarkup } from "react-dom/server";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { AnimatedHeroText, FadeSlideIn } from "./animated-hero-text";

it("includes complete accessible text on the first render without a font measurement", () => {
  const text = "你好 👩🏽‍💻 e\u0301";
  const markup = renderToStaticMarkup(<AnimatedHeroText text={text} />);
  const node = document.createElement("div");
  node.innerHTML = markup;
  expect(node.textContent).toBe(text);
  expect(node.firstElementChild?.getAttribute("aria-label")).toBe(text);
  expect([...node.querySelectorAll('[aria-hidden="true"]')].map((element) => element.textContent))
    .toEqual(["你", "好", " ", "👩🏽‍💻", " ", "e\u0301"]);
});

it("updates text immediately and keeps identities for repeated existing graphemes", () => {
  const { container, rerender } = render(<AnimatedHeroText text="好好" />);
  const first = [...container.querySelectorAll('[aria-hidden="true"]')];
  rerender(<AnimatedHeroText text="好好！" />);
  expect(container.textContent).toBe("好好！");
  expect([...container.querySelectorAll('[aria-hidden="true"]')].slice(0, 2)).toEqual(first);
  rerender(<AnimatedHeroText text="Ready" />);
  expect(container.textContent).toBe("Ready");
  expect(container.firstElementChild?.getAttribute("aria-label")).toBe("Ready");
  rerender(<AnimatedHeroText text="" />);
  expect(container.textContent).toBe("");
});

it("keeps children interactive and preserves their input across container updates", async () => {
  const user = userEvent.setup();
  const action = vi.fn();
  const view = (delayMs: number) => <FadeSlideIn delayMs={delayMs} style={{ display: "inline-flex" }}>
    <input aria-label="Question" defaultValue="draft" />
    <button onClick={action}>Open</button>
  </FadeSlideIn>;
  const { rerender } = render(view(400));
  const input = screen.getByRole("textbox", { name: "Question" });
  fireEvent.change(input, { target: { value: "edited draft" } });
  rerender(view(0));
  expect(screen.getByRole("textbox", { name: "Question" })).toBe(input);
  expect((input as HTMLInputElement).value).toBe("edited draft");
  await user.click(screen.getByRole("button", { name: "Open" }));
  expect(action).toHaveBeenCalledTimes(1);
});
