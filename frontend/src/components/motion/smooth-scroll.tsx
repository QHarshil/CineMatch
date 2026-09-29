"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { ReactLenis, useLenis, type LenisRef } from "lenis/react";
import { gsap, ScrollTrigger } from "@/lib/motion";
import { useReducedMotion } from "@/hooks/use-reduced-motion";

function ScrollTriggerSync() {
  useLenis(() => ScrollTrigger.update());
  return null;
}

/**
 * Lenis smooth scrolling driven by GSAP's ticker, so ScrollTrigger reads the
 * same scroll position Lenis renders. Off under reduced motion. Nested
 * scroll areas opt out with data-lenis-prevent.
 */
export function SmoothScroll({ children }: { children: ReactNode }) {
  const reduced = useReducedMotion();
  const lenisRef = useRef<LenisRef>(null);

  useEffect(() => {
    if (reduced) return;
    // ReactLenis creates its instance after this effect runs, so the ref is
    // read on every tick. Reading it once here left wheel input blocked with
    // nothing driving the scroll.
    const tick = (seconds: number) => lenisRef.current?.lenis?.raf(seconds * 1000);
    gsap.ticker.add(tick);
    gsap.ticker.lagSmoothing(0);
    return () => gsap.ticker.remove(tick);
  }, [reduced]);

  if (reduced) return <>{children}</>;
  return (
    <ReactLenis root ref={lenisRef} options={{ autoRaf: false, lerp: 0.12, allowNestedScroll: true }}>
      <ScrollTriggerSync />
      {children}
    </ReactLenis>
  );
}
