"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { PrismAsyncLight as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneLight, vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";

import { cn } from "@/lib/utils";
import { useTheme } from "@/shared/theme/theme-context";

import { CodeShell } from "./code-shell";

interface CodeBlockContentProps {
  language: string;
  value: string;
}

const MESSAGE_CODE_FONT_FAMILY = "\"KingHwa_OldSong\", var(--font-mono), monospace";

export function CodeBlockContent({ language, value }: CodeBlockContentProps) {
  const { theme } = useTheme();
  const [copied, setCopied] = useState(false);
  const is_dark_theme = theme === "dark";

  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <CodeShell
      language={language}
      class_name="group"
      right_slot={(
        <button
          className={cn(
            "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors duration-(--motion-duration-fast)",
            copied && !is_dark_theme && "border-emerald-200/78 bg-emerald-100/90 text-emerald-600",
            copied && is_dark_theme && "border-green-500/22 bg-green-950/42 text-emerald-300",
          )}
          style={copied ? undefined : {
            background: "var(--chip-default-background)",
            borderColor: "var(--chip-default-border)",
            color: "var(--text-muted)",
          }}
          onClick={handleCopy}
        >
          {copied ? (
            <>
              <Check className="h-3.5 w-3.5" />
              <span>Copied</span>
            </>
          ) : (
            <>
              <Copy className="h-3.5 w-3.5" />
              <span>Copy</span>
            </>
          )}
        </button>
      )}
      content_class_name="relative grid min-w-0 overflow-hidden"
    >
      <div className="relative grid min-w-0 overflow-hidden">
        <SyntaxHighlighter
          language={language || "text"}
          style={is_dark_theme ? vscDarkPlus : oneLight}
          codeTagProps={{
            className: "message-cjk-code-font",
            style: {
              fontFamily: MESSAGE_CODE_FONT_FAMILY,
            },
          }}
          customStyle={{
            margin: 0,
            padding: "1.5rem",
            background: "transparent",
            fontFamily: MESSAGE_CODE_FONT_FAMILY,
            fontSize: "0.875rem",
            lineHeight: "1.5",
            overflowWrap: "anywhere",
            wordBreak: "break-word",
            whiteSpace: "pre-wrap",
          }}
          lineNumberStyle={{
            fontFamily: MESSAGE_CODE_FONT_FAMILY,
          }}
          showLineNumbers
          wrapLines
          wrapLongLines
        >
          {value}
        </SyntaxHighlighter>
      </div>
    </CodeShell>
  );
}
