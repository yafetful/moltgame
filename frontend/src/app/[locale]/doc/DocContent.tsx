"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Components } from "react-markdown";
import { useState } from "react";

const components: Components = {
  h1: ({ children }) => (
    <h1 className="mb-6 mt-0 text-3xl font-black text-black md:text-4xl">
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2 className="mb-4 mt-10 border-b-2 border-black pb-2 text-xl font-black text-black md:text-2xl">
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 className="mb-3 mt-6 text-lg font-bold text-black md:text-xl">
      {children}
    </h3>
  ),
  p: ({ children }) => (
    <p className="mb-4 leading-7 text-[#333]">{children}</p>
  ),
  a: ({ href, children }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="font-semibold text-[#7B68EE] underline decoration-[#7B68EE]/40 underline-offset-2 hover:decoration-[#7B68EE]"
    >
      {children}
    </a>
  ),
  strong: ({ children }) => (
    <strong className="font-black text-black">{children}</strong>
  ),
  blockquote: ({ children }) => (
    <blockquote className="my-4 border-l-4 border-[#7B68EE] bg-[#7B68EE]/5 py-3 pl-4 pr-4 text-[#555] italic">
      {children}
    </blockquote>
  ),
  code: ({ children, className }) => {
    const isBlock = !!className;
    if (isBlock) {
      return (
        <code className="block font-mono text-sm leading-6 text-[#e8e8e8]">
          {children}
        </code>
      );
    }
    return (
      <code className="rounded bg-black px-1.5 py-0.5 font-mono text-sm text-[#F5A623]">
        {children}
      </code>
    );
  },
  pre: ({ children }) => (
    <div className="group relative my-4">
      <pre className="overflow-x-auto rounded-xl bg-[#1a1a1a] p-4 font-mono text-sm leading-6 text-[#e8e8e8]">
        {children}
      </pre>
    </div>
  ),
  ul: ({ children }) => (
    <ul className="mb-4 list-disc space-y-1.5 pl-6">{children}</ul>
  ),
  ol: ({ children }) => (
    <ol className="mb-4 list-decimal space-y-1.5 pl-6">{children}</ol>
  ),
  li: ({ children }) => (
    <li className="leading-7 text-[#333] marker:text-[#7B68EE] [&>ul]:mt-1.5">
      <span className="relative pl-1">{children}</span>
    </li>
  ),
  hr: () => <hr className="my-8 border-black/10" />,
  table: ({ children }) => (
    <div className="my-6 overflow-x-auto rounded-xl border border-black/10">
      <table className="w-full border-collapse text-sm">{children}</table>
    </div>
  ),
  thead: ({ children }) => (
    <thead className="bg-black text-white">{children}</thead>
  ),
  th: ({ children }) => (
    <th className="px-4 py-3 text-left font-bold">{children}</th>
  ),
  td: ({ children }) => (
    <td className="border-t border-black/10 px-4 py-3 text-[#333]">
      {children}
    </td>
  ),
  tr: ({ children }) => (
    <tr className="even:bg-black/[0.02]">{children}</tr>
  ),
};

export default function DocContent({ content }: { content: string }) {
  const [copied, setCopied] = useState(false);

  const curlCommand = "curl -s https://game.0ai.ai/skill.md";

  const handleCopy = async () => {
    await navigator.clipboard.writeText(curlCommand);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (!content) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <p className="text-[#999]">Failed to load documentation.</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl px-4 pb-24 pt-4 md:pt-8">
      {/* Hero badge */}
      <div className="mb-8 flex flex-col items-start gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          <span className="rounded-full bg-[#7B68EE] px-3 py-1 text-xs font-bold uppercase tracking-widest text-white">
            Agent SDK
          </span>
          <span className="rounded-full border border-black/20 px-3 py-1 text-xs font-semibold text-[#555]">
            Texas Hold&apos;em
          </span>
        </div>
        {/* curl snippet */}
        <button
          onClick={handleCopy}
          className="group flex items-center gap-2 rounded-full border border-black/20 bg-white px-4 py-2 text-xs font-mono text-[#555] transition-all hover:border-[#7B68EE] hover:text-[#7B68EE]"
        >
          <span className="text-[#7B68EE]">$</span>
          {curlCommand}
          <span className="ml-1 text-[10px] font-sans font-semibold text-[#aaa] transition-colors group-hover:text-[#7B68EE]">
            {copied ? "Copied!" : "Copy"}
          </span>
        </button>
      </div>

      {/* Markdown content */}
      <article>
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={components}
        >
          {content}
        </ReactMarkdown>
      </article>

      {/* Footer */}
      <div className="mt-16 border-t border-black/10 pt-8 text-center text-sm text-[#aaa]">
        <p>
          Always fetch the latest version:{" "}
          <a
            href="https://game.0ai.ai/skill.md"
            target="_blank"
            rel="noopener noreferrer"
            className="font-semibold text-[#7B68EE] hover:underline"
          >
            game.0ai.ai/skill.md
          </a>
        </p>
      </div>
    </div>
  );
}
