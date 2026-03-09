"use client";

import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import remarkBreaks from 'remark-breaks';
import rehypeKatex from 'rehype-katex';
import { CodeBlock } from './block/code-block';
import { cn } from '@/lib/utils';
import 'katex/dist/katex.min.css';

import { useTypewriter } from '@/hooks/use-typewriter';

interface MarkdownRendererProps {
  content: string;
  className?: string;
  isStreaming?: boolean;
}

export function MarkdownRenderer({ content, className, isStreaming = false }: MarkdownRendererProps) {
  const { displayedText, isComplete } = useTypewriter({
    text: content,
    enabled: isStreaming,
    speed: 5, // Fast typing
  });

  return (
    <div className={cn("prose prose-sm max-w-none", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath, remarkBreaks]}
        rehypePlugins={[rehypeKatex]}
        components={{
          code({ className, children, ...props }: any) {
            const match = /language-(\w+)/.exec(className || '');
            const value = String(children).replace(/\n$/, '');
            const isCodeBlock = match && value.includes('\n');

            return isCodeBlock ? (
              <CodeBlock language={match[1]} value={value} />
            ) : (
              <span
                className="inline-block px-2 py-0.5 mx-0.5 bg-primary/10 border border-primary/20 text-primary text-sm font-mono rounded-md  whitespace-pre-wrap text-justify"
                {...props}
              >
                {children}
              </span>
            );
          },
          p({ children }) {
            return <p className="mb-2 mt-2 last:mb-0 leading-relaxed text-foreground/90 text-justify">{children}</p>;
          },
          ul({ children }) {
            return <ul className="list-disc list-inside mb-4 space-y-1 text-foreground/90">{children}</ul>;
          },
          ol({ children }) {
            return <ol className="list-decimal list-inside mb-4 space-y-1 text-foreground/90">{children}</ol>;
          },
          blockquote({ children }) {
            return (
              <blockquote
                className="border-l-4 border-primary/30 pl-4 my-4 text-muted-foreground italic bg-primary/5 py-2 rounded-r">
                {children}
              </blockquote>
            );
          },
          a({ href, children }) {
            return (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline decoration-primary/30 underline-offset-4 transition-all"
              >
                {children}
              </a>
            );
          },
          h1({ children }) {
            return <h1 className="text-2xl font-bold mb-4 mt-6 first:mt-0 text-foreground">{children}</h1>;
          },
          h2({ children }) {
            return <h2 className="text-xl font-bold mb-3 mt-5 text-foreground">{children}</h2>;
          },
          h3({ children }) {
            return <h3 className="text-lg font-bold mb-2 mt-4 text-foreground">{children}</h3>;
          },
          table({ children }) {
            return (
              <div className="overflow-x-auto my-4 border border-border rounded-lg">
                <table className="w-full text-sm text-left">{children}</table>
              </div>
            );
          },
          thead({ children }) {
            return <thead className="bg-muted/50 text-muted-foreground uppercase">{children}</thead>;
          },
          th({ children }) {
            return <th className="px-4 py-3 font-medium">{children}</th>;
          },
          td({ children }) {
            return <td className="px-4 py-3 border-t border-border">{children}</td>;
          },
        }}
      >
        {displayedText}
      </ReactMarkdown>
      {isStreaming && !isComplete && (
        <span className="inline-block w-2 h-4 ml-1 align-middle bg-primary animate-pulse" />
      )}
    </div>
  );
}