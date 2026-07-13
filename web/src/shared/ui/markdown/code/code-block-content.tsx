"use client";

import { Check, Copy } from "lucide-react";
import { PrismAsyncLight as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneLight, vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";

import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { cn } from "@/shared/ui/class-name";
import { useTheme } from "@/shared/theme/theme-context";

import { CodeShell } from "./code-shell";

interface CodeBlockContentProps {
  language: string;
  value: string;
}

const MESSAGE_CODE_FONT_FAMILY = "\"KingHwaOldSong\", var(--font-mono), monospace";

export function CodeBlockContent({ language, value }: CodeBlockContentProps) {
  const { theme } = useTheme();
  const { copied, copy } = useCopyToClipboard();
  const isDarkTheme = theme === "dark" || theme === "rain";

  const handleCopy = () => {
    void copy(value);
  };

  return (
    <CodeShell
      language={language}
      className="group"
      rightSlot={(
        <button
          className={cn(
            "inline-flex h-3 w-3 items-center justify-center rounded-[6px] border border-transparent transition-colors duration-(--motion-duration-fast)",
            copied && "bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-(--success)",
          )}
          style={copied ? undefined : {
            color: "var(--text-muted)",
          }}
          onClick={handleCopy}
          title={copied ? "已复制" : `复制 ${language || "text"} 代码`}
          type="button"
        >
          {copied ? (
            <Check className="h-3 w-3" />
          ) : (
            <Copy className="h-3 w-3" />
          )}
        </button>
      )}
      contentClassName="relative min-w-0 overflow-x-auto overflow-y-hidden"
    >
      <div className="relative min-w-0">
        <SyntaxHighlighter
          language={language || "text"}
          style={isDarkTheme ? vscDarkPlus : oneLight}
          codeTagProps={{
            className: "message-cjk-code-font",
            style: {
              fontFamily: MESSAGE_CODE_FONT_FAMILY,
            },
          }}
          customStyle={{
            margin: 0,
            padding: "0.85rem 0.95rem 0.9rem",
            background: "transparent",
            fontFamily: MESSAGE_CODE_FONT_FAMILY,
            fontSize: "0.8rem",
            lineHeight: "1.6",
            width: "max-content",
            minWidth: "100%",
            whiteSpace: "pre",
          }}
          lineNumberStyle={{
            fontFamily: MESSAGE_CODE_FONT_FAMILY,
            minWidth: "1.45rem",
            paddingRight: "0.45rem",
            color: "var(--text-faint)",
            fontSize: "0.68rem",
            userSelect: "none",
          }}
          showLineNumbers
        >
          {value}
        </SyntaxHighlighter>
      </div>
    </CodeShell>
  );
}
