import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useState } from "react";

import { resolveScrollFade, useScrollFade } from "./use-scroll-fade";

describe("resolveScrollFade", () => {

  it("returns both when scrolled midway", () => {
    expect(resolveScrollFade({ clientHeight: 200, scrollHeight: 500, scrollTop: 100 })).toBe("both");
  });

  it("returns top when scrolled to bottom", () => {
    expect(resolveScrollFade({ clientHeight: 200, scrollHeight: 500, scrollTop: 300 })).toBe("top");
  });
});


function GrowingFixture() {
  const { scrollRef, fadeProps } = useScrollFade();
  const [rows, setRows] = useState(0);
  return (
    <>
      <button onClick={() => setRows(20)} type="button">load</button>
      <div ref={scrollRef} {...fadeProps} data-testid="growing-container">
        {Array.from({ length: rows }, (_, index) => <div key={index}>row {index}</div>)}
      </div>
    </>
  );
}

describe("useScrollFade content changes", () => {
  it("recomputes the fade when rows load without the container resizing", () => {
    const { getByRole, getByTestId } = render(<GrowingFixture />);
    const container = getByTestId("growing-container");
    expect(container.getAttribute("data-sidebar-fade")).toBe("none");

    // jsdom 不做布局：模拟列表加载后内容溢出、容器高度不变。
    Object.defineProperty(container, "clientHeight", { configurable: true, value: 200 });
    Object.defineProperty(container, "scrollHeight", { configurable: true, value: 600 });
    fireEvent.click(getByRole("button", { name: "load" }));

    expect(container.getAttribute("data-sidebar-fade")).toBe("bottom");
  });
});
