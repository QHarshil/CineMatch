"use client";

import { useRef, type ReactNode, type PointerEvent } from "react";

/**
 * A cell with a soft blue spotlight that follows the pointer. Adapted from
 * React Bits SpotlightCard (MIT + Commons Clause, reactbits.dev); the pointer
 * position is written to CSS variables so moving the mouse never re-renders.
 */
export function SpotlightCard({ children, className = "" }: { children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);

  function track(e: PointerEvent<HTMLDivElement>) {
    const el = ref.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    el.style.setProperty("--spot-x", `${e.clientX - rect.left}px`);
    el.style.setProperty("--spot-y", `${e.clientY - rect.top}px`);
  }

  return (
    <div ref={ref} onPointerMove={track} className={`spotlight-card group relative overflow-hidden ${className}`}>
      <div
        aria-hidden="true"
        className="spotlight-card__glow pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-500 group-hover:opacity-100"
      />
      <div className="relative">{children}</div>
    </div>
  );
}
