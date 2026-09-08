// INPUT: Native composition flags across modern and legacy keyboard events.
// OUTPUT: Composition detection independent from shortcut modifiers or application state.
// POS: Shared keyboard predicate regression; consumer tests prove that commands are actually suppressed.

import { describe, expect, it } from "vitest";
import { isImeKeyboardEvent } from "./ime-keyboard-event";

describe("IME keyboard events", () => {
  it.each([{ isComposing: true }, { key: "Process" }, { keyCode: 229 }, { which: 229 }])("recognizes %j", (event) => {
    expect(isImeKeyboardEvent(event)).toBe(true);
  });
  it("leaves ordinary Enter and Escape available to application shortcuts", () => {
    expect(isImeKeyboardEvent({ key: "Enter", keyCode: 13, isComposing: false })).toBe(false);
    expect(isImeKeyboardEvent({ key: "Escape", keyCode: 27 })).toBe(false);
  });
});
