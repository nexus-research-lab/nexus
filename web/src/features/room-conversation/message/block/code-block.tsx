"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { CodeBlockProps } from "@/types/room-conversation";

export function CodeBlock({ language, value }: CodeBlockProps) {
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
            <div className="w-2.5 h-2.5 rounded-full bg-red-500/20 border border-red-500/50"/>
            <div className="w-2.5 h-2.5 rounded-full bg-yellow-500/20 border border-yellow-500/50"/>
            <div className="w-2.5 h-2.5 rounded-full bg-green-500/20 border border-green-500/50"/>
          </div>
          <span className="text-xs text-muted-foreground font-mono ml-2">{language || 'text'}</span>
        </div>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1.5 rounded-full bg-white/5 px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:text-white"
        >
          {copied ? (
            <>
              <Check className="w-3.5 h-3.5 text-green-500"/>
              <span className="text-green-500">Copied</span>
            </>
          ) : (
            <>
              <Copy className="w-3.5 h-3.5"/>
              <span>Copy</span>
            </>
          )}
        </button>
      </div>

      <div className="relative grid">
        <SyntaxHighlighter
          language={language || 'text'}
          style={vscDarkPlus}
          customStyle={{
            margin: 0,
            padding: '1.5rem',
            background: 'transparent',
            fontSize: '0.875rem',
            lineHeight: '1.5',
            overflowWrap: 'anywhere',
            wordBreak: 'break-word',
            whiteSpace: 'pre-wrap',
          }}
          showLineNumbers={true}
          wrapLines={true}
          wrapLongLines={true}
        >
          {value}
        </SyntaxHighlighter>
      </div>
    </div>
  );
}
