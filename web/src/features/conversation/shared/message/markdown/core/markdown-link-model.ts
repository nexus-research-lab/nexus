import type { ReactNode } from "react";

const URL_TRAILING_PUNCTUATION_PATTERN = /[.,;:!?，。；：！？、]+$/u;
const ALLOWED_MARKDOWN_LINK_PROTOCOLS = new Set(["http:", "https:", "mailto:"]);

export function getPlainTextFromChildren(children: ReactNode): string | null {
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

export function splitTrailingUrlPunctuation(value: string): {
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

export function normalizeExternalMarkdownHref(href: string): string | null {
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

export function compactExternalUrlLabel(href: string): string {
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
