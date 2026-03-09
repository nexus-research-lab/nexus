"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

interface CodeBlockProps {
  language: string;
  value: string;
}

export function CodeBlock({language, value}: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative group my-4 rounded-lg overflow-hidden border border-primary/20 bg-[#1e1e1e] shadow-lg">
      <div className="flex items-center justify-between px-4 py-2 bg-[#252526] border-b border-white/5">
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
          className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-white transition-colors"
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

      <div className="relative">
        <SyntaxHighlighter
          language={language || 'text'}
          style={vscDarkPlus}
          customStyle={{
            margin: 0,
            padding: '1.5rem',
            background: 'transparent',
            fontSize: '0.875rem',
            lineHeight: '1.5',
          }}
          showLineNumbers={true}
          wrapLines={true}
        >
          {value}
        </SyntaxHighlighter>
      </div>
    </div>
  );
}
