// INPUT: micromark 的 Markdown 字符流和容器边界。
// OUTPUT: 显式 LaTeX 括号分隔的完整/未闭合公式 token。
// POS: 公式语法唯一扩展；不在代码、链接目标或已转义字符内重写字符串。

import { markdownLineEnding, markdownSpace } from "micromark-util-character";
import type { Construct, Extension, State, Token, Tokenizer } from "micromark-util-types";

declare module "micromark-util-types" {
  interface TokenTypeMap {
    nexusMath: "nexusMath";
    nexusMathMarker: "nexusMathMarker";
    nexusMathValue: "nexusMathValue";
  }
  interface Token {
    _nexusMath?: { closed: boolean; display: boolean; flow: boolean; value: string };
  }
}

const tokenizeText: Tokenizer = function (effects, ok, nok) {
  let token: Token;
  let closing: number;
  return start;
  function start(code: Parameters<State>[0]): ReturnType<State> {
    token = effects.enter("nexusMath");
    effects.consume(code);
    return open;
  }
  function open(code: Parameters<State>[0]): ReturnType<State> {
    if (code !== 40 && code !== 91) return nok(code);
    closing = code === 40 ? 41 : 93;
    token._nexusMath = { closed: false, display: code === 91, flow: false, value: "" };
    effects.consume(code);
    return inside;
  }
  function finish(code: Parameters<State>[0]): ReturnType<State> {
    effects.exit("nexusMath");
    return ok(code);
  }
  function inside(code: Parameters<State>[0]): ReturnType<State> {
    if (code === null) return finish(code);
    effects.consume(code);
    return code === 92 ? afterSlash : inside;
  }
  function afterSlash(code: Parameters<State>[0]): ReturnType<State> {
    if (code === null) return finish(code);
    effects.consume(code);
    if (code !== closing) return inside; // A doubled slash belongs to the formula, not its closing delimiter.
    token._nexusMath!.closed = true;
    return finish;
  }
};

// Standalone fences own blank lines; inline \[...\] remains phrasing content.
function fence(bracket: number): Construct {
  return { partial: true, tokenize(effects, ok, nok) {
    return before;
    function before(code: Parameters<State>[0]): ReturnType<State> {
      effects.enter("nexusMathMarker");
      return slash(code);
    }
    function slash(code: Parameters<State>[0]): ReturnType<State> {
      if (markdownSpace(code)) { effects.consume(code); return slash; }
      if (code !== 92) return nok(code);
      effects.consume(code);
      return close;
    }
    function close(code: Parameters<State>[0]): ReturnType<State> {
      if (code !== bracket) return nok(code);
      effects.consume(code);
      return after;
    }
    function after(code: Parameters<State>[0]): ReturnType<State> {
      if (markdownSpace(code)) { effects.consume(code); return after; }
      if (code !== null && !markdownLineEnding(code)) return nok(code);
      effects.exit("nexusMathMarker");
      return ok(code);
    }
  } };
}

const nonLazyLine: Construct = { partial: true, tokenize(effects, ok, nok) {
  const context = this;
  return start;
  function start(code: Parameters<State>[0]): ReturnType<State> {
    if (code === null) return nok(code);
    effects.enter("lineEnding");
    effects.consume(code);
    effects.exit("lineEnding");
    return (next) => context.parser.lazy[context.now().line] ? nok(next) : ok(next);
  }
} };

const tokenizeFlow: Tokenizer = function (effects, ok, nok) {
  const context = this;
  let token: Token;
  let chunk: Token;
  const lines: string[] = [];
  return start;
  function start(code: Parameters<State>[0]): ReturnType<State> {
    token = effects.enter("nexusMath");
    token._nexusMath = { closed: false, display: true, flow: true, value: "" };
    return effects.attempt(fence(91), afterOpen, nok)(code);
  }
  function afterOpen(code: Parameters<State>[0]): ReturnType<State> {
    return context.interrupt ? finish(code) : nextLine(code);
  }
  function nextLine(code: Parameters<State>[0]): ReturnType<State> {
    if (code === null) return finish(code);
    return effects.attempt(nonLazyLine, lineStart, finish)(code);
  }
  function lineStart(code: Parameters<State>[0]): ReturnType<State> {
    return effects.attempt(fence(93), closed, contentStart)(code);
  }
  function contentStart(code: Parameters<State>[0]): ReturnType<State> {
    if (code === null) return finish(code);
    if (markdownLineEnding(code)) { lines.push(""); return nextLine(code); }
    chunk = effects.enter("nexusMathValue");
    return content(code);
  }
  function content(code: Parameters<State>[0]): ReturnType<State> {
    if (code === null || markdownLineEnding(code)) {
      effects.exit("nexusMathValue");
      lines.push(context.sliceSerialize(chunk));
      return nextLine(code);
    }
    effects.consume(code);
    return content;
  }
  function closed(code: Parameters<State>[0]): ReturnType<State> {
    token._nexusMath!.closed = true;
    return finish(code);
  }
  function finish(code: Parameters<State>[0]): ReturnType<State> {
    token._nexusMath!.value = lines.join("\n");
    effects.exit("nexusMath");
    return ok(code);
  }
};

export const LATEX_MATH_SYNTAX: Extension = {
  text: { 92: { name: "nexusMathText", tokenize: tokenizeText } },
  flow: { 92: { name: "nexusMathFlow", concrete: true, tokenize: tokenizeFlow } },
};
