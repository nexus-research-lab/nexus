// INPUT: 安全环境异步 Clipboard、旧浏览器复制能力与现有焦点。
// OUTPUT: 证明显式文本复制的成功/失败、降级和焦点恢复合同。
// POS: 共享浏览器能力回归；不访问操作系统真实剪贴板。
import { afterEach, describe, expect, it, vi } from "vitest";

import { writeTextToClipboard } from "./clipboard";

const originalExecCommand = Object.getOwnPropertyDescriptor(document, "execCommand");

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  if (originalExecCommand) Object.defineProperty(document, "execCommand", originalExecCommand);
  else Reflect.deleteProperty(document, "execCommand");
  document.body.replaceChildren();
});

describe("clipboard browser adapter", () => {
  it("uses the secure async clipboard without creating a fallback field", async () => {
    vi.stubGlobal("isSecureContext", true);
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    expect(await writeTextToClipboard("Nexus 中文正文")).toBe(true);
    expect(writeText).toHaveBeenCalledExactlyOnceWith("Nexus 中文正文");
    expect(document.querySelector("textarea")).toBeNull();
  });

  it("falls back after an async rejection, then removes the temporary field and restores focus", async () => {
    vi.stubGlobal("isSecureContext", true);
    vi.stubGlobal("navigator", { clipboard: { writeText: vi.fn().mockRejectedValue(new Error("denied")) } });
    const original = document.createElement("input");
    document.body.append(original);
    original.focus();
    const execCommand = vi.fn(() => {
      const field = document.activeElement as HTMLTextAreaElement;
      expect(field.tagName).toBe("TEXTAREA");
      expect(field.value).toBe("原始文本");
      expect(field.selectionStart).toBe(0);
      expect(field.selectionEnd).toBe(field.value.length);
      return true;
    });
    Object.defineProperty(document, "execCommand", { configurable: true, value: execCommand });
    expect(await writeTextToClipboard("原始文本")).toBe(true);
    expect(execCommand).toHaveBeenCalledExactlyOnceWith("copy");
    expect(document.activeElement).toBe(original);
    expect(document.querySelector("textarea")).toBeNull();
  });

  it("returns failure without a browser capability and ignores empty copy requests", async () => {
    vi.stubGlobal("isSecureContext", false);
    const writeText = vi.fn();
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    Reflect.deleteProperty(document, "execCommand");
    expect(await writeTextToClipboard("不可复制的文本")).toBe(false);
    expect(await writeTextToClipboard("")).toBe(false);
    expect(writeText).not.toHaveBeenCalled();
    expect(document.querySelector("textarea")).toBeNull();
  });
});
