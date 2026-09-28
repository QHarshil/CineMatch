"use client";

import { useTypewriter } from "@/hooks/use-typewriter";

type TokenKind =
  | "plain"
  | "keyword"
  | "fn"
  | "prop"
  | "string"
  | "comment"
  | "num"
  | "punct"
  | "match";

interface Token {
  t: string;
  k?: TokenKind;
}

type CodeLine = Token[];

interface CodeTyperProps {
  /** Label shown in the window chrome. */
  filename: string;
  lines: CodeLine[];
  /** Single line summarising the result, revealed once typing finishes. */
  result?: string;
  className?: string;
  speed?: number;
}

// Near-monochrome blue, the editorial terminal palette. Amber and teal are the
// only sparks, reserved for match scores and string literals.
const TOKEN_COLOR: Record<TokenKind, string> = {
  plain: "text-foreground",
  keyword: "text-primary",
  fn: "text-primary",
  prop: "text-foreground",
  string: "text-[#0f766e]",
  comment: "text-muted-foreground italic",
  num: "text-[#9a6308]",
  punct: "text-muted-foreground",
  match: "text-[#9a6308] font-medium",
};

function lineLength(line: CodeLine): number {
  return line.reduce((sum, token) => sum + token.t.length, 0);
}

export function CodeTyper({
  filename,
  lines,
  result,
  className,
  speed = 28,
}: CodeTyperProps) {
  // Flatten to a single stream so the caret walks line to line. A newline costs
  // one character, which reads as a natural beat between statements.
  const starts: number[] = [];
  let cursor = 0;
  lines.forEach((line, index) => {
    starts.push(cursor);
    cursor += lineLength(line) + (index < lines.length - 1 ? 1 : 0);
  });
  const total = cursor;
  const fullText = lines
    .map((line) => line.map((token) => token.t).join(""))
    .join("\n");

  const { ref, count, done } = useTypewriter<HTMLDivElement>(total, { speed });
  const activeLine = starts.reduce(
    (active, start, index) => (count >= start ? index : active),
    0
  );

  return (
    <div
      ref={ref}
      className={`overflow-hidden border border-primary/40 bg-[#f3f7ff] ring-1 ring-inset ring-primary/10 ${className ?? ""}`}
    >
      {/* Full source for assistive tech and no-JS, since the visible code is
          revealed character by character only after hydration. */}
      <pre className="sr-only">{result ? `${fullText}\n${result}` : fullText}</pre>

      {/* Window chrome */}
      <div className="flex items-center gap-1.5 border-b border-primary/25 bg-white/60 px-4 py-2.5">
        <span className="size-2 rounded-full bg-primary/70" />
        <span className="size-2 rounded-full bg-primary/40" />
        <span className="size-2 rounded-full bg-primary/20" />
        <span className="ml-2 font-mono text-[11px] uppercase tracking-[0.18em] text-primary/70">
          {filename}
        </span>
      </div>

      {/* Code body */}
      <div className="px-4 py-4 font-mono text-[13px] leading-6 text-foreground sm:text-sm">
        {lines.map((line, lineIndex) => {
          let offset = starts[lineIndex];
          return (
            <div key={lineIndex} className="flex min-h-6 gap-4">
              <span className="w-4 shrink-0 select-none text-right text-primary/35">
                {lineIndex + 1}
              </span>
              <code className="whitespace-pre-wrap break-words">
                {line.map((token, tokenIndex) => {
                  const start = offset;
                  offset += token.t.length;
                  const revealed = Math.max(
                    0,
                    Math.min(token.t.length, count - start)
                  );
                  return (
                    <span
                      key={tokenIndex}
                      className={TOKEN_COLOR[token.k ?? "plain"]}
                    >
                      {token.t.slice(0, revealed)}
                    </span>
                  );
                })}
                {!done && activeLine === lineIndex && (
                  <span className="type-caret" aria-hidden="true" />
                )}
              </code>
            </div>
          );
        })}
      </div>

      {/* Result line */}
      {result && (
        <div
          className="flex items-center gap-2 border-t border-primary/20 bg-white/40 px-4 py-3 font-mono text-xs text-primary transition-opacity duration-500"
          style={{ opacity: done ? 1 : 0 }}
        >
          <span className="size-1.5 bg-primary" />
          {result}
        </div>
      )}
    </div>
  );
}
