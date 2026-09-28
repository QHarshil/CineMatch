"use client";

import { useRef, type ElementType } from "react";
import { EASE_OUT, MOTION_OK, SplitText, gsap, useGSAP } from "@/lib/motion";

interface SplitHeadingProps {
  text: string;
  as?: ElementType;
  className?: string;
  /** Start as soon as it mounts, for above-the-fold headlines. */
  immediate?: boolean;
  delay?: number;
}

/**
 * Headline that rises line by line out of a mask as it enters the viewport.
 * Adapted from React Bits SplitText (MIT + Commons Clause, reactbits.dev),
 * rebuilt on GSAP's masked line splitting with a reduced-motion guard.
 */
export function SplitHeading({ text, as: Tag = "h2", className, immediate = false, delay = 0 }: SplitHeadingProps) {
  const ref = useRef<HTMLElement>(null);

  useGSAP(
    () => {
      const el = ref.current;
      if (!el) return;
      const mm = gsap.matchMedia();
      mm.add(MOTION_OK, () => {
        let split: SplitText | undefined;
        // Split after webfonts load so line breaks match the final layout.
        document.fonts.ready.then(() => {
          split = SplitText.create(el, {
            type: "lines,words",
            mask: "lines",
            autoSplit: true,
            onSplit: (self) =>
              gsap.from(self.words, {
                yPercent: 110,
                duration: 1,
                delay,
                ease: EASE_OUT,
                stagger: 0.035,
                scrollTrigger: immediate ? undefined : { trigger: el, start: "top 88%", once: true },
              }),
          });
        });
        return () => split?.revert();
      });
      return () => mm.revert();
    },
    { scope: ref, dependencies: [text] },
  );

  return (
    <Tag ref={ref} className={className}>
      {text}
    </Tag>
  );
}
