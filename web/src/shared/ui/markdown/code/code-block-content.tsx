"use client";

import { Check, Copy } from "lucide-react";
import { PrismAsyncLight as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneLight, vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";

import { useCopyToClipboard } from "@/shared/lib/react/use-copy-to-clipboard";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useTheme } from "@/shared/theme/theme-context";

import { CodeShell } from "./code-shell";

interface CodeBlockContentProps {
  language: string;
  value: string;
}

interface SyntaxHighlightedCodeProps extends CodeBlockContentProps {
  variant?: "block" | "workspace";
}

const MESSAGE_CODE_FONT_FAMILY = "var(--font-mono)";
const LIGHT_CODE_COLOR = "rgb(20, 24, 31)";
const LIGHT_PUNCTUATION_COLOR = "rgb(43, 48, 59)";
const LIGHT_KEYWORD_COLOR = "rgb(129, 0, 194)";
const LIGHT_STRING_COLOR = "rgb(0, 128, 0)";
const LIGHT_FUNCTION_COLOR = "rgb(0, 81, 194)";
const LIGHT_LITERAL_COLOR = "rgb(0, 128, 128)";
const LIGHT_PARAMETER_COLOR = "rgb(184, 79, 5)";

/** 中文注释：浅色代码色板与阅读区基准保持一致，语义色只负责区分 token，不借用品牌交互色。 */
const lightSyntaxStyle = {
  ...oneLight,
  "code[class*=\"language-\"]": {
    ...oneLight["code[class*=\"language-\"]"],
    background: "transparent",
    color: LIGHT_CODE_COLOR,
    lineHeight: "1.625",
  },
  "pre[class*=\"language-\"]": {
    ...oneLight["pre[class*=\"language-\"]"],
    background: "transparent",
    color: LIGHT_CODE_COLOR,
    lineHeight: "1.625",
  },
  doctype: { color: LIGHT_CODE_COLOR },
  punctuation: { color: LIGHT_PUNCTUATION_COLOR },
  entity: { color: LIGHT_PUNCTUATION_COLOR },
  keyword: { color: LIGHT_KEYWORD_COLOR },
  operator: { color: LIGHT_KEYWORD_COLOR },
  string: { color: LIGHT_STRING_COLOR },
  char: { color: LIGHT_STRING_COLOR },
  selector: { color: LIGHT_STRING_COLOR },
  "attr-value": { color: LIGHT_STRING_COLOR },
  regex: { color: LIGHT_STRING_COLOR },
  inserted: { color: LIGHT_STRING_COLOR },
  function: { color: LIGHT_FUNCTION_COLOR },
  "class-name": { color: LIGHT_FUNCTION_COLOR },
  builtin: { color: LIGHT_FUNCTION_COLOR },
  boolean: { color: LIGHT_LITERAL_COLOR },
  constant: { color: LIGHT_LITERAL_COLOR },
  number: { color: LIGHT_LITERAL_COLOR },
  symbol: { color: LIGHT_LITERAL_COLOR },
  variable: { color: LIGHT_PARAMETER_COLOR },
  parameter: { color: LIGHT_PARAMETER_COLOR },
  "attr-name": { color: LIGHT_PARAMETER_COLOR },
};

export function SyntaxHighlightedCode({
  language,
  value,
  variant = "block",
}: SyntaxHighlightedCodeProps) {
  const { theme } = useTheme();
  const isDarkTheme = theme === "dark" || theme === "rain";

  return (
    <SyntaxHighlighter
      language={language || "text"}
      style={isDarkTheme ? vscDarkPlus : lightSyntaxStyle}
      codeTagProps={{
        className: "message-code-font",
        style: {
          fontFamily: MESSAGE_CODE_FONT_FAMILY,
        },
      }}
      customStyle={{
        margin: 0,
        padding: variant === "workspace" ? 0 : "0.875rem",
        minHeight: variant === "workspace" ? "100%" : undefined,
        overflow: variant === "workspace" ? "visible" : undefined,
        background: "transparent",
        fontFamily: MESSAGE_CODE_FONT_FAMILY,
        fontSize: "0.875rem",
        lineHeight: "1.625",
        borderRadius: variant === "workspace" ? 0 : "var(--radius-md)",
        whiteSpace: "pre",
      }}
    >
      {value}
    </SyntaxHighlighter>
  );
}

export function CodeBlockContent({ language, value }: CodeBlockContentProps) {
  const { copied, copy } = useCopyToClipboard();
  const { t } = useI18n();
  const copyLabel = t("markdown.code.copy", { language: language || "text" });

  const handleCopy = () => {
    void copy(value);
  };

  return (
    <CodeShell
      language={language}
      className="group"
      rightSlot={(
        <button
          aria-label={copyLabel}
          className="content-code-action"
          data-copied={copied ? "true" : undefined}
          onClick={handleCopy}
          title={copied ? t("markdown.code.copied") : copyLabel}
          type="button"
        >
          {copied ? (
            <Check className="h-4 w-4" />
          ) : (
            <Copy className="h-4 w-4" />
          )}
        </button>
      )}
      contentClassName="relative min-w-0 overflow-x-auto overflow-y-hidden"
    >
      <div className="relative min-w-0">
        <SyntaxHighlightedCode language={language} value={value} />
      </div>
    </CodeShell>
  );
}
