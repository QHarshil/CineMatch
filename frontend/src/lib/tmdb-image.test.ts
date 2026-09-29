import { describe, expect, it } from "vitest";
import tmdbImageLoader, { TMDB_DEVICE_SIZES, TMDB_IMAGE_SIZES, tmdbImage } from "./tmdb-image";

const poster = tmdbImage("/abc.jpg");

describe("tmdbImageLoader", () => {
  it("serves each srcset width as the TMDB file of that width", () => {
    for (const width of [...TMDB_IMAGE_SIZES, ...TMDB_DEVICE_SIZES]) {
      expect(tmdbImageLoader({ src: poster, width })).toBe(`https://image.tmdb.org/t/p/w${width}/abc.jpg`);
    }
  });

  it("rounds other widths up to the next TMDB width", () => {
    expect(tmdbImageLoader({ src: poster, width: 160 })).toBe("https://image.tmdb.org/t/p/w185/abc.jpg");
    expect(tmdbImageLoader({ src: tmdbImage("/abc.jpg", "w500"), width: 360 })).toBe(
      "https://image.tmdb.org/t/p/w500/abc.jpg",
    );
  });

  it("falls back to the original above the largest width", () => {
    expect(tmdbImageLoader({ src: poster, width: 3840 })).toBe("https://image.tmdb.org/t/p/original/abc.jpg");
  });

  it("leaves other URLs alone", () => {
    expect(tmdbImageLoader({ src: "/favicon.svg", width: 92 })).toBe("/favicon.svg");
  });
});
