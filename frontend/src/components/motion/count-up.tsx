"use client";

import { useRef } from "react";
import { EASE_OUT, MOTION_OK, gsap, useGSAP } from "@/lib/motion";

interface CountUpProps {
  value: number;
  decimals?: number;
  prefix?: string;
  suffix?: string;
  className?: string;
}

/**
 * Counts from zero to a measured value when it scrolls into view. The final
 * value is in the markup, so screen readers and no-JS visitors read it as is.
 */
export function CountUp({ value, decimals = 0, prefix = "", suffix = "", className }: CountUpProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const format = (n: number) => `${prefix}${n.toFixed(decimals)}${suffix}`;

  useGSAP(
    () => {
      const el = ref.current;
      if (!el) return;
      const mm = gsap.matchMedia();
      mm.add(MOTION_OK, () => {
        const counter = { n: 0 };
        el.textContent = format(0);
        gsap.to(counter, {
          n: value,
          duration: 1.6,
          ease: EASE_OUT,
          scrollTrigger: { trigger: el, start: "top 90%", once: true },
          onUpdate: () => {
            el.textContent = format(counter.n);
          },
        });
        return () => {
          el.textContent = format(value);
        };
      });
      return () => mm.revert();
    },
    { scope: ref, dependencies: [value, decimals, prefix, suffix] },
  );

  return (
    <span ref={ref} className={className}>
      {format(value)}
    </span>
  );
}
