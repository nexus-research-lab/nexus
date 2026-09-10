// INPUT: Portal 兄弟层的语义 z-index 与锚点父子关系。
// OUTPUT: 子层高于父层、保留自身更高层级且注销恢复原样。
// POS: 公共浮层层级回归；不声称验证浏览器绘制。
import { expect, it } from "vitest";
import { registerAnchoredOverlay } from "./overlay-dismissal-runtime";

it.each([false, true])("orders nested menus above popovers (child first: %s)", (childFirst) => {
  const anchor = document.createElement("button");
  const parent = document.createElement("div");
  const childAnchor = document.createElement("button");
  const child = document.createElement("div");
  const grandchildAnchor = document.createElement("button");
  const grandchild = document.createElement("div");
  parent.style.zIndex = "140";
  child.style.zIndex = "120";
  grandchild.style.zIndex = "120";
  parent.append(childAnchor); child.append(grandchildAnchor);
  document.body.append(anchor, parent, child, grandchild);
  const cleanups: Array<() => void> = [];
  try {
    if (childFirst) cleanups.push(registerAnchoredOverlay(childAnchor, child));
    cleanups.push(registerAnchoredOverlay(anchor, parent));
    if (!childFirst) cleanups.push(registerAnchoredOverlay(childAnchor, child));
    cleanups.push(registerAnchoredOverlay(grandchildAnchor, grandchild));
    expect(Number(child.style.zIndex)).toBeGreaterThan(Number(parent.style.zIndex));
    expect(Number(grandchild.style.zIndex)).toBeGreaterThan(Number(child.style.zIndex));
    expect(parent.style.zIndex).toBe("140");
  } finally {
    cleanups.reverse().forEach((cleanup) => cleanup());
    expect(child.style.zIndex).toBe("120");
    expect(grandchild.style.zIndex).toBe("120");
    anchor.remove(); parent.remove(); child.remove(); grandchild.remove();
  }
});

it("preserves a child's higher semantic token from CSS", () => {
  const style = document.createElement("style");
  style.textContent = ".stack-parent { z-index: 140; } .stack-tooltip { z-index: 10030; }";
  const anchor = document.createElement("button");
  const parent = document.createElement("div"); parent.className = "stack-parent";
  const childAnchor = document.createElement("button"); parent.append(childAnchor);
  const child = document.createElement("div"); child.className = "stack-tooltip";
  document.head.append(style); document.body.append(anchor, parent, child);
  const unregisterParent = registerAnchoredOverlay(anchor, parent);
  const unregisterChild = registerAnchoredOverlay(childAnchor, child);
  try { expect(getComputedStyle(child).zIndex).toBe("10030"); }
  finally {
    unregisterChild(); unregisterParent();
    expect(child.style.zIndex).toBe("");
    style.remove(); anchor.remove(); parent.remove(); child.remove();
  }
});
