"use client";

import { gsap } from "gsap";
import { ScrambleTextPlugin } from "gsap/ScrambleTextPlugin";
import { ScrollTrigger } from "gsap/ScrollTrigger";
import { SplitText } from "gsap/SplitText";
import { useGSAP } from "@gsap/react";

// Register once for the whole app. GSAP and all its plugins are free to use
// since the Webflow acquisition.
gsap.registerPlugin(ScrollTrigger, SplitText, ScrambleTextPlugin, useGSAP);

/** Animations run only inside this media query. */
export const MOTION_OK = "(prefers-reduced-motion: no-preference)";

/** Editorial easing used across reveals. */
export const EASE_OUT = "power3.out";

export { gsap, ScrollTrigger, SplitText, useGSAP };
