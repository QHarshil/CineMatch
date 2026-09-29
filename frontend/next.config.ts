import type { NextConfig } from "next";
import { API_BASE } from "./src/lib/api";
import { TMDB_DEVICE_SIZES, TMDB_IMAGE_SIZES } from "./src/lib/tmdb-image";

// Only the dev server's fast refresh needs eval.
const devEval = process.env.NODE_ENV === "development" ? " 'unsafe-eval'" : "";

const cspDirectives = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline'${devEval}`,
  "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
  "font-src 'self' https://fonts.gstatic.com",
  "img-src 'self' data: https://image.tmdb.org",
  `connect-src 'self' https://*.supabase.co wss://*.supabase.co ${API_BASE}`,
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
].join("; ");

const securityHeaders = [
  { key: "Content-Security-Policy", value: cspDirectives },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains; preload" },
  { key: "X-DNS-Prefetch-Control", value: "on" },
];

const nextConfig: NextConfig = {
  // TMDB resizes images on its own CDN, so the loader requests its widths
  // directly and no image goes through Vercel's optimizer.
  images: {
    loader: "custom",
    loaderFile: "./src/lib/tmdb-image.ts",
    imageSizes: TMDB_IMAGE_SIZES,
    deviceSizes: TMDB_DEVICE_SIZES,
  },
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: securityHeaders,
      },
    ];
  },
};

export default nextConfig;
