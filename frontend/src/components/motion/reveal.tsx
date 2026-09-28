"use client";

import { useRef, type ReactNode } from "react";
import { EASE_OUT, MOTION_OK, gsap, useGSAP } from "@/lib/motion";

interface RevealProps {
  children: ReactNode;
  className?: string;
  /** Animate direct children one after another instead of the block at once. */
  stagger?: boolean;
  delay?: number;
  y?: number;
}

/**
 * Fades content up as it scrolls into view. Markup renders visible, so the
 * page reads fine without JavaScript or with reduced motion.
 */
export function Reveal({ children, className, stagger = false, delay = 0, y = 28 }: RevealProps) {
  const ref = useRef<HTMLDivElement>(null);

  useGSAP(
    () => {
      const el = ref.current;
      if (!el) return;
      const mm = gsap.matchMedia();
      mm.add(MOTION_OK, () => {
        const targets = stagger ? Array.from(el.children) : el;
        gsap.from(targets, {
          autoAlpha: 0,
          y,
          duration: 0.9,
          delay,
          ease: EASE_OUT,
          stagger: stagger ? 0.08 : 0,
          scrollTrigger: { trigger: el, start: "top 85%", once: true },
        });
      });
      return () => mm.revert();
    },
    { scope: ref },
  );

  return (
    <div ref={ref} className={className}>
      {children}
    </div>
  );
}
