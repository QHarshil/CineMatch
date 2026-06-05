"use client";

import { useEffect, useRef, useState } from "react";

interface TypewriterOptions {
  /** Milliseconds between each revealed character. */
  speed?: number;
  /** Delay before the first character appears, in milliseconds. */
  startDelay?: number;
  /** Gate the animation until the element scrolls into view (default true). */
  startOnVisible?: boolean;
}

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/**
 * Drives a character-by-character reveal. Returns the number of characters that
 * should currently be visible so callers can slice arbitrary content (plain
 * strings, or multi-line tokenised code) against a single source of truth.
 *
 * When the user prefers reduced motion the full content is revealed instantly.
 */
export function useTypewriter<T extends HTMLElement = HTMLDivElement>(
  total: number,
  options: TypewriterOptions = {}
) {
  const { speed = 36, startDelay = 0, startOnVisible = true } = options;
  const ref = useRef<T>(null);
  const [count, setCount] = useState(0);
  const [started, setStarted] = useState(!startOnVisible);

  useEffect(() => {
    if (started) return;
    const el = ref.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setStarted(true);
          observer.disconnect();
        }
      },
      { threshold: 0.25 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [started]);

  useEffect(() => {
    if (!started) return;

    if (prefersReducedMotion()) {
      setCount(total);
      return;
    }

    setCount(0);
    let revealed = 0;
    let tick: ReturnType<typeof setTimeout>;

    const begin = setTimeout(function step() {
      revealed += 1;
      setCount(revealed);
      if (revealed < total) {
        tick = setTimeout(step, speed);
      }
    }, startDelay);

    return () => {
      clearTimeout(begin);
      clearTimeout(tick);
    };
  }, [started, total, speed, startDelay]);

  return { ref, count, done: count >= total, started };
}
