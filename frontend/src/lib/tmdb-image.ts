/**
 * TMDB serves every image at fixed widths from its own CDN, so next/image asks
 * TMDB for the right file and Vercel's optimizer is never used. next.config.ts
 * sets imageSizes and deviceSizes to these widths, so each srcset entry is a
 * file TMDB already has.
 */
export const TMDB_IMAGE_SIZES = [92, 154, 185, 300, 342, 500];
export const TMDB_DEVICE_SIZES = [780, 1280, 1920];

const TMDB_WIDTHS = [...TMDB_IMAGE_SIZES, ...TMDB_DEVICE_SIZES];
const TMDB_FILE = /^https:\/\/image\.tmdb\.org\/t\/p\/[^/]+(\/[^/]+)$/;

/** Full-size TMDB URL for a poster or backdrop path; the loader picks the width. */
export function tmdbImage(path: string, size = "original"): string {
  return `https://image.tmdb.org/t/p/${size}${path}`;
}

export default function tmdbImageLoader({ src, width }: { src: string; width: number }): string {
  const file = TMDB_FILE.exec(src)?.[1];
  if (!file) return src;
  const size = TMDB_WIDTHS.find((w) => w >= width);
  return tmdbImage(file, size ? `w${size}` : "original");
}
