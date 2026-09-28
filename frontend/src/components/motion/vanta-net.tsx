"use client";

import { useEffect, useRef } from "react";
import type { VantaEffect } from "vanta/dist/vanta.net.min";
import { useReducedMotion } from "@/hooks/use-reduced-motion";

/**
 * Vanta's NET effect: points drifting in 3D and linking to their neighbors.
 * Vanta and three.js load only when this mounts, it renders only while on
 * screen, and it stays off on small screens and under reduced motion.
 */
export function VantaNet({ className }: { className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const reduced = useReducedMotion();

  useEffect(() => {
    const el = ref.current;
    if (!el || reduced || window.matchMedia("(max-width: 767px)").matches) return;

    let effect: VantaEffect | undefined;
    let visible = false;

    async function start(target: HTMLElement) {
      const [{ default: NET }, THREE] = await Promise.all([import("vanta/dist/vanta.net.min"), import("three")]);
      if (!visible || effect) return;
      effect = NET({
        el: target,
        THREE,
        color: 0x2f54ff,
        backgroundColor: 0xffffff,
        backgroundAlpha: 0,
        points: 9,
        maxDistance: 21,
        spacing: 17,
        showDots: true,
        mouseControls: true,
        touchControls: false,
        gyroControls: false,
        scale: 1,
        scaleMobile: 1,
      });
    }

    function stop() {
      effect?.destroy();
      effect = undefined;
    }

    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      if (visible) void start(el);
      else stop();
    });
    observer.observe(el);
    return () => {
      visible = false;
      observer.disconnect();
      stop();
    };
  }, [reduced]);

  return <div ref={ref} aria-hidden="true" className={className} />;
}
