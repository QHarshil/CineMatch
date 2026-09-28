"use client";

import { useRef } from "react";
import { MOTION_OK, gsap, useGSAP } from "@/lib/motion";

interface ScrambleCycleProps {
  phrases: string[];
  className?: string;
  /** Seconds each phrase stays readable. */
  hold?: number;
}

/**
 * Cycles through phrases, decoding each from scrambled glyphs with GSAP's
 * ScrambleText plugin. Reduced motion shows the first phrase only.
 */
export function ScrambleCycle({ phrases, className, hold = 2.6 }: ScrambleCycleProps) {
  const ref = useRef<HTMLSpanElement>(null);

  useGSAP(
    () => {
      const el = ref.current;
      if (!el || phrases.length < 2) return;
      const mm = gsap.matchMedia();
      mm.add(MOTION_OK, () => {
        const timeline = gsap.timeline({ repeat: -1 });
        for (const phrase of [...phrases.slice(1), phrases[0]]) {
          timeline.to(el, {
            duration: 1.1,
            delay: hold,
            ease: "none",
            scrambleText: { text: phrase, chars: "lowerCase", speed: 0.5, revealDelay: 0.2 },
          });
        }
        return () => {
          timeline.kill();
          el.textContent = phrases[0];
        };
      });
      return () => mm.revert();
    },
    { scope: ref, dependencies: [phrases.join("|"), hold] },
  );

  return (
    <span ref={ref} className={className}>
      {phrases[0]}
    </span>
  );
}
