"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { ReactLenis, type LenisRef } from "lenis/react";
import { gsap, ScrollTrigger } from "@/lib/motion";
import { useReducedMotion } from "@/hooks/use-reduced-motion";

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
    const lenis = lenisRef.current?.lenis;
    const tick = (seconds: number) => lenis?.raf(seconds * 1000);
    lenis?.on("scroll", ScrollTrigger.update);
    gsap.ticker.add(tick);
    gsap.ticker.lagSmoothing(0);
    return () => {
      lenis?.off("scroll", ScrollTrigger.update);
      gsap.ticker.remove(tick);
    };
  }, [reduced]);

  if (reduced) return <>{children}</>;
  return (
    <ReactLenis root ref={lenisRef} options={{ autoRaf: false, lerp: 0.12, allowNestedScroll: true }}>
      {children}
    </ReactLenis>
  );
}
