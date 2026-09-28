declare module "vanta/dist/vanta.net.min" {
  import type * as Three from "three";

  export interface VantaEffect {
    destroy(): void;
    resize(): void;
  }

  export interface VantaNetOptions {
    el: HTMLElement;
    THREE: typeof Three;
    color?: number;
    backgroundColor?: number;
    backgroundAlpha?: number;
    points?: number;
    maxDistance?: number;
    spacing?: number;
    showDots?: boolean;
    mouseControls?: boolean;
    touchControls?: boolean;
    gyroControls?: boolean;
    minHeight?: number;
    minWidth?: number;
    scale?: number;
    scaleMobile?: number;
  }

  export default function NET(options: VantaNetOptions): VantaEffect;
}
