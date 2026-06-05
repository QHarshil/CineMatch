"use client";

import { useTypewriter } from "@/hooks/use-typewriter";

interface TypingTextProps {
  text: string;
  className?: string;
  speed?: number;
  startDelay?: number;
  showCaret?: boolean;
  startOnVisible?: boolean;
}

/**
 * Types a single string in, one character at a time, with a blinking caret.
 * The full text is exposed to assistive tech immediately via an sr-only copy so
 * the animation never costs accessibility.
 */
export function TypingText({
  text,
  className,
  speed,
  startDelay,
  showCaret = true,
  startOnVisible = true,
}: TypingTextProps) {
  const { ref, count, done } = useTypewriter<HTMLSpanElement>(text.length, {
    speed,
    startDelay,
    startOnVisible,
  });

  return (
    <span ref={ref} className={className}>
      <span className="sr-only">{text}</span>
      <span aria-hidden="true">{text.slice(0, count)}</span>
      {showCaret && !done && <span className="type-caret" aria-hidden="true" />}
    </span>
  );
}
