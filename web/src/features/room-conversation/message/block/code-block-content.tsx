"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { PrismAsyncLight as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";

interface CodeBlockContentProps {
  language: string;
  value: string;
}

export function CodeBlockContent({ language, value }: CodeBlockContentProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative group my-4 overflow-hidden rounded-[22px] border border-white/10 bg-[#1e1e1e] shadow-[0_22px_36px_rgba(17,24,39,0.28)]">
      <div className="flex items-center justify-between border-b border-white/5 bg-[#252526] px-4 py-2">
        <div className="flex items-center gap-2">
          <div className="flex gap-1.5">
            <div className="h-2.5 w-2.5 rounded-full border border-red-500/50 bg-red-500/20" />
            <div className="h-2.5 w-2.5 rounded-full border border-yellow-500/50 bg-yellow-500/20" />
            <div className="h-2.5 w-2.5 rounded-full border border-green-500/50 bg-green-500/20" />
          </div>
          <span className="ml-2 font-mono text-xs text-muted-foreground">{language || "text"}</span>
        </div>
        <button
          className="flex items-center gap-1.5 rounded-full bg-white/5 px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:text-white"
          onClick={handleCopy}
        >
          {copied ? (
            <>
              <Check className="h-3.5 w-3.5 text-green-500" />
              <span className="text-green-500">Copied</span>
            </>
          ) : (
            <>
              <Copy className="h-3.5 w-3.5" />
              <span>Copy</span>
            </>
          )}
        </button>
      </div>

      <div className="relative grid">
        <SyntaxHighlighter
          language={language || "text"}
          style={vscDarkPlus}
          customStyle={{
            margin: 0,
            padding: "1.5rem",
            background: "transparent",
            fontSize: "0.875rem",
            lineHeight: "1.5",
            overflowWrap: "anywhere",
            wordBreak: "break-word",
            whiteSpace: "pre-wrap",
          }}
          showLineNumbers
          wrapLines
          wrapLongLines
        >
          {value}
        </SyntaxHighlighter>
      </div>
    </div>
  );
}
