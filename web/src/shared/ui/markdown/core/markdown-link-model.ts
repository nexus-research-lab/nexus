import type { ReactNode } from "react";

const URL_TRAILING_PUNCTUATION_PATTERN = /[.,;:!?，。；：！？、]+$/u;
const ALLOWED_MARKDOWN_LINK_PROTOCOLS = new Set(["http:", "https:", "mailto:"]);
const STREAMING_URL_TAIL_PATTERN = /(?:https?:\/\/|www\.|mailto:)[^\s<>"'，。；！？]*$/iu;
const STREAMING_MARKDOWN_LINK_DESTINATION_TAIL_PATTERN = /(\[[^\]\n]{0,180}\]\()((?:https?:\/\/|www\.|mailto:)[^\s)]*)$/iu;
const STREAMING_AUTOLINK_TAIL_PATTERN = /<((?:https?:\/\/|www\.|mailto:)[^\s>]*)$/iu;

export type MarkdownLinkPresentation =
  | { kind: "text" }
  | { href: string; kind: "anchor" }
  | { kind: "workspace"; path: string }
  | {
      compactLabel: string | null;
      href: string;
      kind: "external";
      openInNewTab: boolean;
      trailingText: string;
    };

interface MarkdownLinkContext {
  rawHref: string;
  workspacePath: string | null;
}

type ImmediateMarkdownLinkPresentation = Exclude<
  MarkdownLinkPresentation,
  { kind: "external" }
>;

type ImmediateMarkdownLinkRule = (
  context: MarkdownLinkContext,
) => ImmediateMarkdownLinkPresentation | null;

const IMMEDIATE_MARKDOWN_LINK_RULES: ImmediateMarkdownLinkRule[] = [
  ({ rawHref }) => rawHref.length === 0 ? { kind: "text" } : null,
  ({ workspacePath }) => workspacePath === null
    ? null
    : { kind: "workspace", path: workspacePath },
  ({ rawHref }) => rawHref.startsWith("#")
    ? { href: rawHref, kind: "anchor" }
    : null,
];

interface StreamingUrlTail {
  prefix: string;
  replaceOffset: number;
  url: string;
  urlOffset: number;
}

type StreamingUrlTailMatcher = (content: string) => StreamingUrlTail | null;

const STREAMING_URL_TAIL_MATCHERS: StreamingUrlTailMatcher[] = [
  matchMarkdownLinkDestinationTail,
  matchMarkdownAutolinkTail,
  matchBareUrlTail,
];

function getPlainTextFromChildren(children: ReactNode): string | null {
  if (typeof children === "string" || typeof children === "number") {
    return String(children);
  }

  if (!Array.isArray(children)) {
    return null;
  }

  const parts: string[] = [];
  for (const child of children) {
    if (typeof child === "string" || typeof child === "number") {
      parts.push(String(child));
      continue;
    }
    if (child === null || child === undefined || typeof child === "boolean") {
      continue;
    }
    return null;
  }

  return parts.join("");
}

function countChar(value: string, char: string): number {
  return Array.from(value).filter((item) => item === char).length;
}

function splitTrailingUrlPunctuation(value: string): {
  href: string;
  trailingText: string;
} {
  let href = value.trim();
  let trailingText = "";

  const appendTrailing = (text: string) => {
    trailingText = `${text}${trailingText}`;
  };

  while (href) {
    const punctuationMatch = URL_TRAILING_PUNCTUATION_PATTERN.exec(href);
    if (punctuationMatch?.[0]) {
      href = href.slice(0, -punctuationMatch[0].length);
      appendTrailing(punctuationMatch[0]);
      continue;
    }

    const lastChar = href.at(-1);
    const unbalancedPair = lastChar === ")"
      ? [")", "("]
      : lastChar === "]"
        ? ["]", "["]
        : null;
    if (unbalancedPair && countChar(href, unbalancedPair[0]) > countChar(href, unbalancedPair[1])) {
      href = href.slice(0, -1);
      appendTrailing(unbalancedPair[0]);
      continue;
    }

    break;
  }

  return { href, trailingText };
}

function normalizeExternalMarkdownHref(href: string): string | null {
  const trimmed = href.trim();
  if (!trimmed) {
    return null;
  }

  const normalized = /^www\./i.test(trimmed) ? `https://${trimmed}` : trimmed;
  try {
    const url = new URL(normalized);
    return ALLOWED_MARKDOWN_LINK_PROTOCOLS.has(url.protocol) ? url.toString() : null;
  } catch {
    return null;
  }
}

function compactExternalUrlLabel(href: string): string {
  if (href.startsWith("mailto:")) {
    return href.slice("mailto:".length);
  }

  try {
    const url = new URL(href);
    const host = url.hostname.replace(/^www\./i, "");
    const suffix = `${url.pathname === "/" ? "" : url.pathname}${url.search ? "?..." : ""}${url.hash ? "#..." : ""}`;
    const label = `${host}${suffix}`;
    return label.length > 64 ? `${label.slice(0, 42)}...${label.slice(-16)}` : label;
  } catch {
    return href;
  }
}

function shouldCompactExternalLabel(
  plainText: string | null,
  context: MarkdownLinkContext,
  hrefWithoutTrailing: string,
  externalHref: string,
): boolean {
  if (!plainText) {
    return false;
  }
  const equivalentLabels = new Set([
    context.rawHref,
    hrefWithoutTrailing,
    externalHref,
  ]);
  const normalizedPlainTextHref = normalizeExternalMarkdownHref(
    splitTrailingUrlPunctuation(plainText).href,
  );
  return equivalentLabels.has(plainText)
    || normalizedPlainTextHref === externalHref;
}

function buildExternalLinkPresentation(
  context: MarkdownLinkContext,
  children: ReactNode,
): MarkdownLinkPresentation {
  const { href: hrefWithoutTrailing, trailingText } =
    splitTrailingUrlPunctuation(context.rawHref);
  const externalHref = normalizeExternalMarkdownHref(hrefWithoutTrailing);
  if (!externalHref) {
    return { kind: "text" };
  }

  const plainText = getPlainTextFromChildren(children);
  return {
    compactLabel: shouldCompactExternalLabel(
      plainText,
      context,
      hrefWithoutTrailing,
      externalHref,
    )
      ? compactExternalUrlLabel(externalHref)
      : null,
    href: externalHref,
    kind: "external",
    openInNewTab: !externalHref.startsWith("mailto:"),
    trailingText,
  };
}

export function buildMarkdownLinkPresentation(
  href: string | undefined,
  children: ReactNode,
  workspacePath: string | null,
): MarkdownLinkPresentation {
  const context: MarkdownLinkContext = {
    rawHref: String(href ?? "").trim(),
    workspacePath,
  };
  const immediatePresentation = IMMEDIATE_MARKDOWN_LINK_RULES
    .map((build) => build(context))
    .find((candidate) => candidate !== null);
  return immediatePresentation
    ?? buildExternalLinkPresentation(context, children);
}

function matchMarkdownLinkDestinationTail(
  content: string,
): StreamingUrlTail | null {
  const match = STREAMING_MARKDOWN_LINK_DESTINATION_TAIL_PATTERN.exec(content);
  const url = match?.[2];
  if (!url) {
    return null;
  }
  const urlOffset = content.length - url.length;
  return { prefix: "", replaceOffset: urlOffset, url, urlOffset };
}

function matchMarkdownAutolinkTail(content: string): StreamingUrlTail | null {
  const url = STREAMING_AUTOLINK_TAIL_PATTERN.exec(content)?.[1];
  if (!url) {
    return null;
  }
  const urlOffset = content.length - url.length;
  return { prefix: "&lt;", replaceOffset: urlOffset - 1, url, urlOffset };
}

function matchBareUrlTail(content: string): StreamingUrlTail | null {
  const url = STREAMING_URL_TAIL_PATTERN.exec(content)?.[0];
  if (!url) {
    return null;
  }
  const urlOffset = content.length - url.length;
  return { prefix: "", replaceOffset: urlOffset, url, urlOffset };
}

function escapeMarkdownUrlTail(value: string): string {
  return value.replace(/([.:\u003c\u003e()[\]])/g, "\\$1");
}

export function stabilizeStreamingMarkdownUrlTail(
  content: string,
  isStreaming: boolean,
  isProtectedOffset: (offset: number) => boolean,
): string {
  if (!isStreaming || !content) {
    return content;
  }
  const tail = STREAMING_URL_TAIL_MATCHERS
    .map((match) => match(content))
    .find((candidate): candidate is StreamingUrlTail => candidate !== null);
  if (!tail || isProtectedOffset(tail.urlOffset)) {
    return content;
  }

  // 流式 URL 在收尾前先打断自动链接，出现空白或换行后再恢复真实链接。
  return `${content.slice(0, tail.replaceOffset)}${tail.prefix}${escapeMarkdownUrlTail(tail.url)}`;
}
